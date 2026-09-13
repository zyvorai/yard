package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/zyvorai/yard/internal/connectors/deviceagent"
	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

// Dispatcher executes connector actions. Only device-agent runs outbound today;
// Fleet/OTA/Nodra remain display-only.
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
	case "nodra", "fleet", "ota":
		return "unsupported", fmt.Sprintf("%s connector is catalog-only; configure when the product API is available", conn.Kind), nil
	default:
		return "unsupported", fmt.Sprintf("unknown connector kind %q", conn.Kind), nil
	}
}

func (d *Dispatcher) deviceAgent(ctx context.Context, orgID string, conn *model.Connector, action, payload string) (string, string, error) {
	endpoint := strings.TrimRight(conn.Endpoint, "/")
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
	client := deviceagent.New(endpoint, d.YardURL, ingestTok)
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
	// Fall back: look up http connector and require env — hashed tokens cannot be reversed.
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
