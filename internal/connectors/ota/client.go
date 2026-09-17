// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package ota

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client lists OTA campaign / device status from Fleet (preferred) or Nodra.
type Client struct {
	Endpoint  string
	AuthToken string
	Source    string // "fleet" or "nodra"
	HTTP      *http.Client
}

func New(endpoint, authToken, source string) *Client {
	if source == "" {
		source = "fleet"
	}
	return &Client{
		Endpoint:  strings.TrimRight(endpoint, "/"),
		AuthToken: authToken,
		Source:    source,
		HTTP:      &http.Client{Timeout: 20 * time.Second},
	}
}

// CampaignList is returned by List.
type CampaignList struct {
	Source   string           `json:"source"`
	Devices  []map[string]any `json:"devices,omitempty"`
	Items    []map[string]any `json:"items,omitempty"`
	SyncedAt time.Time        `json:"synced_at"`
}

// List campaigns/devices depending on source.
func (c *Client) List(ctx context.Context) (*CampaignList, error) {
	if c.Endpoint == "" {
		return nil, fmt.Errorf("ota endpoint required")
	}
	if c.AuthToken == "" {
		return nil, fmt.Errorf("ota auth_token required in connector config")
	}
	out := &CampaignList{Source: c.Source, SyncedAt: time.Now().UTC()}
	switch c.Source {
	case "nodra":
		// Prefer campaign API when present; fall back to per-device twin OTA via devices list.
		if items, err := c.getList(ctx, c.Endpoint+"/api/v1/ota/campaigns"); err == nil {
			out.Items = items
			return out, nil
		}
		devices, err := c.getList(ctx, c.Endpoint+"/api/v1/devices")
		if err != nil {
			return nil, err
		}
		var otaDevs []map[string]any
		for _, d := range devices {
			id := str(d["id"])
			if id == "" {
				continue
			}
			b, err := c.getRaw(ctx, c.Endpoint+"/api/v1/devices/"+id+"/ota")
			if err != nil {
				continue
			}
			var m map[string]any
			if json.Unmarshal(b, &m) == nil {
				otaDevs = append(otaDevs, m)
			}
		}
		out.Devices = otaDevs
		return out, nil
	default: // fleet
		devs, err := c.getList(ctx, c.Endpoint+"/api/v1/ota/devices")
		if err != nil {
			return nil, err
		}
		out.Devices = devs
		return out, nil
	}
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
	for _, key := range []string{"campaigns", "devices", "items", "data"} {
		if raw, ok := wrap[key]; ok {
			jb, _ := json.Marshal(raw)
			if json.Unmarshal(jb, &list) == nil {
				return list, nil
			}
		}
	}
	return list, nil
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

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
