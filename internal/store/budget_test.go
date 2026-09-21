package store

import (
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/model"
)

func TestIngestBudgetShared(t *testing.T) {
	st, err := Open("file:budget?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	org, err := st.CreateOrganization(t.Context(), "Ops", "ops-budget")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for i := 0; i < 120; i++ {
		ok, err := st.TakeIngestBudget(t.Context(), org.ID, 120, now)
		if err != nil || !ok {
			t.Fatalf("slot %d %v %v", i, ok, err)
		}
	}
	ok, err := st.TakeIngestBudget(t.Context(), org.ID, 120, now)
	if err != nil || ok {
		t.Fatalf("over budget %v %v", ok, err)
	}
}

func TestRegionStamp(t *testing.T) {
	t.Setenv("YARD_REGION", "lab")
	st, err := Open("file:region?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	org, err := st.CreateOrganization(t.Context(), "Ops", "ops-region")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertEvent(t.Context(), &model.Event{OrganizationID: org.ID, Kind: "note", Severity: "info", Title: "hi"}); err != nil {
		t.Fatal(err)
	}
	var region string
	if err := st.queryRow(t.Context(), `SELECT region FROM events WHERE organization_id=?`, org.ID).Scan(&region); err != nil {
		t.Fatal(err)
	}
	if region != "lab" {
		t.Fatalf("region %q", region)
	}
}
