package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/sse"
	"github.com/zyvorai/yard/internal/store"
)

type Engine struct {
	Store *store.Store
	Hub   *sse.Hub
	Log   *log.Logger
	HTTP  *http.Client
}

func (e *Engine) client() *http.Client {
	if e.HTTP != nil {
		return e.HTTP
	}
	return &http.Client{Timeout: 8 * time.Second}
}

func (e *Engine) publish(orgID, kind string, payload any) {
	if e.Hub != nil {
		e.Hub.Publish(orgID, kind, payload)
	}
}

// StartStaleTicker marks stale assets for every org on an interval until ctx ends.
func (e *Engine) StartStaleTicker(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = 30 * time.Second
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				orgs, err := e.Store.ListOrgIDs(ctx)
				if err != nil {
					if e.Log != nil {
						e.Log.Printf("stale ticker orgs: %v", err)
					}
					continue
				}
				for _, org := range orgs {
					if err := e.MarkStale(ctx, org); err != nil && e.Log != nil {
						e.Log.Printf("stale ticker %s: %v", org, err)
					}
				}
			}
		}
	}()
}

func (e *Engine) IngestObservation(ctx context.Context, orgID string, in model.IngestObservation, source string) (*model.Observation, bool, error) {
	var asset *model.Asset
	var err error
	if in.AssetID != "" {
		asset, err = e.Store.AssetByID(ctx, orgID, in.AssetID)
	} else if in.AssetExternalRef != "" {
		asset, err = e.Store.AssetByRef(ctx, orgID, in.AssetExternalRef)
	}
	if err != nil || asset == nil {
		return nil, false, fmt.Errorf("unknown asset")
	}
	if in.ObservedAt.IsZero() {
		in.ObservedAt = time.Now().UTC()
	}
	if in.Quality == "" {
		in.Quality = "good"
	}
	if source != "" && in.Source == "" {
		in.Source = source
	}
	obs := &model.Observation{
		OrganizationID: orgID,
		AssetID:        asset.ID,
		Capability:     in.Capability,
		Value:          in.Value,
		Unit:           in.Unit,
		Quality:        in.Quality,
		Source:         in.Source,
		ObservedAt:     in.ObservedAt.UTC(),
		DedupeKey:      in.DedupeKey,
	}
	inserted, err := e.Store.InsertObservation(ctx, obs)
	if err != nil {
		return nil, false, err
	}
	if !inserted {
		return obs, false, nil
	}
	health := deriveHealth(in.Capability, in.Value, in.Quality)
	if err := e.Store.TouchAsset(ctx, asset.ID, in.Latitude, in.Longitude, health); err != nil {
		return obs, true, err
	}
	asset.Health = health
	e.publish(orgID, "asset.health", map[string]any{"id": asset.ID, "health": health, "name": asset.Name})
	if err := e.applyAutomations(ctx, orgID, asset, obs); err != nil && e.Log != nil {
		e.Log.Printf("automation error: %v", err)
	}
	return obs, true, nil
}

func deriveHealth(cap string, value float64, quality string) string {
	if quality == "bad" || quality == "uncertain" {
		return "degraded"
	}
	switch strings.ToLower(cap) {
	case "temperature":
		if value >= 90 {
			return "critical"
		}
		if value >= 75 {
			return "degraded"
		}
	case "heartbeat":
		if value <= 0 {
			return "stale"
		}
	}
	return "healthy"
}

