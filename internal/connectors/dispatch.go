package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/zyvorai/yard/internal/connectors/deviceagent"
	"github.com/zyvorai/yard/internal/connectors/fleet"
	"github.com/zyvorai/yard/internal/connectors/nodra"
	"github.com/zyvorai/yard/internal/connectors/ota"
	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

// Dispatcher executes connector actions. Device Agent, Nodra, Fleet, and OTA
// run outbound syncs when an endpoint + auth_token are configured.
type Dispatcher struct {
	Store   *store.Store
	YardURL string
}

func NewDispatcher(st *store.Store) *Dispatcher {
	base := strings.TrimRight(os.Getenv("YARD_URL"), "/")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	return &Dispatcher{Store: st, YardURL: base}
}

// Execute runs a connector action and returns a human-readable result.
func (d *Dispatcher) Execute(ctx context.Context, orgID string, conn *model.Connector, action, payload string) (status, result string, err error) {
	if conn == nil {
		return "failed", "", fmt.Errorf("connector required")
	}
	switch conn.Kind {
	case "device-agent":
		return d.deviceAgent(ctx, orgID, conn, action, payload)
	case "http", "simulator":
		return "recorded", "Ingest-only connector; use HTTP ingest endpoints.", nil
	case "nodra":
		return d.nodra(ctx, orgID, conn, action, payload)
	case "fleet":
		return d.fleet(ctx, orgID, conn, action, payload)
	case "ota":
		return d.ota(ctx, orgID, conn, action, payload)
	default:
		return "unsupported", fmt.Sprintf("unknown connector kind %q", conn.Kind), nil
	}
}

func (d *Dispatcher) deviceAgent(ctx context.Context, orgID string, conn *model.Connector, action, payload string) (string, string, error) {
	endpoint, agentTok := endpointAndToken(conn, payload)
	if endpoint == "" {
		endpoint = "http://127.0.0.1:9188"
	}
	if payload != "" && payload != "{}" {
		var cfg map[string]any
		if json.Unmarshal([]byte(payload), &cfg) == nil {
			if u, ok := cfg["agent_url"].(string); ok && u != "" {
				endpoint = strings.TrimRight(u, "/")
			}
		}
	}
	ingestTok, err := d.ingestToken(ctx, orgID)
	if err != nil {
		return "failed", "", err
	}
	client := deviceagent.New(endpoint, d.YardURL, ingestTok).WithAgentAuth(agentTok)
	// Lab Device Agents use self-signed TLS; skip verify for https unless explicitly disabled.
	skipTLS := strings.HasPrefix(strings.ToLower(endpoint), "https://")
	if v := configString(conn.Config, "tls_insecure"); v == "false" || v == "0" {
		skipTLS = false
	}
	if raw := conn.Config; raw != "" {
		var m map[string]any
		if json.Unmarshal([]byte(raw), &m) == nil {
			if b, ok := m["tls_insecure"].(bool); ok {
				skipTLS = b
			}
		}
	}
	if skipTLS {
		client = client.WithTLSSkipVerify(true)
	}
	switch action {
	case "inventory.refresh", "sync":
		if err := client.Sync(ctx); err != nil {
			return "failed", err.Error(), err
		}
		_ = d.Store.TouchConnector(ctx, conn.ID)
		return "completed", "Device Agent inventory and observations published to Yard ingest", nil
	case "diagnostics.read", "diagnostics":
		body, err := client.Diagnostics(ctx)
		if err != nil {
			return "failed", err.Error(), err
		}
		_ = d.Store.TouchConnector(ctx, conn.ID)
		return "completed", body, nil
	default:
		return "unsupported", fmt.Sprintf("device-agent does not execute %q", action), nil
	}
}

