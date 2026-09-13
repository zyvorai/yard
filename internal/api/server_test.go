package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

func TestAssetExportImportAndSeverityPolicies(t *testing.T) {
	st, err := store.Open("file:api-bulk?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := seed.Bootstrap(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(st, nil).Handler())
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"email": seed.DemoEmail, "password": seed.DemoPassword})
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var login struct{ Token string }
	_ = json.NewDecoder(resp.Body).Decode(&login)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/assets/export?format=csv", nil)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	eresp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer eresp.Body.Close()
	if eresp.StatusCode != 200 {
		t.Fatalf("export %d", eresp.StatusCode)
	}
	csvBody, _ := io.ReadAll(eresp.Body)
	if !bytes.Contains(csvBody, []byte("external_ref")) {
		t.Fatalf("csv missing header")
	}

	imp := `[{"name":"Bulk Pump","external_ref":"BULK-1","kind":"machine"}]`
	ireq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/assets/import?format=json", bytes.NewReader([]byte(imp)))
	ireq.Header.Set("Authorization", "Bearer "+login.Token)
	ireq.Header.Set("Content-Type", "application/json")
	iresp, err := http.DefaultClient.Do(ireq)
	if err != nil {
		t.Fatal(err)
	}
	defer iresp.Body.Close()
	if iresp.StatusCode != 200 {
		b, _ := io.ReadAll(iresp.Body)
		t.Fatalf("import %d %s", iresp.StatusCode, b)
	}

	preq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/severity-policies", nil)
	preq.Header.Set("Authorization", "Bearer "+login.Token)
	presp, err := http.DefaultClient.Do(preq)
	if err != nil {
		t.Fatal(err)
	}
	defer presp.Body.Close()
	var policies []map[string]any
	_ = json.NewDecoder(presp.Body).Decode(&policies)
	if len(policies) < 1 {
		t.Fatal("expected seeded severity policies")
	}
}

func TestValidLatLng(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	cases := []struct {
		name     string
		lat, lng *float64
		want     bool
	}{
		{"nil both", nil, nil, true},
		{"in range", f(19.87), f(75.34), true},
		{"lat too high", f(90.1), nil, false},
		{"lat too low", f(-90.1), nil, false},
		{"lng too high", nil, f(180.1), false},
		{"lng too low", nil, f(-180.1), false},
		{"boundary valid positive", f(90), f(180), true},
		{"boundary valid negative", f(-90), f(-180), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := validLatLng(c.lat, c.lng); got != c.want {
				t.Fatalf("validLatLng(%v,%v) = %v, want %v", c.lat, c.lng, got, c.want)
			}
		})
	}
}

func TestIngestInventoryPreservesLocationOnPartialUpdate(t *testing.T) {
	st, err := store.Open("file:api-inventory-loc?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	res, err := seed.Bootstrap(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(st, nil).Handler())
	defer ts.Close()

	post := func(body string) *http.Response {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/ingest/inventory", bytes.NewReader([]byte(body)))
		req.Header.Set("Authorization", "Bearer "+res.IngestToken)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	first := post(`{"external_ref":"LOC-TEST-1","name":"Loc Test","kind":"device","latitude":19.87,"longitude":75.34}`)
	defer first.Body.Close()
	if first.StatusCode != 202 {
		b, _ := io.ReadAll(first.Body)
		t.Fatalf("first ingest %d %s", first.StatusCode, b)
	}

	second := post(`{"external_ref":"LOC-TEST-1","name":"Loc Test","kind":"device","capabilities":["cpu_temp"]}`)
	defer second.Body.Close()
	if second.StatusCode != 202 {
		b, _ := io.ReadAll(second.Body)
		t.Fatalf("second ingest %d %s", second.StatusCode, b)
	}
	var a struct {
		Latitude  *float64 `json:"latitude"`
		Longitude *float64 `json:"longitude"`
	}
	if err := json.NewDecoder(second.Body).Decode(&a); err != nil {
		t.Fatal(err)
	}
	if a.Latitude == nil || a.Longitude == nil {
		t.Fatal("expected location to be preserved after a partial update omitting lat/lng")
	}
	if *a.Latitude != 19.87 || *a.Longitude != 75.34 {
		t.Fatalf("expected preserved coords 19.87,75.34, got %v,%v", *a.Latitude, *a.Longitude)
	}
}

func TestIngestInventoryRejectsOutOfRangeLatLng(t *testing.T) {
	st, err := store.Open("file:api-inventory-badloc?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	res, err := seed.Bootstrap(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(st, nil).Handler())
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/ingest/inventory", bytes.NewReader([]byte(`{"external_ref":"BAD-LOC-1","latitude":999,"longitude":75.34}`)))
	req.Header.Set("Authorization", "Bearer "+res.IngestToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
