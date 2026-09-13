package seed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/zyvorai/estate/internal/idgen"
	"github.com/zyvorai/estate/internal/model"
	"github.com/zyvorai/estate/internal/store"
	"golang.org/x/crypto/bcrypt"
)

const DemoEmail = "admin@estate.local"
const DemoPassword = "estate-admin"

type Result struct {
	Organization *model.Organization
	User         *model.User
	IngestToken  string
	SimulatorTok string
}

func HashToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}

func Bootstrap(ctx context.Context, st *store.Store) (*Result, error) {
	n, err := st.CountOrgs(ctx)
	if err != nil {
		return nil, err
	}
	if n > 0 {
		u, err := st.UserByEmail(ctx, DemoEmail)
		if err != nil {
			return nil, err
		}
		return &Result{User: u}, nil
	}
	org, err := st.CreateOrganization(ctx, "Northwind Operations", "northwind")
	if err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(DemoPassword), 10)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		OrganizationID: org.ID,
		Email:          DemoEmail,
		DisplayName:    "Operations Admin",
		Role:           "admin",
		PasswordHash:   string(hash),
	}
	if err := st.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	lat1, lng1 := 19.8762, 75.3433
	lat2, lng2 := 19.9012, 75.3128
	plant := &model.Site{OrganizationID: org.ID, Name: "Riverside Plant", Kind: "factory", Address: "MIDC Waluj, Aurangabad", Latitude: &lat1, Longitude: &lng1, Timezone: "Asia/Kolkata"}
	wh := &model.Site{OrganizationID: org.ID, Name: "West Warehouse", Kind: "warehouse", Address: "Chikalthana, Aurangabad", Latitude: &lat2, Longitude: &lng2, Timezone: "Asia/Kolkata"}
	if err := st.UpsertSite(ctx, plant); err != nil {
		return nil, err
	}
	if err := st.UpsertSite(ctx, wh); err != nil {
		return nil, err
	}

	type spec struct {
		name, ref, kind, mfr, model, serial, site string
		lat, lng                                  float64
		caps                                      []model.Capability
	}
	assets := []spec{
		{"Edge gateway ZY-GW-0001", "ZY-GW-0001", "device", "Zyvor", "Device Agent", "DA-9188-0001", plant.ID, 19.8764, 75.3431, []model.Capability{
			{Name: "cpu_temp", Kind: "measurement", Unit: "°C"},
			{Name: "heartbeat", Kind: "presence", Unit: "1"},
			{Name: "uptime", Kind: "measurement", Unit: "s"},
		}},
		{"Cold room sensor CR-12", "ZY-SEN-0412", "sensor", "Sensirion", "SHT40", "SHT-0412", plant.ID, 19.8760, 75.3436, []model.Capability{
			{Name: "temperature", Kind: "measurement", Unit: "°C", Max: f64(8)},
			{Name: "humidity", Kind: "measurement", Unit: "%"},
		}},
		{"Process pump P-03", "ZY-PUMP-03", "machine", "Grundfos", "CR 15-3", "GF-8803", plant.ID, 19.8768, 75.3440, []model.Capability{
			{Name: "temperature", Kind: "measurement", Unit: "°C", Max: f64(80)},
			{Name: "vibration", Kind: "measurement", Unit: "mm/s"},
			{Name: "heartbeat", Kind: "presence", Unit: "1"},
		}},
		{"CNC mill Line A", "ZY-CNC-08", "machine", "Haas", "VF-2", "HAAS-2208", plant.ID, 19.8758, 75.3428, []model.Capability{
			{Name: "spindle_load", Kind: "measurement", Unit: "%"},
			{Name: "heartbeat", Kind: "presence", Unit: "1"},
		}},
		{"Service van 17", "ZY-VEH-17", "vehicle", "Tata", "Ace", "MH20-AB-4417", wh.ID, 19.9010, 75.3132, []model.Capability{
			{Name: "speed", Kind: "measurement", Unit: "km/h"},
			{Name: "fuel", Kind: "measurement", Unit: "%"},
			{Name: "heartbeat", Kind: "presence", Unit: "1"},
		}},
		{"Dock door sensor D-2", "ZY-SEN-D2", "sensor", "Honeywell", "Limit", "LIM-D2", wh.ID, 19.9014, 75.3124, []model.Capability{
			{Name: "open", Kind: "state", Unit: "1"},
			{Name: "heartbeat", Kind: "presence", Unit: "1"},
		}},
		{"Simulator thermal load", "SIM-TEMP-A", "equipment", "Estate", "Sim v1", "SIM-A", plant.ID, 19.8765, 75.3438, []model.Capability{
			{Name: "temperature", Kind: "measurement", Unit: "°C", Max: f64(75)},
			{Name: "heartbeat", Kind: "presence", Unit: "1"},
		}},
	}
	for _, sp := range assets {
		siteID := sp.site
		lat, lng := sp.lat, sp.lng
		a := &model.Asset{
			OrganizationID: org.ID,
			SiteID:         &siteID,
			Name:           sp.name,
			ExternalRef:    sp.ref,
			Kind:           sp.kind,
			Status:         "active",
			Health:         "healthy",
			Manufacturer:   sp.mfr,
			Model:          sp.model,
			Serial:         sp.serial,
			Latitude:       &lat,
			Longitude:      &lng,
			StaleAfterSec:  90,
			Metadata:       `{"source":"seed"}`,
		}
		if err := st.UpsertAsset(ctx, a); err != nil {
			return nil, err
		}
		for i := range sp.caps {
			sp.caps[i].AssetID = a.ID
		}
		if err := st.ReplaceCapabilities(ctx, a.ID, sp.caps); err != nil {
			return nil, err
		}
	}

	ingestTok := "est_ingest_" + idgen.Secret(16)
	simTok := "est_sim_" + idgen.Secret(16)
	httpConn := &model.Connector{
		OrganizationID: org.ID,
		Name:           "Generic HTTP ingestion",
		Kind:           "http",
		Status:         "connected",
		Endpoint:       "/api/v1/ingest",
		TokenHint:      ingestTok[:12] + "…",
		TokenHash:      HashToken(ingestTok),
		Actions:        `["ingest.observation","ingest.inventory","ingest.event"]`,
	}
	simConn := &model.Connector{
		OrganizationID: org.ID,
		Name:           "Included simulator",
		Kind:           "simulator",
		Status:         "connected",
		Endpoint:       "internal://simulator",
		TokenHint:      simTok[:12] + "…",
		TokenHash:      HashToken(simTok),
		Actions:        `["ingest.observation","ingest.inventory"]`,
	}
	agentConn := &model.Connector{
		OrganizationID: org.ID,
		Name:           "Device Agent gateway",
		Kind:           "device-agent",
		Status:         "pending",
		Endpoint:       "http://127.0.0.1:9188",
		Actions:        `["inventory.refresh","diagnostics.read"]`,
		Config:         `{"poll_seconds":15,"path_inventory":"/api/v1/inventory","path_sensors":"/api/v1/sensors"}`,
	}
	nodra := &model.Connector{OrganizationID: org.ID, Name: "Nodra (optional)", Kind: "nodra", Status: "available", Endpoint: "", Actions: `["telemetry.receive"]`, Config: `{"optional":true}`}
	fleet := &model.Connector{OrganizationID: org.ID, Name: "Zyvor Fleet (optional)", Kind: "fleet", Status: "available", Endpoint: "", Actions: `["lifecycle.request","desired.progress"]`, Config: `{"optional":true}`}
	ota := &model.Connector{OrganizationID: org.ID, Name: "OTA campaigns (optional)", Kind: "ota", Status: "available", Endpoint: "", Actions: `["campaign.list","update.delegate"]`, Config: `{"optional":true}`}
	for _, c := range []*model.Connector{httpConn, simConn, agentConn, nodra, fleet, ota} {
		if err := st.CreateConnector(ctx, c); err != nil {
			return nil, err
		}
	}

	autos := []model.Automation{
		{OrganizationID: org.ID, Name: "High temperature incident", Enabled: true, TriggerKind: "threshold", Capability: "temperature", Operator: "gt", Threshold: 75, Action: "open_incident"},
		{OrganizationID: org.ID, Name: "Missed heartbeat", Enabled: true, TriggerKind: "stale", Capability: "heartbeat", Operator: "missing", Threshold: 90, Action: "open_incident"},
		{OrganizationID: org.ID, Name: "Notify on critical event", Enabled: true, TriggerKind: "event", Capability: "", Operator: "eq", Threshold: 0, Action: "notify"},
	}
	for i := range autos {
		if err := st.CreateAutomation(ctx, &autos[i]); err != nil {
			return nil, err
		}
	}

	_ = st.Audit(ctx, org.ID, user.Email, "workspace.create", org.ID, "Seeded first workspace, sites, assets, connectors")
	return &Result{Organization: org, User: user, IngestToken: ingestTok, SimulatorTok: simTok}, nil
}

func f64(v float64) *float64 { return &v }

func FormatWelcome(r *Result) string {
	if r.IngestToken == "" {
		return "workspace already exists"
	}
	return fmt.Sprintf("workspace=%s ingest=%s simulator=%s login=%s / %s", r.Organization.Slug, r.IngestToken, r.SimulatorTok, DemoEmail, DemoPassword)
}
