// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package nodra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client pulls devices/twins from a Nodra control plane and publishes to Yard ingest.
type Client struct {
	NodraURL    string
	YardURL     string
	AuthToken   string
	IngestToken string
	HTTP        *http.Client
}

func New(nodraURL, yardURL, authToken, ingestToken string) *Client {
	return &Client{
		NodraURL:    strings.TrimRight(nodraURL, "/"),
		YardURL:     strings.TrimRight(yardURL, "/"),
		AuthToken:   authToken,
		IngestToken: ingestToken,
		HTTP:        &http.Client{Timeout: 20 * time.Second},
	}
}

// SyncResult summarizes one pull → ingest cycle.
type SyncResult struct {
	Devices      int              `json:"devices"`
	Observations int              `json:"observations"`
	Snapshots    []DeviceSnapshot `json:"snapshots"`
}

// DeviceSnapshot is stored on asset metadata under key "nodra".
type DeviceSnapshot struct {
	DeviceID   string         `json:"device_id"`
	SiteID     string         `json:"site_id,omitempty"`
	Name       string         `json:"name,omitempty"`
	Protocol   string         `json:"protocol,omitempty"`
	Reported   map[string]any `json:"reported,omitempty"`
	DesiredOTA any            `json:"desired_ota,omitempty"`
	StatusOTA  any            `json:"status_ota,omitempty"`
	SyncedAt   time.Time      `json:"synced_at"`
}

// Sync lists Nodra devices + twins, upserts inventory, and publishes numeric reported fields as observations.
func (c *Client) Sync(ctx context.Context) (*SyncResult, error) {
	if c.NodraURL == "" {
		return nil, fmt.Errorf("nodra endpoint required")
	}
	if c.AuthToken == "" {
		return nil, fmt.Errorf("nodra auth_token required in connector config")
	}
	devices, err := c.getList(ctx, c.NodraURL+"/api/v1/devices")
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	twins, _ := c.getList(ctx, c.NodraURL+"/api/v1/twins")
	twinByDev := map[string]map[string]any{}
	for _, tw := range twins {
		id := str(tw["device_id"])
		if id != "" {
			twinByDev[id] = tw
		}
	}
	out := &SyncResult{}
	now := time.Now().UTC()
	var batch []map[string]any
	for _, d := range devices {
		id := str(d["id"])
		if id == "" {
			continue
		}
		name := str(d["name"])
		if name == "" {
			name = id
		}
		siteID := str(d["site_id"])
		protocol := str(d["protocol"])
		meta := map[string]any{"nodra": map[string]any{
			"device_id": id, "site_id": siteID, "protocol": protocol, "raw": d,
		}}
		tw := twinByDev[id]
		snap := DeviceSnapshot{
			DeviceID: id, SiteID: siteID, Name: name, Protocol: protocol, SyncedAt: now,
		}
		if tw != nil {
			if rep, ok := tw["reported"].(map[string]any); ok {
				snap.Reported = rep
				if ota, ok := rep["ota"]; ok {
					snap.StatusOTA = ota
				}
				for k, v := range rep {
					if k == "ota" {
						continue
					}
					if n, ok := asFloat(v); ok {
						batch = append(batch, map[string]any{
							"asset_external_ref": id,
							"capability":         k,
							"value":              n,
							"unit":               "",
							"quality":            "good",
							"source":             "nodra",
							"observed_at":        now,
							"dedupe_key":         fmt.Sprintf("%s:%s:%s", id, k, now.Format("20060102150405")),
						})
					}
				}
			}
			if des, ok := tw["desired"].(map[string]any); ok {
				if ota, ok := des["ota"]; ok {
					snap.DesiredOTA = ota
				}
			}
			meta["nodra"] = snap
		}
		inv := map[string]any{
			"external_ref": id,
			"name":         name,
			"kind":         "device",
			"manufacturer": "Nodra",
			"model":        first(protocol, "edge-device"),
			"serial":       id,
			"site_name":    siteID,
			"capabilities": []string{"heartbeat"},
			"metadata":     mustJSON(meta),
		}
		if err := c.post(ctx, c.YardURL+"/api/v1/ingest/inventory", inv); err != nil {
			return nil, fmt.Errorf("ingest inventory %s: %w", id, err)
		}
		batch = append(batch, map[string]any{
			"asset_external_ref": id,
			"capability":         "heartbeat",
			"value":              1,
			"unit":               "1",
			"quality":            "good",
			"source":             "nodra",
			"observed_at":        now,
			"dedupe_key":         id + ":heartbeat:" + now.Format("20060102150405"),
		})
		out.Devices++
		out.Snapshots = append(out.Snapshots, snap)
	}
	if len(batch) > 0 {
		if err := c.post(ctx, c.YardURL+"/api/v1/ingest/observations", batch); err != nil {
			return nil, fmt.Errorf("ingest observations: %w", err)
		}
		out.Observations = len(batch)
	}
	return out, nil
}

func (c *Client) getList(ctx context.Context, url string) ([]map[string]any, error) {
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
	for _, key := range []string{"devices", "twins", "items", "data"} {
		if raw, ok := wrap[key]; ok {
			jb, _ := json.Marshal(raw)
			if json.Unmarshal(jb, &list) == nil {
				return list, nil
			}
		}
	}
	return nil, fmt.Errorf("unexpected list payload from %s", url)
}

func (c *Client) getRaw(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s → %d: %s", url, resp.StatusCode, truncate(string(b), 200))
	}
	return b, nil
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
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s → %d: %s", url, resp.StatusCode, truncate(string(rb), 200))
	}
	return nil
}

func str(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return ""
	}
}

func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	default:
		return 0, false
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
