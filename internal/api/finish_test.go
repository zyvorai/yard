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
	"github.com/zyvorai/yard/internal/totp"
	"golang.org/x/crypto/bcrypt"
)

func TestTOTPComplianceQueryAndPeer(t *testing.T) {
	st, err := store.Open("file:finish?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	res, err := seed.Bootstrap(t.Context(), st)
	if err != nil || res.User == nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("viewer-pass"), 4)
	if err != nil {
		t.Fatal(err)
	}
	viewerUser := &model.User{OrganizationID: res.User.OrganizationID, Email: "view@yard.local", DisplayName: "Viewer", Role: "viewer", PasswordHash: string(hash), Active: true}
	if err := st.CreateUser(t.Context(), viewerUser); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YARD_PEER_TOKEN", "peer-token")
	t.Setenv("YARD_REGION", "west")
	srv := New(st, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	admin := loginToken(t, ts.URL, seed.DemoEmail, seed.DemoPassword)
	setup := postJSON[map[string]string](t, ts.URL, "/api/v1/auth/totp/setup", admin, map[string]string{}, 200)
	code, err := totp.Code(setup["secret"], time.Now())
	if err != nil {
		t.Fatal(err)
	}
	postJSON[map[string]any](t, ts.URL, "/api/v1/auth/totp/confirm", admin, map[string]string{"code": code}, 200)
	denied := authed(t, http.MethodPost, ts.URL+"/api/v1/auth/login", "", map[string]string{"email": seed.DemoEmail, "password": seed.DemoPassword})
	raw, _ := io.ReadAll(denied.Body)
	denied.Body.Close()
	if denied.StatusCode != 401 || !strings.Contains(string(raw), "otp required") {
		t.Fatalf("login without otp %d %s", denied.StatusCode, raw)
	}
	allowed := authed(t, http.MethodPost, ts.URL+"/api/v1/auth/login", "", map[string]string{"email": seed.DemoEmail, "password": seed.DemoPassword, "otp": code})
	allowed.Body.Close()
	if allowed.StatusCode != 200 {
		t.Fatalf("login with otp %d", allowed.StatusCode)
	}
	viewer := loginToken(t, ts.URL, "view@yard.local", "viewer-pass")
	resp := authed(t, http.MethodGet, ts.URL+"/api/v1/compliance/export", viewer, nil)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("viewer export %d", resp.StatusCode)
	}
	exp := authed(t, http.MethodGet, ts.URL+"/api/v1/compliance/export", admin, nil)
	csv, _ := io.ReadAll(exp.Body)
	exp.Body.Close()
	if exp.StatusCode != 200 || !strings.Contains(string(csv), "edition,community") || !strings.Contains(string(csv), seed.DemoEmail) {
		t.Fatalf("export %d %s", exp.StatusCode, csv)
	}
	assets := getJSON[[]model.Asset](t, ts.URL, "/api/v1/assets", admin)
	if len(assets) == 0 {
		t.Fatal("no assets")
	}
	if _, err := st.InsertObservation(t.Context(), &model.Observation{
		OrganizationID: res.User.OrganizationID, AssetID: assets[0].ID, Capability: "latency", Value: 3, ValueKind: "histogram", ValueText: `{"count":1,"sum":3,"buckets":[1]}`, ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	rows := getJSON[[]model.Observation](t, ts.URL, "/api/v1/telemetry/query?capability=latency&asset_id="+assets[0].ID, admin)
	if len(rows) != 1 || rows[0].ValueKind != "histogram" {
		t.Fatalf("query %+v", rows)
	}
	peerBody := map[string]any{"events": []model.Event{{OrganizationID: res.User.OrganizationID, Kind: "note", Severity: "info", Title: "from east", Region: "east"}}}
	bad := authed(t, http.MethodPost, ts.URL+"/api/v1/peers/events", "nope", peerBody)
	bad.Body.Close()
	if bad.StatusCode != 401 {
		t.Fatalf("peer auth %d", bad.StatusCode)
	}
	okResp := authed(t, http.MethodPost, ts.URL+"/api/v1/peers/events", "peer-token", peerBody)
	okRaw, _ := io.ReadAll(okResp.Body)
	okResp.Body.Close()
	if okResp.StatusCode != 202 || !strings.Contains(string(okRaw), `"accepted":1`) {
		t.Fatalf("peer %d %s", okResp.StatusCode, okRaw)
	}
}
