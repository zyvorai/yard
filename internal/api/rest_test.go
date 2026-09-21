package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/seed"
	"github.com/zyvorai/yard/internal/store"
)

func TestRoadmapRemainder(t *testing.T) {
	st, err := store.Open("file:roadmap?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	res, err := seed.Bootstrap(t.Context(), st)
	if err != nil || res.User == nil {
		t.Fatal(err)
	}
	srv := New(st, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	admin := loginToken(t, ts.URL, seed.DemoEmail, seed.DemoPassword)
	assets := getJSON[[]model.Asset](t, ts.URL, "/api/v1/assets", admin)
	if len(assets) == 0 {
		t.Fatal("no assets")
	}
	a := assets[0]
	parent := assets[1].ID
	bought := time.Now().UTC().Add(-365 * 24 * time.Hour)
	warranty := time.Now().UTC().Add(30 * 24 * time.Hour)
	patched := authed(t, http.MethodPatch, ts.URL+"/api/v1/assets/"+a.ID, admin, map[string]any{
		"nfc_id": "nfc-42", "parent_asset_id": parent,
		"purchased_at": bought, "warranty_expires_at": warranty, "purchase_cents": 120000, "useful_life_months": 60,
	})
	patched.Body.Close()
	if patched.StatusCode != 200 {
		t.Fatalf("patch %d", patched.StatusCode)
	}
	got := getJSON[model.Asset](t, ts.URL, "/api/v1/assets/lookup?nfc=nfc-42", admin)
	if got.ID != a.ID || got.BookValueCents <= 0 || got.BookValueCents >= 120000 {
		t.Fatalf("nfc/warranty %+v", got)
	}
	installs := getJSON[[]store.InstallRow](t, ts.URL, "/api/v1/assets/"+a.ID+"/installs", admin)
	if len(installs) == 0 {
		t.Fatal("install history empty")
	}
	postJSON[store.BOMLine](t, ts.URL, "/api/v1/assets/"+a.ID+"/bom", admin, map[string]any{"part_number": "BRG-1", "name": "Bearing", "quantity": 2}, 201)
	bom := getJSON[[]store.BOMLine](t, ts.URL, "/api/v1/assets/"+a.ID+"/bom", admin)
	if len(bom) != 1 {
		t.Fatalf("bom %+v", bom)
	}
	postJSON[store.Catalog](t, ts.URL, "/api/v1/catalogs", admin, map[string]any{"name": "Pumps", "template_ids": []string{}}, 201)
	wo := postJSON[model.WorkOrder](t, ts.URL, "/api/v1/work-orders", admin, map[string]string{"title": "Multi asset job"}, 201)
	put := authed(t, http.MethodPut, ts.URL+"/api/v1/work-orders/"+wo.ID+"/assets", admin, map[string]any{"asset_ids": []string{a.ID, parent}})
	put.Body.Close()
	if put.StatusCode != 200 {
		t.Fatalf("assets %d", put.StatusCode)
	}
	postJSON[store.TimeEntry](t, ts.URL, "/api/v1/work-orders/"+wo.ID+"/time", admin, map[string]any{"minutes": 45, "note": "on site"}, 201)
	printResp := authed(t, http.MethodGet, ts.URL+"/api/v1/work-orders/"+wo.ID+"/print", admin, nil)
	printBody, _ := io.ReadAll(printResp.Body)
	printResp.Body.Close()
	if printResp.StatusCode != 200 || !strings.Contains(string(printBody), wo.Title) {
		t.Fatalf("print %d %s", printResp.StatusCode, printBody)
	}
	del := authed(t, http.MethodDelete, ts.URL+"/api/v1/assets/"+a.ID, admin, nil)
	del.Body.Close()
	if del.StatusCode != 200 {
		t.Fatalf("delete %d", del.StatusCode)
	}
	listed := getJSON[[]model.Asset](t, ts.URL, "/api/v1/assets", admin)
	for _, row := range listed {
		if row.ID == a.ID {
			t.Fatal("soft-deleted asset still listed")
		}
	}
	csv := authed(t, http.MethodGet, ts.URL+"/api/v1/telemetry/export?format=csv", admin, nil)
	raw, _ := io.ReadAll(csv.Body)
	csv.Body.Close()
	if csv.StatusCode != 200 || !strings.Contains(string(raw), "observed_at") {
		t.Fatalf("export %d %s", csv.StatusCode, raw)
	}
	postJSON[store.SavedView](t, ts.URL, "/api/v1/saved-views", admin, map[string]string{"name": "Critical", "kind": "assets", "query": `{"health":"critical"}`}, 201)
	orgs := getJSON[[]model.Organization](t, ts.URL, "/api/v1/orgs", admin)
	if len(orgs) == 0 {
		t.Fatal("no orgs")
	}
	prefs := authed(t, http.MethodPatch, ts.URL+"/api/v1/me/prefs", admin, map[string]any{"widgets": []string{"health", "incidents"}, "locale": "hi"})
	prefs.Body.Close()
	if prefs.StatusCode != 200 {
		t.Fatalf("prefs %d", prefs.StatusCode)
	}
	i18n := getJSON[map[string]any](t, ts.URL, "/api/v1/i18n?lang=hi", "")
	if i18n["lang"] != "hi" {
		t.Fatalf("i18n %#v", i18n)
	}
	sched := getJSON[[]model.WorkOrder](t, ts.URL, "/api/v1/schedules", admin)
	_ = sched
	bbox := getJSON[[]model.Asset](t, ts.URL, "/api/v1/assets?bbox=75.3,19.87,75.35,19.88", admin)
	if len(bbox) == 0 {
		t.Fatal("bbox empty")
	}
}
