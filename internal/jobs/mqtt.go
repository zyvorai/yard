package jobs

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/zyvorai/yard/internal/model"
)

// StartMQTT subscribes when YARD_MQTT_URL is set. An empty URL leaves MQTT off.
// The default topic is yard/+/observations. YARD_MQTT_ORG selects the organization
// when the database has more than one.
func (e *Engine) StartMQTT(ctx context.Context) {
	if e == nil || e.Store == nil {
		return
	}
	url := os.Getenv("YARD_MQTT_URL")
	if url == "" {
		return
	}
	topic := os.Getenv("YARD_MQTT_TOPIC")
	if topic == "" {
		topic = "yard/+/observations"
	}
	go e.mqttLoop(ctx, url, topic)
}

func (e *Engine) mqttLoop(ctx context.Context, url, topic string) {
	opts := mqtt.NewClientOptions().AddBroker(url).SetClientID("yard").SetAutoReconnect(true).SetConnectRetry(true)
	if user := os.Getenv("YARD_MQTT_USER"); user != "" {
		opts.SetUsername(user)
		opts.SetPassword(os.Getenv("YARD_MQTT_PASSWORD"))
	}
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		token := c.Subscribe(topic, 0, func(_ mqtt.Client, m mqtt.Message) {
			org := e.mqttOrg(ctx)
			if org == "" {
				return
			}
			if _, err := ApplyMQTT(ctx, e, org, m.Topic(), m.Payload()); err != nil && e.Log != nil {
				e.Log.Error("mqtt", "err", err, "topic", m.Topic())
			}
		})
		token.Wait()
		if token.Error() != nil && e.Log != nil {
			e.Log.Error("mqtt subscribe", "err", token.Error())
		}
	})
	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(10*time.Second) && e.Log != nil {
		e.Log.Error("mqtt connect timeout", "url", url)
	}
	if err := token.Error(); err != nil && e.Log != nil {
		e.Log.Error("mqtt connect", "err", err)
	}
	<-ctx.Done()
	client.Disconnect(200)
}

func (e *Engine) mqttOrg(ctx context.Context) string {
	if v := strings.TrimSpace(os.Getenv("YARD_MQTT_ORG")); v != "" {
		return v
	}
	ids, err := e.Store.ListOrgIDs(ctx)
	if err != nil || len(ids) != 1 {
		if e.Log != nil {
			e.Log.Error("mqtt needs YARD_MQTT_ORG when more than one organization exists")
		}
		return ""
	}
	return ids[0]
}

// ApplyMQTT turns a topic payload into observations. The asset external ref
// comes from the JSON body or from yard/{ref}/... in the topic.
func ApplyMQTT(ctx context.Context, e *Engine, orgID, topic string, body []byte) (int, error) {
	batch, err := decodeMQTT(body)
	if err != nil {
		return 0, err
	}
	ref := mqttAssetRef(topic)
	accepted := 0
	for _, in := range batch {
		if in.AssetExternalRef == "" && in.AssetID == "" {
			in.AssetExternalRef = ref
		}
		if in.Source == "" {
			in.Source = "mqtt"
		}
		_, ok, err := e.IngestObservation(ctx, orgID, in, "mqtt")
		if err != nil || !ok {
			continue
		}
		accepted++
	}
	return accepted, nil
}

func decodeMQTT(body []byte) ([]model.IngestObservation, error) {
	if len(body) > 0 && body[0] == '[' {
		var batch []model.IngestObservation
		err := json.Unmarshal(body, &batch)
		return batch, err
	}
	var one model.IngestObservation
	if err := json.Unmarshal(body, &one); err != nil {
		return nil, err
	}
	return []model.IngestObservation{one}, nil
}

func mqttAssetRef(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) >= 2 && parts[0] == "yard" && parts[1] != "+" {
		return parts[1]
	}
	return ""
}
