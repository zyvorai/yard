package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/seed"
	"github.com/zyvorai/yard/internal/store"
)

func TestRoleOKEmptyRoleIsRestrictive(t *testing.T) {
	u := &model.User{Role: ""}
	if roleOK(u, true) {
		t.Fatal("an empty/unrecognized role must not be granted write access")
	}
	if !roleOK(u, false) {
		t.Fatal("read access should still be allowed regardless of role")
	}
}

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

// TestStreamSupportsFlushThroughMiddleware guards against a real regression:
// wrapping the ResponseWriter for request logging (requestLog/statusRecorder
// in Handler()) must still satisfy http.Flusher, since the stream handler
// type-asserts it and returns 500 outright if the assertion fails. A test
// against the handler function directly (bypassing Handler()'s middleware
// chain) would not have caught this.
func TestStreamSupportsFlushThroughMiddleware(t *testing.T) {
	st, err := store.Open("file:api-test-stream?mode=memory&cache=shared")
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

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/stream", nil)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	sresp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer sresp.Body.Close()
	if sresp.StatusCode != 200 {
		t.Fatalf("expected 200 from /api/v1/stream, got %d", sresp.StatusCode)
	}
	want := "event: ready\ndata: {}\n\n"
	buf := make([]byte, len(want))
	if _, err := io.ReadFull(sresp.Body, buf); err != nil {
		t.Fatalf("reading initial SSE event: %v", err)
	}
	if string(buf) != want {
		t.Fatalf("unexpected initial SSE event: %q", buf)
	}
}

func loginAs(t *testing.T, ts *httptest.Server, email, password string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login %q: expected 200, got %d", email, resp.StatusCode)
	}
	var login struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&login)
	return login.Token
}

func TestInviteAcceptRoleAndDeactivate(t *testing.T) {
	st, err := store.Open("file:api-test-invite?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := seed.Bootstrap(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(st, nil).Handler())
	defer ts.Close()

	adminToken := loginAs(t, ts, seed.DemoEmail, seed.DemoPassword)

	inviteBody, _ := json.Marshal(map[string]string{"email": "newop@example.com", "display_name": "New Op", "role": "operator"})
	ireq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/users", bytes.NewReader(inviteBody))
	ireq.Header.Set("Authorization", "Bearer "+adminToken)
	iresp, err := http.DefaultClient.Do(ireq)
	if err != nil {
		t.Fatal(err)
	}
	defer iresp.Body.Close()
	if iresp.StatusCode != 201 {
		t.Fatalf("invite: expected 201, got %d", iresp.StatusCode)
	}
	var invited struct {
		User        model.User `json:"user"`
		InviteToken string     `json:"invite_token"`
	}
	_ = json.NewDecoder(iresp.Body).Decode(&invited)
	if invited.InviteToken == "" || invited.User.ID == "" {
		t.Fatal("missing invite token or user id")
	}

	acceptBody, _ := json.Marshal(map[string]string{"token": invited.InviteToken, "password": "correct horse battery staple"})
	aresp, err := http.Post(ts.URL+"/api/v1/auth/accept-invite", "application/json", bytes.NewReader(acceptBody))
	if err != nil {
		t.Fatal(err)
	}
	defer aresp.Body.Close()
	if aresp.StatusCode != 200 {
		t.Fatalf("accept-invite: expected 200, got %d", aresp.StatusCode)
	}
	var accepted struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(aresp.Body).Decode(&accepted)
	if accepted.Token == "" {
		t.Fatal("accept-invite did not return a session token")
	}

	replay, err := http.Post(ts.URL+"/api/v1/auth/accept-invite", "application/json", bytes.NewReader(acceptBody))
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Body.Close()
	if replay.StatusCode != 401 {
		t.Fatalf("replayed invite token: expected 401, got %d", replay.StatusCode)
	}

	opInviteBody, _ := json.Marshal(map[string]string{"email": "blocked@example.com"})
	opReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/admin/users", bytes.NewReader(opInviteBody))
	opReq.Header.Set("Authorization", "Bearer "+accepted.Token)
	opResp, err := http.DefaultClient.Do(opReq)
	if err != nil {
		t.Fatal(err)
	}
	defer opResp.Body.Close()
	if opResp.StatusCode != 403 {
		t.Fatalf("operator invite: expected 403 (admin-only), got %d", opResp.StatusCode)
	}

	// Deactivating the user must invalidate their existing session
	// immediately, not just block their next login.
	deactivateBody, _ := json.Marshal(map[string]any{"id": invited.User.ID, "active": false})
	dreq, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/admin/users", bytes.NewReader(deactivateBody))
	dreq.Header.Set("Authorization", "Bearer "+adminToken)
	dresp, err := http.DefaultClient.Do(dreq)
	if err != nil {
		t.Fatal(err)
	}
	defer dresp.Body.Close()
	if dresp.StatusCode != 200 {
		t.Fatalf("deactivate: expected 200, got %d", dresp.StatusCode)
	}

	meReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+accepted.Token)
	meResp, err := http.DefaultClient.Do(meReq)
	if err != nil {
		t.Fatal(err)
	}
	defer meResp.Body.Close()
	if meResp.StatusCode != 401 {
		t.Fatalf("deactivated user's session: expected 401, got %d", meResp.StatusCode)
	}
}

