package store

import (
	"context"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/model"
)

func TestAssetAndObservationRoundTrip(t *testing.T) {
	st, err := Open("file:memdb1?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	org, err := st.CreateOrganization(ctx, "Acme", "acme")
	if err != nil {
		t.Fatal(err)
	}
	a := &model.Asset{OrganizationID: org.ID, Name: "Pump", ExternalRef: "P-1", Kind: "machine"}
	if err := st.UpsertAsset(ctx, a); err != nil {
		t.Fatal(err)
	}
	got, err := st.AssetByRef(ctx, org.ID, "P-1")
	if err != nil || got.Name != "Pump" {
		t.Fatalf("asset: %v %#v", err, got)
	}
	ok, err := st.InsertObservation(ctx, &model.Observation{
		OrganizationID: org.ID, AssetID: a.ID, Capability: "temperature", Value: 70, Unit: "°C",
		ObservedAt: time.Now().UTC(), DedupeKey: "p1:t:1",
	})
	if err != nil || !ok {
		t.Fatalf("insert: %v %v", err, ok)
	}
	ok, err = st.InsertObservation(ctx, &model.Observation{
		OrganizationID: org.ID, AssetID: a.ID, Capability: "temperature", Value: 71, Unit: "°C",
		ObservedAt: time.Now().UTC(), DedupeKey: "p1:t:1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected duplicate observation to be rejected")
	}
}

func TestTenantIsolation(t *testing.T) {
	st, err := Open("file:memdb2?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	a, _ := st.CreateOrganization(ctx, "A", "a")
	b, _ := st.CreateOrganization(ctx, "B", "b")
	_ = st.UpsertAsset(ctx, &model.Asset{OrganizationID: a.ID, Name: "A1", ExternalRef: "shared", Kind: "device"})
	_ = st.UpsertAsset(ctx, &model.Asset{OrganizationID: b.ID, Name: "B1", ExternalRef: "shared", Kind: "device"})
	listA, _ := st.ListAssets(ctx, a.ID, "", "", "")
	listB, _ := st.ListAssets(ctx, b.ID, "", "", "")
	if len(listA) != 1 || listA[0].Name != "A1" {
		t.Fatalf("org A: %#v", listA)
	}
	if len(listB) != 1 || listB[0].Name != "B1" {
		t.Fatalf("org B: %#v", listB)
	}
	if _, err := st.AssetByID(ctx, a.ID, listB[0].ID); err == nil {
		t.Fatal("cross-tenant asset read should fail")
	}
}

func TestStaleHealth(t *testing.T) {
	st, err := Open("file:memdb3?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	org, _ := st.CreateOrganization(ctx, "S", "s")
	past := time.Now().UTC().Add(-10 * time.Minute)
	a := &model.Asset{OrganizationID: org.ID, Name: "Quiet", ExternalRef: "Q-1", Kind: "sensor", Health: "healthy", LastSeenAt: &past, StaleAfterSec: 30}
	if err := st.UpsertAsset(ctx, a); err != nil {
		t.Fatal(err)
	}
	n, err := st.RefreshStale(ctx, org.ID)
	if err != nil || n != 1 {
		t.Fatalf("stale n=%d err=%v", n, err)
	}
	got, _ := st.AssetByID(ctx, org.ID, a.ID)
	if got.Health != "stale" {
		t.Fatalf("health=%s", got.Health)
	}
}

func TestSeverityPolicyResolve(t *testing.T) {
	st, err := Open("file:memdb-sev?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	org, _ := st.CreateOrganization(ctx, "Sev", "sev")
	_ = st.CreateSeverityPolicy(ctx, &model.SeverityPolicy{
		OrganizationID: org.ID, Name: "Temp", MatchKind: "capability", MatchValue: "temperature",
		Severity: "critical", Runbook: "cool it", Priority: 100,
	})
	_ = st.CreateSeverityPolicy(ctx, &model.SeverityPolicy{
		OrganizationID: org.ID, Name: "Default", MatchKind: "default",
		Severity: "info", Runbook: "default rb", Priority: 0,
	})
	sev, rb := st.ResolveSeverity(ctx, org.ID, "temperature", "")
	if sev != "critical" || rb != "cool it" {
		t.Fatalf("temp: %s %q", sev, rb)
	}
	sev, rb = st.ResolveSeverity(ctx, org.ID, "humidity", "")
	if sev != "info" || rb != "default rb" {
		t.Fatalf("default: %s %q", sev, rb)
	}
}

func TestImportAssetRow(t *testing.T) {
	st, err := Open("file:memdb-import?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	org, _ := st.CreateOrganization(ctx, "Imp", "imp")
	a, action, err := st.ImportAssetRow(ctx, org.ID, &model.Asset{Name: "Pump", ExternalRef: "P-IMP", Kind: "machine"})
	if err != nil || action != "created" {
		t.Fatalf("create: %v %s", err, action)
	}
	a2, action, err := st.ImportAssetRow(ctx, org.ID, &model.Asset{Name: "Pump 2", ExternalRef: "P-IMP", Kind: "machine"})
	if err != nil || action != "updated" || a2.ID != a.ID {
		t.Fatalf("update: %v %s %#v", err, action, a2)
	}
}
