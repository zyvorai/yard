package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/connectors/deviceagent"
	"github.com/zyvorai/yard/internal/connectors/fleet"
	"github.com/zyvorai/yard/internal/connectors/nodra"
	"github.com/zyvorai/yard/internal/connectors/ota"
	"github.com/zyvorai/yard/internal/egress"
	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

// Dispatcher executes connector actions. Device Agent, Nodra, Fleet, and OTA
// run outbound syncs when an endpoint and a stored secret are configured.
type Dispatcher struct {
	Store            *store.Store
	YardURL          string
	Mode             string
	AllowInsecureTLS bool
	Policy           *egress.Policy
	// Secret returns the decrypted connector credential, if any.
	Secret func(ctx context.Context, conn *model.Connector) (string, error)
}

func NewDispatcher(st *store.Store) *Dispatcher {
	base := strings.TrimRight(os.Getenv("YARD_URL"), "/")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	return &Dispatcher{Store: st, YardURL: base, Mode: "demo", Policy: egress.Demo()}
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
	endpoint, agentTok, err := d.endpointAndToken(ctx, conn, payload)
	if err != nil {
		return "failed", "", err
	}
	if endpoint == "" {
		return "failed", "", fmt.Errorf("connector endpoint required")
	}
	if d.tlsRequested(conn.Config) && !d.allowSkipTLS() {
		return "failed", "", fmt.Errorf("insecure TLS is not allowed in production")
	}
	ingestTok, err := d.ingestToken(ctx, orgID)
	if err != nil {
		return "failed", "", err
	}
	client := deviceagent.New(endpoint, d.YardURL, ingestTok).WithAgentAuth(agentTok)
	client.HTTP = d.policy().HTTPClient(12*time.Second, d.allowSkipTLS() && d.tlsRequested(conn.Config))
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
	endpoint, token, err := d.endpointAndToken(ctx, conn, payload)
	if err != nil {
		return "failed", "", err
	}
	if endpoint == "" {
		return "failed", "", fmt.Errorf("set connector endpoint to the Nodra control-plane URL")
	}
	ingestTok, err := d.ingestToken(ctx, orgID)
	if err != nil {
		return "failed", "", err
	}
	client := nodra.New(endpoint, d.YardURL, token, ingestTok)
	client.HTTP = d.policy().HTTPClient(20*time.Second, false)
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
	endpoint, token, err := d.endpointAndToken(ctx, conn, payload)
	if err != nil {
		return "failed", "", err
	}
	if endpoint == "" {
		return "failed", "", fmt.Errorf("set connector endpoint to the Zyvor Fleet URL")
	}
	client := fleet.New(endpoint, token)
	client.HTTP = d.policy().HTTPClient(20*time.Second, false)
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
	endpoint, token, err := d.endpointAndToken(ctx, conn, payload)
	if err != nil {
		return "failed", "", err
	}
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
	client.HTTP = d.policy().HTTPClient(20*time.Second, false)
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

func (d *Dispatcher) endpointAndToken(ctx context.Context, conn *model.Connector, payload string) (endpoint, token string, err error) {
	endpoint = strings.TrimRight(conn.Endpoint, "/")
	if d.Secret != nil {
		token, _ = d.Secret(ctx, conn)
	}
	if token == "" {
		token = configString(conn.Config, "auth_token")
		if token == "" {
			token = configString(conn.Config, "token")
		}
	}
	mode := d.Mode
	if mode == "" {
		mode = "demo"
	}
	if override := payloadEndpoint(payload); override != "" && mode != "production" {
		if err := d.policy().Validate(override); err != nil {
			return "", "", err
		}
		endpoint = strings.TrimRight(override, "/")
	}
	if endpoint == "" && conn.Kind == "device-agent" && mode != "production" {
		endpoint = "http://127.0.0.1:9188"
	}
	if strings.Contains(endpoint, "://") && !strings.HasPrefix(endpoint, "internal://") {
		if err := d.policy().Validate(endpoint); err != nil {
			return "", "", err
		}
	}
	return endpoint, token, nil
}

func (d *Dispatcher) policy() *egress.Policy {
	if d.Policy != nil {
		return d.Policy
	}
	return egress.Demo()
}

func (d *Dispatcher) allowSkipTLS() bool {
	if d.AllowInsecureTLS {
		return true
	}
	return d.Mode == "" || d.Mode == "demo"
}

func (d *Dispatcher) tlsRequested(raw string) bool {
	if configString(raw, "tls_insecure") == "true" || configString(raw, "tls_insecure") == "1" {
		return true
	}
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return false
	}
	b, ok := m["tls_insecure"].(bool)
	return ok && b
}

func payloadEndpoint(payload string) string {
	if payload == "" || payload == "{}" {
		return ""
	}
	var p map[string]any
	if json.Unmarshal([]byte(payload), &p) != nil {
		return ""
	}
	if u, ok := p["endpoint"].(string); ok && u != "" {
		return u
	}
	if u, ok := p["agent_url"].(string); ok && u != "" {
		return u
	}
	return ""
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
