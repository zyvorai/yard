package yardclient

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/api"
	"github.com/zyvorai/yard/internal/seed"
	"github.com/zyvorai/yard/internal/store"
)

// TestClientAgainstRealServer exercises the SDK against an actual Yard
// server (not a mock), the same httptest.Server pattern internal/api's own
// tests use — this is what proves the hand-authored wrapper matches what
// the server really accepts and returns, not just what openapi.yaml says.
func TestClientAgainstRealServer(t *testing.T) {
	st, err := store.Open("file:sdk-client-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := seed.Bootstrap(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(api.New(st, nil).Handler())
	defer ts.Close()

	ctx := context.Background()
	anon := New(ts.URL)

	authed, sess, err := anon.Login(ctx, seed.DemoEmail, seed.DemoPassword)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if sess.Token == "" || sess.User.Email != seed.DemoEmail {
		t.Fatalf("unexpected session: %+v", sess)
	}

	me, err := authed.Me(ctx)
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	if me.Role != "admin" {
		t.Fatalf("expected admin role, got %q", me.Role)
	}

	assets, err := authed.ListAssets(ctx, "", "", "")
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(assets) == 0 {
		t.Fatal("expected seeded assets")
	}

	detail, err := authed.GetAsset(ctx, assets[0].ID)
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if detail.Asset.ID != assets[0].ID {
		t.Fatalf("asset detail id mismatch: %s != %s", detail.Asset.ID, assets[0].ID)
	}

	if _, err := authed.ListSites(ctx); err != nil {
		t.Fatalf("list sites: %v", err)
	}
	if _, err := authed.ListIncidents(ctx, ""); err != nil {
		t.Fatalf("list incidents: %v", err)
	}
	if _, err := authed.ListWorkOrders(ctx); err != nil {
		t.Fatalf("list work orders: %v", err)
	}

	autos, err := authed.ListAutomations(ctx)
	if err != nil {
		t.Fatalf("list automations: %v", err)
	}
	if len(autos) == 0 {
		t.Fatal("expected seeded automations")
	}

	created, err := authed.CreateAutomation(ctx, Automation{
		Name: "SDK test rule", Enabled: true, TriggerKind: "threshold",
		Capability: "temperature", Operator: "gt", Threshold: 99, Action: "notify",
	})
	if err != nil {
		t.Fatalf("create automation: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected an id on the created automation")
	}

	if _, err := authed.ObservationRange(ctx, assets[0].ID, "", time.Time{}, time.Time{}); err != nil {
		t.Fatalf("observation range: %v", err)
	}

	// A wrong/missing token must surface as an APIError, not a generic error.
	_, err = New(ts.URL).Me(ctx)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 401 {
		t.Fatalf("expected a 401 APIError, got %v", err)
	}
}
