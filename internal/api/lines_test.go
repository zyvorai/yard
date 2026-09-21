package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/seed"
	"github.com/zyvorai/yard/internal/store"
)

func TestWorkOrderPartsAndLabor(t *testing.T) {
	st, err := store.Open("file:lines?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	res, err := seed.Bootstrap(context.Background(), st)
	if err != nil || res.User == nil {
		t.Fatal(err)
	}
	wo := &model.WorkOrder{OrganizationID: res.User.OrganizationID, Title: "Replace belt", Kind: "repair"}
	if err := st.CreateWorkOrder(context.Background(), wo); err != nil {
		t.Fatal(err)
	}
	sess, _ := st.CreateSession(context.Background(), res.User.ID, 12*time.Hour)
	srv := New(st, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/work-orders/"+wo.ID+"/lines", bytes.NewReader([]byte(`{"kind":"part","name":"belt","quantity":2,"unit":"ea","unit_cost_cents":1250}`)))
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("create %d", resp.StatusCode)
	}
	lines, err := st.ListWorkOrderLines(context.Background(), res.User.OrganizationID, wo.ID)
	if err != nil || len(lines) != 1 || lines[0].Kind != "part" || lines[0].UnitCostCents != 1250 {
		t.Fatalf("%+v %v", lines, err)
	}
}
