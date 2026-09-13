package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Gateway connector: pull Device Agent locally and push normalized inventory
// and observations to Estate. No inbound hole-punch to remote devices.
func main() {
	agent := strings.TrimRight(env("DEVICE_AGENT_URL", "http://127.0.0.1:9188"), "/")
	estate := strings.TrimRight(env("ESTATE_URL", "http://127.0.0.1:8080"), "/")
	tok := strings.TrimSpace(env("ESTATE_INGEST_TOKEN", readFile("data/ingest.token")))
	if tok == "" {
		log.Fatal("ESTATE_INGEST_TOKEN is required")
	}
	log.Printf("device-agent gateway %s → %s", agent, estate)
	for {
		if err := cycle(agent, estate, tok); err != nil {
			log.Printf("cycle: %v", err)
		}
		time.Sleep(15 * time.Second)
	}
}

func cycle(agent, estate, tok string) error {
	inv, err := getJSON(agent + "/api/v1/inventory")
	if err != nil {
		inv, err = getJSON(agent + "/inventory")
	}
	if err != nil {
		return err
	}
	ref := str(inv["serial"])
	if ref == "" {
		ref = str(inv["id"])
	}
	if ref == "" {
		ref = "device-agent-local"
	}
	name := str(inv["hostname"])
	if name == "" {
		name = "Device Agent " + ref
	}
	payload := map[string]any{
		"external_ref": ref,
		"name":         name,
		"kind":         "device",
		"manufacturer": "Zyvor",
		"model":        "Device Agent",
		"serial":       ref,
		"capabilities": []string{"cpu_temp", "heartbeat"},
		"metadata":     mustJSON(inv),
	}
	if err := post(estate+"/api/v1/ingest/inventory", tok, payload); err != nil {
		return err
	}
	sensors, err := getJSONList(agent + "/api/v1/sensors")
	if err != nil {
		sensors, _ = getJSONList(agent + "/sensors")
	}
	now := time.Now().UTC()
	var batch []map[string]any
	batch = append(batch, map[string]any{
		"asset_external_ref": ref,
		"capability":         "heartbeat",
		"value":              1,
		"unit":               "1",
		"quality":            "good",
		"source":             "device-agent",
		"observed_at":        now,
		"dedupe_key":         ref + ":heartbeat:" + now.Format("20060102150405"),
	})
	for _, s := range sensors {
		cap := str(s["id"])
		if cap == "" {
			cap = str(s["name"])
		}
		if cap == "" {
			continue
		}
		batch = append(batch, map[string]any{
			"asset_external_ref": ref,
			"capability":         cap,
			"value":              num(s["value"]),
			"unit":               str(s["unit"]),
			"quality":            first(str(s["quality"]), "good"),
			"source":             "device-agent",
			"observed_at":        now,
			"dedupe_key":         ref + ":" + cap + ":" + now.Format("20060102150405"),
		})
	}
	return post(estate+"/api/v1/ingest/observations", tok, batch)
}

func getJSON(url string) (map[string]any, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func getJSONList(url string) ([]map[string]any, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var list []map[string]any
	if err := json.Unmarshal(b, &list); err == nil {
		return list, nil
	}
	var wrap map[string]any
	if err := json.Unmarshal(b, &wrap); err != nil {
		return nil, err
	}
	if raw, ok := wrap["sensors"]; ok {
		jb, _ := json.Marshal(raw)
		_ = json.Unmarshal(jb, &list)
	}
	return list, nil
}

func post(url, tok string, body any) error {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		rb, _ := io.ReadAll(resp.Body)
		log.Printf("%s → %d %s", url, resp.StatusCode, string(rb))
	}
	return nil
}

func str(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}

func num(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	default:
		return 0
	}
}

func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func readFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}