func TestAPIKeyAuthenticatesLikeASession(t *testing.T) {
	st, err := store.Open("file:api-test-apikey?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := seed.Bootstrap(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(st, nil).Handler())
	defer ts.Close()

	token := loginAs(t, ts, seed.DemoEmail, seed.DemoPassword)

	createBody, _ := json.Marshal(map[string]string{"name": "CI script"})
	creq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/api-keys", bytes.NewReader(createBody))
	creq.Header.Set("Authorization", "Bearer "+token)
	cresp, err := http.DefaultClient.Do(creq)
	if err != nil {
		t.Fatal(err)
	}
	defer cresp.Body.Close()
	if cresp.StatusCode != 201 {
		t.Fatalf("create key: expected 201, got %d", cresp.StatusCode)
	}
	var created struct {
		APIKey struct {
			ID string `json:"id"`
		} `json:"api_key"`
		Token string `json:"token"`
	}
	_ = json.NewDecoder(cresp.Body).Decode(&created)
	if created.Token == "" || created.APIKey.ID == "" {
		t.Fatal("missing api key token or id")
	}

	meReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+created.Token)
	meResp, err := http.DefaultClient.Do(meReq)
	if err != nil {
		t.Fatal(err)
	}
	defer meResp.Body.Close()
	if meResp.StatusCode != 200 {
		t.Fatalf("api key auth: expected 200, got %d", meResp.StatusCode)
	}
	var me model.User
	_ = json.NewDecoder(meResp.Body).Decode(&me)
	if me.Email != seed.DemoEmail {
		t.Fatalf("api key resolved to wrong user: %s", me.Email)
	}

	deleteBody, _ := json.Marshal(map[string]any{"id": created.APIKey.ID, "delete": true})
	dreq, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/api-keys", bytes.NewReader(deleteBody))
	dreq.Header.Set("Authorization", "Bearer "+token)
	dresp, err := http.DefaultClient.Do(dreq)
	if err != nil {
		t.Fatal(err)
	}
	defer dresp.Body.Close()
	if dresp.StatusCode != 200 {
		t.Fatalf("delete key: expected 200, got %d", dresp.StatusCode)
	}

	me2Req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/auth/me", nil)
	me2Req.Header.Set("Authorization", "Bearer "+created.Token)
	me2Resp, err := http.DefaultClient.Do(me2Req)
	if err != nil {
		t.Fatal(err)
	}
	defer me2Resp.Body.Close()
	if me2Resp.StatusCode != 401 {
		t.Fatalf("revoked api key: expected 401, got %d", me2Resp.StatusCode)
	}
}
