package jobs

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

type Engine struct {
	Store *store.Store
	Log   *log.Logger
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
			if hit && a.Action == "open_incident" {
				if err := e.openIncident(ctx, orgID, asset, a, obs); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (e *Engine) openIncident(ctx context.Context, orgID string, asset *model.Asset, a model.Automation, obs *model.Observation) error {
	title := fmt.Sprintf("%s on %s", a.Name, asset.Name)
	existing, need, err := e.Store.OpenIncidentByDedupe(ctx, orgID, title, &asset.ID)
	if err != nil {
		return err
	}
	if !need && existing != nil {
		return nil
	}
	sev := "warning"
	if obs.Value >= a.Threshold*1.15 {
		sev = "critical"
	}
	inc := &model.Incident{
		OrganizationID: orgID,
		AssetID:        &asset.ID,
		SiteID:         asset.SiteID,
		Title:          title,
		Severity:       sev,
		Status:         "open",
		Summary:        fmt.Sprintf("%s %s %.2f %s (threshold %.2f)", obs.Capability, a.Operator, obs.Value, obs.Unit, a.Threshold),
	}
	if err := e.Store.CreateIncident(ctx, inc); err != nil {
		return err
	}
	_, _ = e.Store.InsertEvent(ctx, &model.Event{
		OrganizationID: orgID,
		AssetID:        &asset.ID,
		SiteID:         asset.SiteID,
		Kind:           "incident.opened",
		Severity:       sev,
		Title:          title,
		Body:           inc.Summary,
		DedupeKey:      "incopen:" + inc.ID,
	})
	_ = e.Store.Audit(ctx, orgID, "automation:"+a.ID, "incident.open", inc.ID, inc.Summary)
	_ = e.Store.SetAssetHealth(ctx, asset.ID, map[string]string{"warning": "degraded", "critical": "critical"}[sev])
	return nil
}

func (e *Engine) IngestInventory(ctx context.Context, orgID string, in model.IngestInventory, source string) (*model.Asset, error) {
	if in.ExternalRef == "" {
		return nil, fmt.Errorf("external_ref required")
	}
	asset, err := e.Store.AssetByRef(ctx, orgID, in.ExternalRef)
	if err != nil {
		asset = &model.Asset{OrganizationID: orgID, ExternalRef: in.ExternalRef}
	}
	if in.Name != "" {
		asset.Name = in.Name
	} else if asset.Name == "" {
		asset.Name = in.ExternalRef
	}
	if in.Kind != "" {
		asset.Kind = in.Kind
	} else if asset.Kind == "" {
		asset.Kind = "device"
	}
	asset.Manufacturer, asset.Model, asset.Serial = first(in.Manufacturer, asset.Manufacturer), first(in.Model, asset.Model), first(in.Serial, asset.Serial)
	if in.Latitude != nil {
		asset.Latitude = in.Latitude
	}
	if in.Longitude != nil {
		asset.Longitude = in.Longitude
	}
	if in.Metadata != "" {
		asset.Metadata = in.Metadata
	}
	if in.SiteName != "" {
		if site, err := e.Store.SiteByName(ctx, orgID, in.SiteName); err == nil {
			asset.SiteID = &site.ID
		}
	}
	now := time.Now().UTC()
	asset.LastSeenAt = &now
	if asset.Health == "unknown" || asset.Health == "stale" {
		asset.Health = "healthy"
	}
	if err := e.Store.UpsertAsset(ctx, asset); err != nil {
		return nil, err
	}
	if len(in.Capabilities) > 0 {
		caps := make([]model.Capability, 0, len(in.Capabilities))
		for _, name := range in.Capabilities {
			caps = append(caps, model.Capability{AssetID: asset.ID, Name: name, Kind: "measurement"})
		}
		_ = e.Store.ReplaceCapabilities(ctx, asset.ID, caps)
	}
	_, _ = e.Store.InsertEvent(ctx, &model.Event{
		OrganizationID: orgID,
		AssetID:        &asset.ID,
		Kind:           "inventory.upsert",
		Severity:       "info",
		Title:          "Inventory received for " + asset.Name,
		Body:           source,
		DedupeKey:      fmt.Sprintf("inv:%s:%d", asset.ExternalRef, now.Unix()/60),
	})
	return asset, nil
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
	if n == 0 {
		return nil
	}
	autos, _ := e.Store.ListAutomations(ctx, orgID)
	assets, _ := e.Store.ListAssets(ctx, orgID, "", "", "stale")
	for _, a := range autos {
		if !a.Enabled || a.TriggerKind != "stale" || a.Action != "open_incident" {
			continue
		}
		for i := range assets {
			asset := assets[i]
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
		}
	}
	return nil
}
