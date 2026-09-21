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

func TestJobRetryAndCancel(t *testing.T) {
	st, err := store.Open("file:jobs-ops?mode=memory&cache=shared")
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
	hash, _ := bcrypt.GenerateFromPassword([]byte("viewer-pass-9"), 10)
	viewer := &model.User{
		OrganizationID: admin.OrganizationID, Email: "viewer-jobs@yard.local",
		DisplayName: "Viewer", Role: "viewer", Active: true, PasswordHash: string(hash),
	}
	if err := st.CreateUser(context.Background(), viewer); err != nil {
		t.Fatal(err)
	}
	adminSess, _ := st.CreateSession(context.Background(), admin.ID, 12*time.Hour)
	viewSess, _ := st.CreateSession(context.Background(), viewer.ID, 12*time.Hour)

	act := &model.ActionRequest{
		OrganizationID: admin.OrganizationID, Action: "inventory.refresh",
		IdempotencyKey: "idem-job-1", Status: "queued", Payload: "{}",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if err := st.CreateAction(context.Background(), act); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"action_id": act.ID, "action": act.Action})
	dead := &store.Job{
		OrganizationID: admin.OrganizationID, Kind: "remote_action", Status: "dead",
		Attempts: 5, MaxAttempts: 5, Payload: string(body), Error: "dial failed",
		RunAfter: time.Now().UTC(),
	}
	queued := &store.Job{
		OrganizationID: admin.OrganizationID, Kind: "remote_action", Status: "queued",
		Payload: string(body), RunAfter: time.Now().UTC(),
	}
	if err := st.CreateJob(context.Background(), dead); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateJob(context.Background(), queued); err != nil {
		t.Fatal(err)
	}

	srv := New(st, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/jobs/"+dead.ID+"/retry", bytes.NewReader(nil))
	req.Header.Set("Authorization", "Bearer "+viewSess.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("viewer retry: %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/v1/jobs/"+dead.ID+"/retry", bytes.NewReader(nil))
	req.Header.Set("Authorization", "Bearer "+adminSess.Token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("retry: %d", resp.StatusCode)
	}
	var view jobView
	if err := json.NewDecoder(resp.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	if view.Status != "queued" || view.Attempts != 0 || view.ActionID != act.ID {
		t.Fatalf("retried job: %+v", view)
	}

	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/v1/jobs/"+queued.ID+"/cancel", bytes.NewReader(nil))
	req.Header.Set("Authorization", "Bearer "+adminSess.Token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("cancel: %d", resp.StatusCode)
	}
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/v1/jobs/"+queued.ID+"/cancel", bytes.NewReader(nil))
	req.Header.Set("Authorization", "Bearer "+adminSess.Token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Fatalf("second cancel: %d", resp.StatusCode)
	}
}