func (e *Engine) applyAutomations(ctx context.Context, orgID string, asset *model.Asset, obs *model.Observation) error {
	autos, err := e.Store.ListAutomations(ctx, orgID)
	if err != nil {
		return err
	}
	for _, a := range autos {
		if !a.Enabled {
			continue
		}
		if a.TriggerKind == "threshold" && a.Capability == obs.Capability {
			hit := false
			switch a.Operator {
			case "gt":
				hit = obs.Value > a.Threshold
			case "gte":
				hit = obs.Value >= a.Threshold
			case "lt":
				hit = obs.Value < a.Threshold
			case "lte":
				hit = obs.Value <= a.Threshold
			}
			if !hit {
				continue
			}
			if err := e.runAutomationAction(ctx, orgID, asset, a, obs); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Engine) runAutomationAction(ctx context.Context, orgID string, asset *model.Asset, a model.Automation, obs *model.Observation) error {
	switch a.Action {
	case "open_incident":
		return e.openIncident(ctx, orgID, asset, a, obs)
	case "notify":
		e.publish(orgID, "automation.notify", map[string]any{
			"automation": a.Name, "asset": asset.Name, "capability": obs.Capability, "value": obs.Value,
		})
		_ = e.Store.Audit(ctx, orgID, "automation:"+a.ID, "notify", asset.ID, a.Name)
		return nil
	case "webhook":
		return e.fireWebhook(ctx, orgID, asset, a, obs)
	default:
		return nil
	}
}

func (e *Engine) fireWebhook(ctx context.Context, orgID string, asset *model.Asset, a model.Automation, obs *model.Observation) error {
	url := ""
	var cfg map[string]any
	if json.Unmarshal([]byte(a.Config), &cfg) == nil {
		if u, ok := cfg["url"].(string); ok {
			url = u
		}
	}
	if url == "" {
		return nil
	}
	body, _ := json.Marshal(map[string]any{
		"automation": a.Name,
		"asset_id":   asset.ID,
		"asset_name": asset.Name,
		"capability": obs.Capability,
		"value":      obs.Value,
		"unit":       obs.Unit,
		"observed_at": obs.ObservedAt,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "yard-automation/1")
	resp, err := e.client().Do(req)
	if err != nil {
		if e.Log != nil {
			e.Log.Printf("webhook %s: %v", url, err)
		}
		return nil
	}
	defer resp.Body.Close()
	_ = e.Store.Audit(ctx, orgID, "automation:"+a.ID, "webhook", asset.ID, fmt.Sprintf("%s → %d", url, resp.StatusCode))
	return nil
}

func (e *Engine) openIncident(ctx context.Context, orgID string, asset *model.Asset, a model.Automation, obs *model.Observation) error {
	title := fmt.Sprintf("%s on %s", a.Name, asset.Name)
	existing, need, err := e.Store.OpenIncidentByDedupe(ctx, orgID, title, &asset.ID)
	if err != nil {
		return err
	}
	if !need {
		return nil
	}
	_ = existing
	sev := "warning"
	if obs.Value >= 90 || asset.Health == "critical" {
		sev = "critical"
	}
	inc := &model.Incident{
		OrganizationID: orgID,
		AssetID:        &asset.ID,
		SiteID:         asset.SiteID,
		Title:          title,
		Severity:       sev,
		Status:         "open",
		Summary:        fmt.Sprintf("%s=%v %s (threshold %s %v)", obs.Capability, obs.Value, obs.Unit, a.Operator, a.Threshold),
	}
	if err := e.Store.CreateIncident(ctx, inc); err != nil {
		return err
	}
	_, _ = e.Store.InsertEvent(ctx, &model.Event{
		OrganizationID: orgID, AssetID: &asset.ID, Kind: "incident.auto", Severity: sev,
		Title: title, Body: inc.Summary, DedupeKey: "auto:" + a.ID + ":" + asset.ID + ":" + obs.Capability,
	})
	_ = e.Store.Audit(ctx, orgID, "automation:"+a.ID, "incident.open", inc.ID, title)
	e.publish(orgID, "incident.opened", inc)
	return nil
}

func (e *Engine) IngestInventory(ctx context.Context, orgID string, in model.IngestInventory, source string) (*model.Asset, error) {
	var siteID *string
	if in.SiteName != "" {
		if site, err := e.Store.SiteByName(ctx, orgID, in.SiteName); err == nil {
			siteID = &site.ID
		}
	}
	existing, _ := e.Store.AssetByRef(ctx, orgID, in.ExternalRef)
	a := &model.Asset{
		OrganizationID: orgID,
		SiteID:         siteID,
		Name:           first(in.Name, in.ExternalRef),
		ExternalRef:    in.ExternalRef,
		Kind:           first(in.Kind, "device"),
		Manufacturer:   in.Manufacturer,
		Model:          in.Model,
		Serial:         first(in.Serial, in.ExternalRef),
		Metadata:       first(in.Metadata, "{}"),
		Health:         "unknown",
		Status:         "active",
		StaleAfterSec:  90,
	}
	if existing != nil {
		a.ID = existing.ID
		a.CreatedAt = existing.CreatedAt
		a.Health = existing.Health
	}
	if in.Latitude != nil {
		a.Latitude = in.Latitude
	}
	if in.Longitude != nil {
		a.Longitude = in.Longitude
	}
	if err := e.Store.UpsertAsset(ctx, a); err != nil {
		return nil, err
	}
	var caps []model.Capability
	for _, c := range in.Capabilities {
		caps = append(caps, model.Capability{AssetID: a.ID, Name: c, Kind: "metric"})
	}
	if len(caps) > 0 {
		_ = e.Store.ReplaceCapabilities(ctx, a.ID, caps)
	}
	e.publish(orgID, "inventory", a)
	return a, nil
}

func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func (e *Engine) MarkStale(ctx context.Context, orgID string) error {
	n, err := e.Store.RefreshStale(ctx, orgID)
	if err != nil {
		return err
	}
	if n > 0 {
		e.publish(orgID, "assets.stale", map[string]any{"count": n})
	}
	autos, _ := e.Store.ListAutomations(ctx, orgID)
	assets, _ := e.Store.ListAssets(ctx, orgID, "", "", "stale")
	for _, a := range autos {
		if !a.Enabled || a.TriggerKind != "stale" {
			continue
		}
		for i := range assets {
			asset := assets[i]
			switch a.Action {
			case "open_incident", "":
				title := fmt.Sprintf("Missed heartbeat on %s", asset.Name)
				_, need, err := e.Store.OpenIncidentByDedupe(ctx, orgID, title, &asset.ID)
				if err != nil || !need {
					continue
				}
				inc := &model.Incident{
					OrganizationID: orgID,
					AssetID:        &asset.ID,
					SiteID:         asset.SiteID,
					Title:          title,
					Severity:       "warning",
					Status:         "open",
					Summary:        "Telemetry older than stale_after_sec; heartbeat missing.",
				}
				if err := e.Store.CreateIncident(ctx, inc); err != nil {
					return err
				}
				_, _ = e.Store.InsertEvent(ctx, &model.Event{
					OrganizationID: orgID, AssetID: &asset.ID, Kind: "heartbeat.missed", Severity: "warning",
					Title: title, Body: inc.Summary, DedupeKey: "stale:" + asset.ID,
				})
				e.publish(orgID, "incident.opened", inc)
			case "notify":
				e.publish(orgID, "automation.notify", map[string]any{"automation": a.Name, "asset": asset.Name, "reason": "stale"})
			case "webhook":
				_ = e.fireWebhook(ctx, orgID, &asset, a, &model.Observation{Capability: "heartbeat", Value: 0})
			}
		}
	}
	return nil
}
