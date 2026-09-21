package store

import (
	"context"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/model"
)

func TestDownsampleKeepsRecentRaw(t *testing.T) {
	st, err := Open("file:downsample?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Date(2026, 9, 21, 15, 30, 0, 0, time.UTC)
	oldA := &model.Observation{OrganizationID: "org", AssetID: "a", Capability: "temp", Value: 10, ValueKind: "number", ObservedAt: now.Add(-48 * time.Hour)}
	oldB := &model.Observation{OrganizationID: "org", AssetID: "a", Capability: "temp", Value: 20, ValueKind: "number", ObservedAt: now.Add(-48*time.Hour + time.Minute)}
	fresh := &model.Observation{OrganizationID: "org", AssetID: "a", Capability: "temp", Value: 7, ValueKind: "number", ObservedAt: now.Add(-2 * time.Hour)}
	text := &model.Observation{OrganizationID: "org", AssetID: "a", Capability: "status", ValueKind: "text", ValueText: "standby", ObservedAt: now.Add(-48 * time.Hour)}
	for _, o := range []*model.Observation{oldA, oldB, fresh, text} {
		if _, err := st.InsertObservation(context.Background(), o); err != nil {
			t.Fatal(err)
		}
	}
	n, err := st.DownsampleBefore(context.Background(), now.Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("removed %d", n)
	}
	list, err := st.ListObservations(context.Background(), "org", "a", "", time.Time{}, time.Time{}, 20)
	if err != nil {
		t.Fatal(err)
	}
	var sawFresh, sawRoll, sawText bool
	for _, o := range list {
		switch {
		case o.Capability == "temp" && o.Source != "rollup" && o.Value == 7:
			sawFresh = true
		case o.Capability == "temp" && o.Source == "rollup" && o.Value == 15:
			sawRoll = true
		case o.Capability == "status" && o.ValueText == "standby":
			sawText = true
		}
	}
	if !sawFresh || !sawRoll || !sawText {
		t.Fatalf("list %+v", list)
	}
}
