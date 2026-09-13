package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zyvorai/yard/internal/seed"
	"github.com/zyvorai/yard/internal/store"
)

func TestLoginAndFirstReleaseWorkflow(t *testing.T) {
	st, err := store.Open("file:api-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	res, err := seed.Bootstrap(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	srv := New(st, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"email": seed.DemoEmail, "password": seed.DemoPassword})
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login %d", resp.StatusCode)
	}
	var login struct{ Token string }
	_ = json.NewDecoder(resp.Body).Decode(&login)
	if login.Token == "" {
		t.Fatal("missing token")
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/assets", nil)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	aresp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer aresp.Body.Close()
	var assets []map[string]any
	_ = json.NewDecoder(aresp.Body).Decode(&assets)
	if len(assets) < 5 {
		t.Fatalf("expected seeded assets, got %d", len(assets))
	}

	obs, _ := json.Marshal([]map[string]any{{
		"asset_external_ref": "SIM-TEMP-A",
		"capability":         "temperature",
		"value":              88.4,
		"unit":               "°C",
		"source":             "test",
		"dedupe_key":         "wf-1",
	}})
	ireq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/ingest/observations", bytes.NewReader(obs))
	ireq.Header.Set("Authorization", "Bearer "+res.SimulatorTok)
	iresp, err := http.DefaultClient.Do(ireq)
	if err != nil {
		t.Fatal(err)
	}
	defer iresp.Body.Close()
	if iresp.StatusCode != 202 {
		t.Fatalf("ingest %d", iresp.StatusCode)
	}

	ireq2, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/incidents?status=open", nil)
	ireq2.Header.Set("Authorization", "Bearer "+login.Token)
	i2, err := http.DefaultClient.Do(ireq2)
	if err != nil {
		t.Fatal(err)
	}
	defer i2.Body.Close()
	var incs []map[string]any
	_ = json.NewDecoder(i2.Body).Decode(&incs)
	if len(incs) == 0 {
		t.Fatal("expected automation to open an incident")
	}
	incID, _ := incs[0]["id"].(string)

	wo, _ := json.Marshal(map[string]any{
		"title":       "Inspect thermal load",
		"kind":        "maintenance",
		"priority":    "high",
		"incident_id": incID,
		"asset_id":    incs[0]["asset_id"],
		"assignee":    "Maya Chen",
	})
	wreq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/work-orders", bytes.NewReader(wo))
	wreq.Header.Set("Authorization", "Bearer "+login.Token)
	wresp, err := http.DefaultClient.Do(wreq)
	if err != nil {
		t.Fatal(err)
	}
	defer wresp.Body.Close()
	if wresp.StatusCode != 201 {
		t.Fatalf("work order %d", wresp.StatusCode)
	}

	patch, _ := json.Marshal(map[string]string{"status": "resolved", "resolution": "Replaced intake filter; temperature returned to band."})
	preq, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/incidents/"+incID, bytes.NewReader(patch))
	preq.Header.Set("Authorization", "Bearer "+login.Token)
	presp, err := http.DefaultClient.Do(preq)
	if err != nil {
		t.Fatal(err)
	}
	defer presp.Body.Close()
	if presp.StatusCode != 200 {
		t.Fatalf("resolve %d", presp.StatusCode)
	}

	bad := []byte(`{"email":"admin@yard.local","password":"nope"}`)
	bresp, _ := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(bad))
	if bresp.StatusCode != 401 {
		t.Fatalf("bad login %d", bresp.StatusCode)
	}
	bresp.Body.Close()
}

func TestConnectorAuthRequired(t *testing.T) {
	st, err := store.Open("file:api-auth?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := seed.Bootstrap(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(st, nil).Handler())
	defer ts.Close()
	resp, err := http.Post(ts.URL+"/api/v1/ingest/observations", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
