package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

func setup(t *testing.T) (*store.Store, *Engine, string, string) {
	t.Helper()
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	org, err := st.CreateOrganization(ctx, "Ops", "ops")
	if err != nil {
		t.Fatal(err)
	}
	a := &model.Asset{OrganizationID: org.ID, Name: "Thermal load", ExternalRef: "SIM-TEMP-A", Kind: "equipment"}
	if err := st.UpsertAsset(ctx, a); err != nil {
		t.Fatal(err)
	}
	_ = st.CreateAutomation(ctx, &model.Automation{
		OrganizationID: org.ID, Name: "High temperature incident", Enabled: true,
		TriggerKind: "threshold", Capability: "temperature", Operator: "gt", Threshold: 75, Action: "open_incident",
	})
	return st, &Engine{Store: st}, org.ID, a.ID
}

func TestThresholdOpensIncidentOnce(t *testing.T) {
	st, eng, orgID, _ := setup(t)
	ctx := context.Background()
	_, ok, err := eng.IngestObservation(ctx, orgID, model.IngestObservation{
		AssetExternalRef: "SIM-TEMP-A", Capability: "temperature", Value: 82, Unit: "°C",
		ObservedAt: time.Now().UTC(), DedupeKey: "a",
	}, "sim")
	if err != nil || !ok {
		t.Fatalf("ingest: %v %v", err, ok)
	}
	_, ok, err = eng.IngestObservation(ctx, orgID, model.IngestObservation{
		AssetExternalRef: "SIM-TEMP-A", Capability: "temperature", Value: 83, Unit: "°C",
		ObservedAt: time.Now().UTC(), DedupeKey: "b",
	}, "sim")
	if err != nil || !ok {
		t.Fatalf("second ingest: %v %v", err, ok)
	}
	incs, err := st.ListIncidents(ctx, orgID, "open")
	if err != nil {
		t.Fatal(err)
	}
	if len(incs) != 1 {
		t.Fatalf("expected 1 open incident, got %d", len(incs))
	}
}

func TestDuplicateObservationSkipped(t *testing.T) {
	_, eng, orgID, _ := setup(t)
	ctx := context.Background()
	in := model.IngestObservation{
		AssetExternalRef: "SIM-TEMP-A", Capability: "heartbeat", Value: 1, Unit: "1",
		ObservedAt: time.Now().UTC(), DedupeKey: "same",
	}
	_, ok1, err := eng.IngestObservation(ctx, orgID, in, "sim")
	if err != nil || !ok1 {
		t.Fatalf("first: %v %v", err, ok1)
	}
	_, ok2, err := eng.IngestObservation(ctx, orgID, in, "sim")
	if err != nil {
		t.Fatal(err)
	}
	if ok2 {
		t.Fatal("duplicate should not insert")
	}
}

func TestUnknownAssetRejected(t *testing.T) {
	_, eng, orgID, _ := setup(t)
	_, _, err := eng.IngestObservation(context.Background(), orgID, model.IngestObservation{
		AssetExternalRef: "NOPE", Capability: "temperature", Value: 1,
	}, "sim")
	if err == nil {
		t.Fatal("expected unknown asset error")
	}
}
