package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/seed"
	"github.com/zyvorai/yard/internal/store"
)

func TestRetentionDropsOldObservations(t *testing.T) {
	st, err := store.Open("file:retain?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	res, err := seed.Bootstrap(context.Background(), st)
	if err != nil || res.User == nil {
		t.Fatal(err)
	}
	org := res.User.OrganizationID
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	old := &model.Observation{OrganizationID: org, AssetID: "ast_old", Capability: "temp", Value: 1, ObservedAt: now.Add(-48 * time.Hour)}
	fresh := &model.Observation{OrganizationID: org, AssetID: "ast_old", Capability: "temp", Value: 2, ObservedAt: now.Add(-2 * time.Hour)}
	if _, err := st.InsertObservation(context.Background(), old); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertObservation(context.Background(), fresh); err != nil {
		t.Fatal(err)
	}
	if err := st.SetRetentionDays(context.Background(), org, 1); err != nil {
		t.Fatal(err)
	}
	eng := &Engine{Store: st}
	if err := eng.RunRetention(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListObservations(context.Background(), org, "ast_old", "", time.Time{}, time.Time{}, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Value != 2 {
		t.Fatalf("kept %+v", list)
	}
}
