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
	"golang.org/x/crypto/bcrypt"
)

func TestPlaybookDryRunApprovalAndStop(t *testing.T) {
	st, err := store.Open("file:playbook?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	res, err := seed.Bootstrap(context.Background(), st)
	if err != nil || res.User == nil {
		t.Fatal(err)
	}
	conn := &model.Connector{OrganizationID: res.User.OrganizationID, Name: "agent", Kind: "device-agent"}
	if err := st.CreateConnector(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("operator-pass"), 4)
	if err != nil {
		t.Fatal(err)
	}
	other := &model.User{OrganizationID: res.User.OrganizationID, Email: "op@yard.local", DisplayName: "Op", Role: "operator", PasswordHash: string(hash), Active: true}
	if err := st.CreateUser(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	srv := New(st, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	admin := loginToken(t, ts.URL, seed.DemoEmail, seed.DemoPassword)
	op := loginToken(t, ts.URL, "op@yard.local", "operator-pass")
	assets := getJSON[[]model.Asset](t, ts.URL, "/api/v1/assets", admin)
	if len(assets) == 0 {
		t.Fatal("no assets")
	}
	yaml := "name: Restart\nsteps:\n  - action: diagnostics.read\n  - action: reboot\n"
	pb := postJSON[model.Playbook](t, ts.URL, "/api/v1/playbooks", admin, map[string]string{"body": yaml}, 201)
	dry := postJSON[map[string]any](t, ts.URL, "/api/v1/playbooks/"+pb.ID+"/run", admin, map[string]any{"asset_id": assets[0].ID, "dry_run": true}, 200)
	if dry["run"] == nil {
		t.Fatalf("dry %#v", dry)
	}
	actions := getJSON[[]model.ActionRequest](t, ts.URL, "/api/v1/actions", admin)
	if len(actions) != 0 {
		t.Fatalf("dry-run created actions: %d", len(actions))
	}
	started := postJSON[map[string]any](t, ts.URL, "/api/v1/playbooks/"+pb.ID+"/run", admin, map[string]any{
		"asset_id": assets[0].ID, "connector_id": conn.ID,
	}, 202)
	runMap, _ := started["run"].(map[string]any)
	runID, _ := runMap["id"].(string)
	if runID == "" || runMap["status"] != "pending_approval" {
		t.Fatalf("run %#v", started)
	}
	deny := authed(t, http.MethodPost, ts.URL+"/api/v1/playbook-runs/"+runID+"/approve", admin, nil)
	if deny.StatusCode != 403 {
		t.Fatalf("requester approve %d", deny.StatusCode)
	}
	deny.Body.Close()
	ok := authed(t, http.MethodPost, ts.URL+"/api/v1/playbook-runs/"+runID+"/approve", op, nil)
	if ok.StatusCode != 200 {
		b, _ := io.ReadAll(ok.Body)
		t.Fatalf("approve %d %s", ok.StatusCode, b)
	}
	ok.Body.Close()
	if err := srv.Queue.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	actions = getJSON[[]model.ActionRequest](t, ts.URL, "/api/v1/actions", admin)
	if len(actions) != 1 || actions[0].Action != "diagnostics.read" {
		t.Fatalf("actions %+v", actions)
	}
	saved, err := st.PlaybookRunByID(context.Background(), res.User.OrganizationID, runID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.StepIndex != 0 || saved.Status != "failed" {
		t.Fatalf("run %+v", saved)
	}
}

func loginToken(t *testing.T, base, email, password string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, err := http.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct{ Token string }
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.Token == "" {
		t.Fatalf("login %d", resp.StatusCode)
	}
	return out.Token
}

func authed(t *testing.T, method, url, token string, body any) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func postJSON[T any](t *testing.T, base, path, token string, body any, want int) T {
	t.Helper()
	resp := authed(t, http.MethodPost, base+path, token, body)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		t.Fatalf("%s %d %s", path, resp.StatusCode, raw)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func getJSON[T any](t *testing.T, base, path, token string) T {
	t.Helper()
	resp := authed(t, http.MethodGet, base+path, token, nil)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("%s %d %s", path, resp.StatusCode, raw)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
