package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/sse"
	"github.com/zyvorai/yard/internal/store"
)

type Engine struct {
	Store *store.Store
	Hub   *sse.Hub
	Log   *slog.Logger
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
						e.Log.Error("stale ticker: list orgs", "err", err)
					}
					continue
				}
				for _, org := range orgs {
					if err := e.MarkStale(ctx, org); err != nil && e.Log != nil {
						e.Log.Error("stale ticker", "org", org, "err", err)
					}
				}
			}
		}
	}()
}

// StartActionSweeper expires queued remote actions past their deadline.
func (e *Engine) StartActionSweeper(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = 60 * time.Second
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				n, err := e.Store.ExpireActions(ctx)
				if err != nil && e.Log != nil {
					e.Log.Error("action sweeper", "err", err)
					continue
				}
				if n > 0 && e.Log != nil {
					e.Log.Info("action sweeper expired", "count", n)
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
		e.Log.Error("automation", "err", err)
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

// applyAutomations evaluates every enabled automation against a fresh
// observation. Besides the original "threshold" trigger (a literal operator
// + value on the rule itself), "capability_min"/"capability_max" fire off
// the matching Capability's own Min/Max range instead of a duplicated
// literal — so alarming on a capability's declared range needs no threshold
// entry, and soft-vs-hard severity is just the operator's choice of the
// existing "notify" vs "open_incident" action.
func (e *Engine) applyAutomations(ctx context.Context, orgID string, asset *model.Asset, obs *model.Observation) error {
	autos, err := e.Store.ListAutomations(ctx, orgID)
	if err != nil {
		return err
	}
	var caps []model.Capability
	capsLoaded := false
	capabilityFor := func(name string) *model.Capability {
		if !capsLoaded {
			caps, _ = e.Store.ListCapabilities(ctx, asset.ID)
			capsLoaded = true
		}
		for i := range caps {
			if caps[i].Name == name {
				return &caps[i]
			}
		}
		return nil
	}
	for _, a := range autos {
		if !a.Enabled || a.Capability != obs.Capability {
			continue
		}
		hit := false
		switch a.TriggerKind {
		case "threshold":
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
		case "capability_min":
			if c := capabilityFor(obs.Capability); c != nil && c.Min != nil {
				hit = obs.Value < *c.Min
			}
		case "capability_max":
			if c := capabilityFor(obs.Capability); c != nil && c.Max != nil {
				hit = obs.Value > *c.Max
			}
		default:
			continue
		}
		if !hit {
			continue
		}
		if err := e.runAutomationAction(ctx, orgID, asset, a, obs); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) runAutomationAction(ctx context.Context, orgID string, asset *model.Asset, a model.Automation, obs *model.Observation) error {
	if a.Action == "open_incident" {
		return e.openIncident(ctx, orgID, asset, a, obs)
	}
	return e.actionDispatch(ctx, orgID, asset, a, obs)
}

// actionDispatch fires every non-incident automation action (notify, webhook,
// email, slack, pagerduty) for both the threshold/capability trigger path
// (runAutomationAction, above) and the stale trigger path (MarkStale, below)
// — incident-opening stays separate in each since threshold and stale
// incidents have different title/dedupe/summary shapes.
func (e *Engine) actionDispatch(ctx context.Context, orgID string, asset *model.Asset, a model.Automation, obs *model.Observation) error {
	switch a.Action {
	case "notify":
		e.publish(orgID, "automation.notify", map[string]any{
			"automation": a.Name, "asset": asset.Name, "capability": obs.Capability, "value": obs.Value,
		})
		_ = e.Store.Audit(ctx, orgID, "automation:"+a.ID, "notify", asset.ID, a.Name)
		return nil
	case "webhook":
		return e.fireWebhook(ctx, orgID, asset, a, obs)
	case "email":
		return e.sendEmail(ctx, orgID, asset, a, obs)
	case "slack":
		return e.fireSlack(ctx, orgID, asset, a, obs)
	case "pagerduty":
		return e.firePagerDuty(ctx, orgID, asset, a, obs)
	default:
		return nil
	}
}

// configString reads a single string field out of an Automation.Config JSON
// blob, returning "" if the field is absent or the blob doesn't parse.
func configString(cfg, key string) string {
	var m map[string]any
	if json.Unmarshal([]byte(cfg), &m) != nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

// postJSON POSTs a JSON payload to url and records an audit entry. Transport
// errors are logged and swallowed — a notification delivery failure must
// never fail the automation run that triggered it. Shared by fireWebhook,
// fireSlack, and firePagerDuty: what distinguishes them is the target URL
// and payload shape, not the delivery mechanism.
func (e *Engine) postJSON(ctx context.Context, orgID, automationID, assetID, kind, url string, payload any) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "yard-automation/1")
	resp, err := e.client().Do(req)
	if err != nil {
		if e.Log != nil {
			e.Log.Error(kind, "url", url, "err", err)
		}
		return nil
	}
	defer resp.Body.Close()
	_ = e.Store.Audit(ctx, orgID, "automation:"+automationID, kind, assetID, fmt.Sprintf("%s → %d", url, resp.StatusCode))
	return nil
}

func (e *Engine) fireWebhook(ctx context.Context, orgID string, asset *model.Asset, a model.Automation, obs *model.Observation) error {
	url := configString(a.Config, "url")
	if url == "" {
		return nil
	}
	payload := map[string]any{
		"automation": a.Name, "asset_id": asset.ID, "asset_name": asset.Name,
		"capability": obs.Capability, "value": obs.Value, "unit": obs.Unit, "observed_at": obs.ObservedAt,
	}
	return e.postJSON(ctx, orgID, a.ID, asset.ID, "webhook", url, payload)
}

// fireSlack posts to a Slack incoming-webhook URL (config key "slack_url").
// Unverified against a real Slack workspace — no account available to test
// delivery against; validate with real credentials before relying on it.
func (e *Engine) fireSlack(ctx context.Context, orgID string, asset *model.Asset, a model.Automation, obs *model.Observation) error {
	url := configString(a.Config, "slack_url")
	if url == "" {
		return nil
	}
	text := fmt.Sprintf("*%s* on %s — %s = %v%s", a.Name, asset.Name, obs.Capability, obs.Value, obs.Unit)
	return e.postJSON(ctx, orgID, a.ID, asset.ID, "slack", url, map[string]any{"text": text})
}

// firePagerDuty sends a PagerDuty Events API v2 trigger event (config key
// "routing_key"). Unverified against a real PagerDuty account — no account
// available to test delivery against; validate with real credentials
// before relying on it.
func (e *Engine) firePagerDuty(ctx context.Context, orgID string, asset *model.Asset, a model.Automation, obs *model.Observation) error {
	routingKey := configString(a.Config, "routing_key")
	if routingKey == "" {
		return nil
	}
	payload := map[string]any{
		"routing_key":  routingKey,
		"event_action": "trigger",
		"dedup_key":    "automation:" + a.ID + ":" + asset.ID,
		"payload": map[string]any{
			"summary":  fmt.Sprintf("%s on %s", a.Name, asset.Name),
			"source":   asset.Name,
			"severity": "warning",
			"custom_details": map[string]any{
				"capability": obs.Capability, "value": obs.Value, "unit": obs.Unit,
			},
		},
	}
	return e.postJSON(ctx, orgID, a.ID, asset.ID, "pagerduty", "https://events.pagerduty.com/v2/enqueue", payload)
}

// sendEmail sends a plain-text notification via SMTP. The recipient comes
// from the automation's own config (key "to"); the SMTP server itself is a
// server-wide setting via YARD_SMTP_HOST/PORT/USER/PASS/FROM (same
// env-var-configured-integration convention as the ingest token fallback in
// internal/connectors/dispatch.go), since a mail relay is operator
// infrastructure, not a per-rule credential. Unverified against a real SMTP
// account — no account available to test delivery against; validate with
// real credentials before relying on it.
func (e *Engine) sendEmail(ctx context.Context, orgID string, asset *model.Asset, a model.Automation, obs *model.Observation) error {
	to := configString(a.Config, "to")
	host := os.Getenv("YARD_SMTP_HOST")
	if to == "" || host == "" {
		return nil
	}
	port := os.Getenv("YARD_SMTP_PORT")
	if port == "" {
		port = "587"
	}
	from := os.Getenv("YARD_SMTP_FROM")
	if from == "" {
		from = "yard@localhost"
	}
	subject := fmt.Sprintf("[Yard] %s on %s", a.Name, asset.Name)
	body := fmt.Sprintf("%s\r\n\r\nAsset: %s\r\nCapability: %s\r\nValue: %v %s\r\nObserved at: %s\r\n",
		a.Name, asset.Name, obs.Capability, obs.Value, obs.Unit, obs.ObservedAt.Format(time.RFC3339))
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to, subject, body))

	var auth smtp.Auth
	if user := os.Getenv("YARD_SMTP_USER"); user != "" {
		auth = smtp.PlainAuth("", user, os.Getenv("YARD_SMTP_PASS"), host)
	}
	if err := smtp.SendMail(host+":"+port, auth, from, []string{to}, msg); err != nil {
		if e.Log != nil {
			e.Log.Error("email", "to", to, "err", err)
		}
		return nil
	}
	_ = e.Store.Audit(ctx, orgID, "automation:"+a.ID, "email", asset.ID, "sent to "+to)
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
	sev, runbook := e.Store.ResolveSeverity(ctx, orgID, obs.Capability, a.Name)
	if (obs.Value >= 90 || asset.Health == "critical") && sev != "critical" {
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
		Runbook:        runbook,
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
		a.Latitude = existing.Latitude
		a.Longitude = existing.Longitude
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
				sev, runbook := e.Store.ResolveSeverity(ctx, orgID, "heartbeat", a.Name)
				inc := &model.Incident{
					OrganizationID: orgID,
					AssetID:        &asset.ID,
					SiteID:         asset.SiteID,
					Title:          title,
					Severity:       sev,
					Status:         "open",
					Summary:        "Telemetry older than stale_after_sec; heartbeat missing.",
					Runbook:        runbook,
				}
				if err := e.Store.CreateIncident(ctx, inc); err != nil {
					return err
				}
				_, _ = e.Store.InsertEvent(ctx, &model.Event{
					OrganizationID: orgID, AssetID: &asset.ID, Kind: "heartbeat.missed", Severity: sev,
					Title: title, Body: inc.Summary, DedupeKey: "stale:" + asset.ID,
				})
				e.publish(orgID, "incident.opened", inc)
			default:
				_ = e.actionDispatch(ctx, orgID, &asset, a, &model.Observation{Capability: "heartbeat", Value: 0})
			}
		}
	}
	return nil
}