func (d *Dispatcher) nodra(ctx context.Context, orgID string, conn *model.Connector, action, payload string) (string, string, error) {
	endpoint, token := endpointAndToken(conn, payload)
	if endpoint == "" {
		return "failed", "", fmt.Errorf("set connector endpoint to the Nodra control-plane URL")
	}
	ingestTok, err := d.ingestToken(ctx, orgID)
	if err != nil {
		return "failed", "", err
	}
	client := nodra.New(endpoint, d.YardURL, token, ingestTok)
	switch action {
	case "telemetry.receive", "sync":
		res, err := client.Sync(ctx)
		if err != nil {
			return "failed", err.Error(), err
		}
		_ = d.Store.TouchConnector(ctx, conn.ID)
		b, _ := json.Marshal(res)
		return "completed", string(b), nil
	default:
		return "unsupported", fmt.Sprintf("nodra does not execute %q", action), nil
	}
}

func (d *Dispatcher) fleet(ctx context.Context, orgID string, conn *model.Connector, action, payload string) (string, string, error) {
	endpoint, token := endpointAndToken(conn, payload)
	if endpoint == "" {
		return "failed", "", fmt.Errorf("set connector endpoint to the Zyvor Fleet URL")
	}
	client := fleet.New(endpoint, token)
	switch action {
	case "lifecycle.request", "desired.progress", "sync":
		body, err := client.DesiredProgress(ctx)
		if err != nil {
			return "failed", err.Error(), err
		}
		_ = d.Store.TouchConnector(ctx, conn.ID)
		_ = orgID
		return "completed", body, nil
	default:
		return "unsupported", fmt.Sprintf("fleet does not execute %q", action), nil
	}
}

func (d *Dispatcher) ota(ctx context.Context, orgID string, conn *model.Connector, action, payload string) (string, string, error) {
	endpoint, token := endpointAndToken(conn, payload)
	if endpoint == "" {
		return "failed", "", fmt.Errorf("set connector endpoint to Fleet or Nodra URL")
	}
	source := configString(conn.Config, "source")
	if source == "" {
		source = "fleet"
	}
	if payload != "" {
		var p map[string]any
		if json.Unmarshal([]byte(payload), &p) == nil {
			if s, ok := p["source"].(string); ok && s != "" {
				source = s
			}
		}
	}
	client := ota.New(endpoint, token, source)
	switch action {
	case "campaign.list", "update.delegate", "sync":
		res, err := client.List(ctx)
		if err != nil {
			return "failed", err.Error(), err
		}
		_ = d.Store.TouchConnector(ctx, conn.ID)
		_ = orgID
		b, _ := json.Marshal(res)
		return "completed", string(b), nil
	default:
		return "unsupported", fmt.Sprintf("ota does not execute %q", action), nil
	}
}

func endpointAndToken(conn *model.Connector, payload string) (endpoint, token string) {
	endpoint = strings.TrimRight(conn.Endpoint, "/")
	token = configString(conn.Config, "auth_token")
	if token == "" {
		token = configString(conn.Config, "token")
	}
	if payload != "" && payload != "{}" {
		var p map[string]any
		if json.Unmarshal([]byte(payload), &p) == nil {
			if u, ok := p["endpoint"].(string); ok && u != "" {
				endpoint = strings.TrimRight(u, "/")
			}
			if t, ok := p["auth_token"].(string); ok && t != "" {
				token = t
			}
		}
	}
	return endpoint, token
}

func configString(raw, key string) string {
	if raw == "" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func (d *Dispatcher) ingestToken(ctx context.Context, orgID string) (string, error) {
	if t := strings.TrimSpace(os.Getenv("YARD_INGEST_TOKEN")); t != "" {
		return t, nil
	}
	if p := os.Getenv("YARD_INGEST_TOKEN_FILE"); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			if t := strings.TrimSpace(string(b)); t != "" {
				return t, nil
			}
		}
	}
	for _, p := range []string{"data/ingest.token", "/var/lib/yard/ingest.token"} {
		if b, err := os.ReadFile(p); err == nil {
			if t := strings.TrimSpace(string(b)); t != "" {
				return t, nil
			}
		}
	}
	list, err := d.Store.ListConnectors(ctx, orgID)
	if err != nil {
		return "", err
	}
	for _, c := range list {
		if c.Kind == "http" && c.Status == "connected" {
			return "", fmt.Errorf("YARD_INGEST_TOKEN (or data/ingest.token) required to dispatch device-agent actions")
		}
	}
	return "", fmt.Errorf("no ingest token available")
}
