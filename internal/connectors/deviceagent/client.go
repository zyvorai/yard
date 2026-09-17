package deviceagent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client pulls inventory and sensors from a Device Agent and publishes to Yard ingest.
type Client struct {
	AgentURL      string
	YardURL       string
	IngestToken   string
	AgentToken    string // optional Bearer token for the Device Agent API
	TLSSkipVerify bool   // lab self-signed Device Agent certs
	HTTP          *http.Client
}

func New(agentURL, yardURL, ingestToken string) *Client {
	return &Client{
		AgentURL:    strings.TrimRight(agentURL, "/"),
		YardURL:     strings.TrimRight(yardURL, "/"),
		IngestToken: ingestToken,
		HTTP:        &http.Client{Timeout: 12 * time.Second},
	}
}

// WithAgentAuth sets the Device Agent bearer token used on inventory/sensor GETs.
func (c *Client) WithAgentAuth(token string) *Client {
	c.AgentToken = token
	return c
}

// WithTLSSkipVerify configures an HTTP client that accepts self-signed agent certs.
func (c *Client) WithTLSSkipVerify(skip bool) *Client {
	c.TLSSkipVerify = skip
	if skip {
		c.HTTP = &http.Client{
			Timeout: 12 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // lab / self-signed Device Agent
			},
		}
	}
	return c
}

// Sync performs one inventory + observations cycle.
func (c *Client) Sync(ctx context.Context) error {
	inv, err := c.getJSON(ctx, c.AgentURL+"/api/v1/inventory")
	if err != nil {
		inv, err = c.getJSON(ctx, c.AgentURL+"/inventory")
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
	if err := c.post(ctx, c.YardURL+"/api/v1/ingest/inventory", payload); err != nil {
		return err
	}
	sensors, err := c.getJSONList(ctx, c.AgentURL+"/api/v1/sensors")
	if err != nil {
		sensors, _ = c.getJSONList(ctx, c.AgentURL+"/sensors")
	}
	now := time.Now().UTC()
	batch := []map[string]any{{
		"asset_external_ref": ref,
		"capability":         "heartbeat",
		"value":              1,
		"unit":               "1",
		"quality":            "good",
		"source":             "device-agent",
		"observed_at":        now,
		"dedupe_key":         ref + ":heartbeat:" + now.Format("20060102150405"),
	}}
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
	return c.post(ctx, c.YardURL+"/api/v1/ingest/observations", batch)
}

// Diagnostics reads agent health/info endpoints for action results.
func (c *Client) Diagnostics(ctx context.Context) (string, error) {
	for _, path := range []string{"/api/v1/health", "/healthz", "/health", "/api/v1/status"} {
		body, err := c.getRaw(ctx, c.AgentURL+path)
		if err == nil && len(body) > 0 {
			return string(body), nil
		}
	}
	inv, err := c.getJSON(ctx, c.AgentURL+"/api/v1/inventory")
	if err != nil {
		inv, err = c.getJSON(ctx, c.AgentURL+"/inventory")
	}
	if err != nil {
		return "", fmt.Errorf("device-agent unreachable: %w", err)
	}
	b, _ := json.Marshal(inv)
	return string(b), nil
}

func (c *Client) getRaw(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if c.AgentToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AgentToken)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s → %d", url, resp.StatusCode)
	}
	return b, nil
}

func (c *Client) getJSON(ctx context.Context, url string) (map[string]any, error) {
	b, err := c.getRaw(ctx, url)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *Client) getJSONList(ctx context.Context, url string) ([]map[string]any, error) {
	b, err := c.getRaw(ctx, url)
	if err != nil {
		return nil, err
	}
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

func (c *Client) post(ctx context.Context, url string, body any) error {
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.IngestToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		rb, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s → %d %s", url, resp.StatusCode, string(rb))
	}
	return nil
}

func str(v any) string {
	if v == nil {
		return ""
	}
	if t, ok := v.(string); ok {
		return t
	}
	return fmt.Sprint(v)
}

func num(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
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
