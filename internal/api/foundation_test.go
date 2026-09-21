package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/config"
	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/seed"
	"github.com/zyvorai/yard/internal/store"
	"golang.org/x/crypto/bcrypt"
)

func TestRBACMatrixViewerForbidden(t *testing.T) {
	st, err := store.Open("file:rbac-matrix?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := seed.Bootstrap(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	orgs, _ := st.CountOrgs(context.Background())
	if orgs == 0 {
		t.Fatal("expected seed org")
	}
	admin, err := st.UserByEmail(context.Background(), seed.DemoEmail)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("viewer-pass-9"), 10)
	viewer := &model.User{
		OrganizationID: admin.OrganizationID,
		Email:          "viewer@yard.local",
		DisplayName:    "Viewer",
		Role:           "viewer",
		Active:         true,
		PasswordHash:   string(hash),
	}
	if err := st.CreateUser(context.Background(), viewer); err != nil {
		t.Fatal(err)
	}
	sess, err := st.CreateSession(context.Background(), viewer.ID, 12*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	srv := New(st, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	forbidden := []struct {
		method, path, body string
	}{
		{"POST", "/api/v1/incidents", `{"title":"x","severity":"warning"}`},
		{"POST", "/api/v1/actions", `{"action":"diagnostics.read"}`},
		{"PATCH", "/api/v1/connectors", `{"id":"missing"}`},
		{"POST", "/api/v1/sites", `{"name":"x"}`},
		{"POST", "/api/v1/work-orders", `{"title":"x"}`},
		{"GET", "/api/v1/audit", ""},
	}
	for _, tc := range forbidden {
		var body *bytes.Reader
		if tc.body != "" {
			body = bytes.NewReader([]byte(tc.body))
		} else {
			body = bytes.NewReader(nil)
		}
		req, _ := http.NewRequest(tc.method, ts.URL+tc.path, body)
		req.Header.Set("Authorization", "Bearer "+sess.Token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Fatalf("%s %s: want 403, got %d", tc.method, tc.path, resp.StatusCode)
		}
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/assets", nil)
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("viewer GET assets: %d", resp.StatusCode)
	}
}

func TestProductionBootstrapAndMeta(t *testing.T) {
	st, err := store.Open("file:prod-boot?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	cfg := config.Config{Mode: "production", PublicURL: "https://yard.example", SecretKey: key, Egress: config.DemoConfig().Egress}
	res, err := seed.BootstrapWith(context.Background(), st, seed.Options{
		Mode: "production", BootstrapEmail: "ops@example.com", BootstrapPassword: "secure-pass-99", PublicURL: cfg.PublicURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.User == nil || res.User.Email != "ops@example.com" {
		t.Fatalf("unexpected user %#v", res.User)
	}
	if _, err := st.UserByEmail(context.Background(), seed.DemoEmail); err == nil {
		t.Fatal("production must not create demo admin")
	}
	srv := NewWith(st, nil, cfg)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/api/v1/meta")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var meta map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&meta)
	if meta["mode"] != "production" {
		t.Fatalf("meta %#v", meta)
	}
	h := resp.Header.Get("X-Content-Type-Options")
	if h != "nosniff" {
		t.Fatalf("missing security headers: %q", h)
	}
}

func TestConnectorSecretRedacted(t *testing.T) {
	st, err := store.Open("file:secret-redact?mode=memory&cache=shared")
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
	loginResp, _ := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	var login struct{ Token string }
	_ = json.NewDecoder(loginResp.Body).Decode(&login)
	loginResp.Body.Close()
	list, err := st.ListConnectors(context.Background(), res.User.OrganizationID)
	if err != nil || len(list) == 0 {
		t.Fatal(err)
	}
	conn := list[0]
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/connectors/"+conn.ID+"/secret", bytes.NewReader([]byte(`{"secret":"super-secret-token"}`)))
	req.Header.Set("Authorization", "Bearer "+login.Token)
	req.Header.Set("Content-Type", "application/json")
	putResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	putResp.Body.Close()
	if putResp.StatusCode != 200 {
		t.Fatalf("put secret %d", putResp.StatusCode)
	}
	bad, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/connectors", bytes.NewReader([]byte(`{"id":"`+conn.ID+`","config":"{\"auth_token\":\"leak\"}"}`)))
	bad.Header.Set("Authorization", "Bearer "+login.Token)
	bad.Header.Set("Content-Type", "application/json")
	badResp, _ := http.DefaultClient.Do(bad)
	badBody, _ := io.ReadAll(badResp.Body)
	badResp.Body.Close()
	if badResp.StatusCode != 400 {
		t.Fatalf("expected 400 for auth_token in config, got %d %s", badResp.StatusCode, badBody)
	}
	getReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/connectors", nil)
	getReq.Header.Set("Authorization", "Bearer "+login.Token)
	getResp, _ := http.DefaultClient.Do(getReq)
	raw, _ := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	if strings.Contains(string(raw), "super-secret-token") || strings.Contains(string(raw), `"auth_token"`) {
		t.Fatalf("secret leaked in response: %s", raw)
	}
}

func TestStreamTicketNotSessionToken(t *testing.T) {
	st, err := store.Open("file:sse-ticket?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := seed.Bootstrap(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	srv := New(st, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	body, _ := json.Marshal(map[string]string{"email": seed.DemoEmail, "password": seed.DemoPassword})
	loginResp, _ := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	var login struct{ Token string }
	_ = json.NewDecoder(loginResp.Body).Decode(&login)
	loginResp.Body.Close()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/stream?token="+login.Token, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("session token on stream must be rejected, got %d", resp.StatusCode)
	}
	ticketReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/stream/ticket", nil)
	ticketReq.Header.Set("Authorization", "Bearer "+login.Token)
	ticketResp, err := http.DefaultClient.Do(ticketReq)
	if err != nil {
		t.Fatal(err)
	}
	var ticket struct{ Ticket string }
	_ = json.NewDecoder(ticketResp.Body).Decode(&ticket)
	ticketResp.Body.Close()
	if ticket.Ticket == "" {
		t.Fatal("missing ticket")
	}
}
