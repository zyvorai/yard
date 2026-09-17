package connectors_test

import (
	"context"
	"testing"

	"github.com/zyvorai/yard/internal/connectors"
	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

func TestDispatchFleetNeedsEndpoint(t *testing.T) {
	st, err := store.Open("file:dispatch_test.db?mode=memory&cache=shared&_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	d := connectors.NewDispatcher(st)
	status, _, err := d.Execute(context.Background(), "org", &model.Connector{Kind: "fleet", Name: "Fleet"}, "lifecycle.request", "{}")
	if err == nil || status != "failed" {
		t.Fatalf("expected failure without endpoint, status=%s err=%v", status, err)
	}
}

func TestDispatchDeviceAgentNeedsToken(t *testing.T) {
	st, err := store.Open("file:dispatch_test2.db?mode=memory&cache=shared&_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	d := connectors.NewDispatcher(st)
	status, _, err := d.Execute(context.Background(), "org", &model.Connector{
		ID: "c1", Kind: "device-agent", Endpoint: "http://127.0.0.1:1",
	}, "inventory.refresh", "{}")
	if err == nil || status != "failed" {
		t.Fatalf("expected failure without ingest token, status=%s err=%v", status, err)
	}
}

func TestDispatchNodraNeedsAuth(t *testing.T) {
	st, err := store.Open("file:dispatch_test3.db?mode=memory&cache=shared&_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	t.Setenv("YARD_INGEST_TOKEN", "tok")
	d := connectors.NewDispatcher(st)
	status, _, err := d.Execute(context.Background(), "org", &model.Connector{
		Kind: "nodra", Endpoint: "http://127.0.0.1:1", Config: `{"optional":true}`,
	}, "telemetry.receive", "{}")
	if err == nil || status != "failed" {
		t.Fatalf("expected failure without auth_token, status=%s err=%v", status, err)
	}
}
