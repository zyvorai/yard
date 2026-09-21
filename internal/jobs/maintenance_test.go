package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/seed"
	"github.com/zyvorai/yard/internal/store"
)

func TestMaintenanceOpensOneWorkOrderPerMinute(t *testing.T) {
	st, err := store.Open("file:pm?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	res, err := seed.Bootstrap(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 21, 8, 0, 10, 0, time.UTC)
	sched := &model.WorkOrder{
		OrganizationID: res.User.OrganizationID,
		Title:          "Weekly inspection",
		Kind:           "inspection",
		ScheduleCron:   "0 8 * * 1",
		Status:         "scheduled",
	}
	if err := st.CreateWorkOrder(context.Background(), sched); err != nil {
		t.Fatal(err)
	}
	eng := &Engine{Store: st}
	if err := eng.RunMaintenance(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if err := eng.RunMaintenance(context.Background(), now.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListWorkOrders(context.Background(), res.User.OrganizationID, "")
	if err != nil {
		t.Fatal(err)
	}
	opened := 0
	for _, wo := range list {
		if wo.ScheduleCron == "" && wo.Notes == "Opened by schedule "+sched.ID {
			opened++
		}
	}
	if opened != 1 {
		t.Fatalf("opened %d work orders, want 1 (total %d)", opened, len(list))
	}
}
