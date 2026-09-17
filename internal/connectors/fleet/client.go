// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: Apache-2.0

package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client reads Zyvor Fleet control-plane lifecycle (sites, rollouts, OTA devices).
type Client struct {
	FleetURL  string
	AuthToken string
	HTTP      *http.Client
}

func New(fleetURL, authToken string) *Client {
	return &Client{
		FleetURL:  strings.TrimRight(fleetURL, "/"),
		AuthToken: authToken,
		HTTP:      &http.Client{Timeout: 20 * time.Second},
	}
}

// LifecycleSnapshot is returned by Sync / DesiredProgress.
type LifecycleSnapshot struct {
	Sites    int              `json:"sites"`
	Rollouts []map[string]any `json:"rollouts"`
	OTADevs  []map[string]any `json:"ota_devices,omitempty"`
	SyncedAt time.Time        `json:"synced_at"`
}

// Sync pulls sites + rollouts (+ OTA devices when available).
func (c *Client) Sync(ctx context.Context) (*LifecycleSnapshot, error) {
	if c.FleetURL == "" {
		return nil, fmt.Errorf("fleet endpoint required")
	}
	if c.AuthToken == "" {
		return nil, fmt.Errorf("fleet auth_token required in connector config")
	}
	sites, err := c.getList(ctx, c.FleetURL+"/api/v1/sites")
	if err != nil {
		return nil, fmt.Errorf("list sites: %w", err)
	}
	rollouts, err := c.getList(ctx, c.FleetURL+"/api/v1/rollouts")
	if err != nil {
		return nil, fmt.Errorf("list rollouts: %w", err)
	}
	ota, _ := c.getList(ctx, c.FleetURL+"/api/v1/ota/devices")
	return &LifecycleSnapshot{
		Sites:    len(sites),
		Rollouts: rollouts,
		OTADevs:  ota,
		SyncedAt: time.Now().UTC(),
	}, nil
}

// DesiredProgress returns a compact summary for action results / UI.
func (c *Client) DesiredProgress(ctx context.Context) (string, error) {
	snap, err := c.Sync(ctx)
	if err != nil {
		return "", err
	}
	active, paused, failed := 0, 0, 0
	for _, ro := range snap.Rollouts {
		switch strings.ToLower(str(ro["status"])) {
		case "running", "active", "in_progress":
			active++
		case "paused":
			paused++
		case "failed":
			failed++
		}
	}
	summary := map[string]any{
		"sites":           snap.Sites,
		"rollouts_total":  len(snap.Rollouts),
		"rollouts_active": active,
		"rollouts_paused": paused,
		"rollouts_failed": failed,
		"ota_devices":     len(snap.OTADevs),
		"synced_at":       snap.SyncedAt,
		"rollouts":        snap.Rollouts,
		"ota_devices_raw": snap.OTADevs,
	}
	b, _ := json.Marshal(summary)
	return string(b), nil
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
	for _, key := range []string{"sites", "rollouts", "devices", "items", "data"} {
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
