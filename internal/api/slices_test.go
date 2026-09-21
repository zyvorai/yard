package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/seed"
	"github.com/zyvorai/yard/internal/store"
)

func TestPermitBlocksCompletionUntilApproved(t *testing.T) {
	st, err := store.Open("file:permit?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := seed.Bootstrap(t.Context(), st); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(st, nil).Handler())
	defer ts.Close()
	token := loginToken(t, ts.URL, seed.DemoEmail, seed.DemoPassword)
	wo := postJSON[model.WorkOrder](t, ts.URL, "/api/v1/work-orders", token, map[string]any{
		"title": "Lockout", "checklist": `[{"id":"a","label":"Lockout","done":false}]`,
	}, 201)
	deny := authed(t, http.MethodPatch, ts.URL+"/api/v1/work-orders/"+wo.ID, token, map[string]string{"status": "done"})
	if deny.StatusCode != 409 {
		t.Fatalf("complete without permit %d", deny.StatusCode)
	}
	deny.Body.Close()
	permit := postJSON[store.Permit](t, ts.URL, "/api/v1/work-orders/"+wo.ID+"/permits", token, map[string]any{}, 201)
	postJSON[map[string]string](t, ts.URL, "/api/v1/work-orders/"+wo.ID+"/permits/"+permit.ID+"/approve", token, nil, 200)
	ok := authed(t, http.MethodPatch, ts.URL+"/api/v1/work-orders/"+wo.ID, token, map[string]string{"status": "done"})
	if ok.StatusCode != 200 {
		t.Fatalf("complete with permit %d", ok.StatusCode)
	}
	ok.Body.Close()
}

func TestCostReportAndPackImport(t *testing.T) {
	st, err := store.Open("file:costpack?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	res, err := seed.Bootstrap(t.Context(), st)
	if err != nil || res.User == nil {
		t.Fatal(err)
	}
	org := res.User.OrganizationID
	assets, err := st.ListAssets(t.Context(), org, "", "", "")
	if err != nil || len(assets) == 0 {
		t.Fatal(err)
	}
	asset := assets[0]
	if err := st.SetEnergyCentsPerKWh(t.Context(), org, 12); err != nil {
		t.Fatal(err)
	}
	down, repl := 100, 50000
	if err := st.SetAssetDesired(t.Context(), org, asset.ID, "{}", &down, &repl); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertObservation(t.Context(), &model.Observation{
		OrganizationID: org, AssetID: asset.ID, Capability: "energy_kwh", Value: 5, ValueKind: "number", ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	wo := &model.WorkOrder{OrganizationID: org, AssetID: &asset.ID, Title: "Bearing", Kind: "maintenance"}
	if err := st.CreateWorkOrder(t.Context(), wo); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWorkOrderLine(t.Context(), &store.WorkOrderLine{
		OrganizationID: org, WorkOrderID: wo.ID, Kind: "part", Name: "bearing", Quantity: 2, UnitCostCents: 150,
	}); err != nil {
		t.Fatal(err)
	}
	opened := time.Now().UTC().Add(-2 * time.Hour)
	if err := st.CreateIncident(t.Context(), &model.Incident{
		OrganizationID: org, AssetID: &asset.ID, Title: "Pump down", Severity: "warning", Status: "open", OpenedAt: opened,
	}); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(st, nil).Handler())
	defer ts.Close()
	token := loginToken(t, ts.URL, seed.DemoEmail, seed.DemoPassword)
	report := getJSON[store.CostReport](t, ts.URL, "/api/v1/reports/cost", token)
	if report.EnergyCents != 60 || report.RepairCents != 300 || report.ReplacementCents != 50000 || report.DowntimeCents < 150 {
		t.Fatalf("cost %+v", report)
	}
	imported := postJSON[map[string]any](t, ts.URL, "/api/v1/packs/manufacturing", token, nil, 201)
	if imported["dashboard_id"] == "" {
		t.Fatalf("pack %#v", imported)
	}
	templates, err := st.ListAssetTemplates(t.Context(), org)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range templates {
		if row.Name == "Press" {
			found = true
		}
	}
	if !found {
		t.Fatal("manufacturing template missing")
	}
	hits := getJSON[[]store.SearchHit](t, ts.URL, "/api/v1/search?q=pump", token)
	if len(hits) == 0 {
		t.Fatal("search missed pump")
	}
	meta := getJSON[map[string]any](t, ts.URL, "/api/v1/meta", token)
	if meta["edition"] != "community" {
		t.Fatalf("meta %#v", meta)
	}
}
