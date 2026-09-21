package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/seed"
	"github.com/zyvorai/yard/internal/store"
)

func TestIncidentParentSLAOnCallAndTimeline(t *testing.T) {
	st, err := store.Open("file:incidentops?mode=memory&cache=shared")
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
	me := getJSON[model.User](t, ts.URL, "/api/v1/auth/me", admin)
	now := time.Now().UTC().Truncate(time.Second)
	postJSON[model.OnCall](t, ts.URL, "/api/v1/oncall", admin, map[string]any{
		"user_id": me.ID, "starts_at": now.Add(-time.Hour), "ends_at": now.Add(8 * time.Hour),
	}, 201)
	resp := authed(t, http.MethodPatch, ts.URL+"/api/v1/org", admin, map[string]int{"ack_minutes": 60, "resolve_minutes": 120})
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("sla %d %s", resp.StatusCode, raw)
	}
	parent := postJSON[model.Incident](t, ts.URL, "/api/v1/incidents", admin, map[string]string{"title": "Parent pump"}, 201)
	if parent.Owner != "Operations Admin" {
		t.Fatalf("owner %q", parent.Owner)
	}
	if parent.AckDueAt == nil || parent.AckDueAt.Sub(parent.OpenedAt) != time.Hour {
		t.Fatalf("ack due %v opened %s", parent.AckDueAt, parent.OpenedAt)
	}
	if parent.AckBreached {
		t.Fatal("fresh incident breached ack")
	}
	child := postJSON[model.Incident](t, ts.URL, "/api/v1/incidents", admin, map[string]string{"title": "Child seal", "parent_id": parent.ID}, 201)
	if child.ParentID != parent.ID {
		t.Fatalf("parent %s", child.ParentID)
	}
	late := postJSON[model.Incident](t, ts.URL, "/api/v1/incidents", admin, map[string]any{
		"title": "Late trip", "opened_at": now.Add(-3 * time.Hour),
	}, 201)
	if !late.AckBreached {
		t.Fatal("expected ack breach")
	}
	ackResp := authed(t, http.MethodPatch, ts.URL+"/api/v1/incidents/"+parent.ID, admin, map[string]string{"status": "ack"})
	ackRaw, _ := io.ReadAll(ackResp.Body)
	ackResp.Body.Close()
	if ackResp.StatusCode != 200 {
		t.Fatalf("ack %d %s", ackResp.StatusCode, ackRaw)
	}
	var acked model.Incident
	if err := json.Unmarshal(ackRaw, &acked); err != nil {
		t.Fatal(err)
	}
	if acked.AckedAt == nil || acked.AckBreached {
		t.Fatalf("ack %+v", acked)
	}
	notes := getJSON[[]model.IncidentNote](t, ts.URL, "/api/v1/incidents/"+parent.ID+"/timeline", admin)
	if len(notes) < 2 || notes[0].Kind != "opened" {
		t.Fatalf("timeline %+v", notes)
	}
	sawChild := false
	sawAck := false
	for _, n := range notes {
		if n.Kind == "child" && n.Title == "Child seal" {
			sawChild = true
		}
		if n.Kind == "ack" {
			sawAck = true
		}
	}
	if !sawChild || !sawAck {
		t.Fatalf("timeline %+v", notes)
	}
}
