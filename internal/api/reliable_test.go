package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/seed"
	"github.com/zyvorai/yard/internal/store"
	"golang.org/x/crypto/bcrypt"
)

func TestDangerousActionNeedsSecondApprover(t *testing.T) {
	st, err := store.Open("file:approve?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := seed.Bootstrap(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	admin, err := st.UserByEmail(context.Background(), seed.DemoEmail)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("operator-pass-9"), 10)
	op := &model.User{
		OrganizationID: admin.OrganizationID, Email: "operator-approve@yard.local",
		DisplayName: "Op", Role: "operator", Active: true, PasswordHash: string(hash),
	}
	if err := st.CreateUser(context.Background(), op); err != nil {
		t.Fatal(err)
	}
	adminSess, _ := st.CreateSession(context.Background(), admin.ID, 12*time.Hour)
	opSess, _ := st.CreateSession(context.Background(), op.ID, 12*time.Hour)
	conns, err := st.ListConnectors(context.Background(), admin.OrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	var fleetID string
	for _, c := range conns {
		if c.Kind == "fleet" {
			fleetID = c.ID
		}
	}
	if fleetID == "" {
		t.Fatal("missing fleet connector")
	}
	srv := New(st, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/actions", bytes.NewReader([]byte(`{"connector_id":"`+fleetID+`","action":"lifecycle.request","payload":"{}"}`)))
	req.Header.Set("Authorization", "Bearer "+adminSess.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var act map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&act); err != nil {
		t.Fatal(err)
	}
	if act["status"] != "pending_approval" {
		t.Fatalf("status %v", act["status"])
	}
	jobs, err := st.ListJobs(context.Background(), admin.OrganizationID, 10)
	if err != nil || len(jobs) == 0 || jobs[0].Status != "pending_approval" {
		t.Fatalf("jobs %+v %v", jobs, err)
	}
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/v1/jobs/"+jobs[0].ID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminSess.Token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("requester approve: %d", resp.StatusCode)
	}
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/v1/jobs/"+jobs[0].ID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+opSess.Token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("second approve: %d", resp.StatusCode)
	}
}

func TestLoginLimitIsStored(t *testing.T) {
	st, err := store.Open("file:login-limit?mode=memory&cache=shared")
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
	for i := 0; i < 10; i++ {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/auth/login", bytes.NewReader([]byte(`{"email":"admin@yard.local","password":"nope"}`)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Fatalf("attempt %d: %d", i, resp.StatusCode)
		}
	}
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/auth/login", bytes.NewReader([]byte(`{"email":"admin@yard.local","password":"yard-admin"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 429 {
		t.Fatalf("lockout: %d", resp.StatusCode)
	}
}
