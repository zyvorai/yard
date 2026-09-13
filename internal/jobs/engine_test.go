package jobs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

// redirectTransport rewrites every outbound request's scheme/host to a local
// test server, regardless of the URL the code under test dialed — used to
// exercise firePagerDuty's hardcoded events.pagerduty.com endpoint without a
// real PagerDuty account.
type redirectTransport struct{ target *url.URL }

func (t redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = t.target.Scheme
	req.URL.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

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

func TestCapabilityMaxAlarmOpensIncident(t *testing.T) {
	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	org, err := st.CreateOrganization(ctx, "Ops", "ops")
	if err != nil {
		t.Fatal(err)
	}
	a := &model.Asset{OrganizationID: org.ID, Name: "Cold room", ExternalRef: "CR-1", Kind: "sensor"}
	if err := st.UpsertAsset(ctx, a); err != nil {
		t.Fatal(err)
	}
	max := 8.0
	if err := st.ReplaceCapabilities(ctx, a.ID, []model.Capability{
		{Name: "temperature", Kind: "measurement", Unit: "°C", Max: &max},
	}); err != nil {
		t.Fatal(err)
	}
	// No literal Threshold: capability_max reads the range from the
	// capability itself, so operators don't duplicate the number.
	if err := st.CreateAutomation(ctx, &model.Automation{
		OrganizationID: org.ID, Name: "Cold room breach", Enabled: true,
		TriggerKind: "capability_max", Capability: "temperature", Action: "open_incident",
	}); err != nil {
		t.Fatal(err)
	}
	eng := &Engine{Store: st}

	_, ok, err := eng.IngestObservation(ctx, org.ID, model.IngestObservation{
		AssetExternalRef: "CR-1", Capability: "temperature", Value: 9.5, Unit: "°C",
		ObservedAt: time.Now().UTC(), DedupeKey: "over",
	}, "sim")
	if err != nil || !ok {
		t.Fatalf("ingest over max: %v %v", err, ok)
	}
	incs, err := st.ListIncidents(ctx, org.ID, "open")
	if err != nil {
		t.Fatal(err)
	}
	if len(incs) != 1 {
		t.Fatalf("expected 1 open incident from capability_max breach, got %d", len(incs))
	}

	_, ok, err = eng.IngestObservation(ctx, org.ID, model.IngestObservation{
		AssetExternalRef: "CR-1", Capability: "temperature", Value: 4.0, Unit: "°C",
		ObservedAt: time.Now().UTC(), DedupeKey: "in-range",
	}, "sim")
	if err != nil || !ok {
		t.Fatalf("ingest in range: %v %v", err, ok)
	}
	incs, err = st.ListIncidents(ctx, org.ID, "open")
	if err != nil {
		t.Fatal(err)
	}
	if len(incs) != 1 {
		t.Fatalf("expected still 1 open incident after an in-range value, got %d", len(incs))
	}
}

func TestSlackAndPagerDutyActionsPost(t *testing.T) {
	type call struct{ path, body string }
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, call{r.URL.Path, string(b)})
		w.WriteHeader(200)
	}))
	defer srv.Close()
	srvURL, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	st, err := store.Open("file:" + t.Name() + "?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	org, err := st.CreateOrganization(ctx, "Ops", "ops")
	if err != nil {
		t.Fatal(err)
	}
	a := &model.Asset{OrganizationID: org.ID, Name: "Thermal load", ExternalRef: "SIM-TEMP-B", Kind: "equipment"}
	if err := st.UpsertAsset(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAutomation(ctx, &model.Automation{
		OrganizationID: org.ID, Name: "Slack alert", Enabled: true,
		TriggerKind: "threshold", Capability: "temperature", Operator: "gt", Threshold: 50,
		Action: "slack", Config: fmt.Sprintf(`{"slack_url":"%s/slack"}`, srv.URL),
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAutomation(ctx, &model.Automation{
		OrganizationID: org.ID, Name: "Page on-call", Enabled: true,
		TriggerKind: "threshold", Capability: "temperature", Operator: "gt", Threshold: 50,
		Action: "pagerduty", Config: `{"routing_key":"test-key"}`,
	}); err != nil {
		t.Fatal(err)
	}
	eng := &Engine{Store: st, HTTP: &http.Client{Transport: redirectTransport{target: srvURL}}}

	_, ok, err := eng.IngestObservation(ctx, org.ID, model.IngestObservation{
		AssetExternalRef: "SIM-TEMP-B", Capability: "temperature", Value: 90, Unit: "°C",
		ObservedAt: time.Now().UTC(), DedupeKey: "x",
	}, "sim")
	if err != nil || !ok {
		t.Fatalf("ingest: %v %v", err, ok)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 outbound calls (slack + pagerduty), got %d: %+v", len(calls), calls)
	}
	var sawSlack, sawPagerDuty bool
	for _, c := range calls {
		switch c.path {
		case "/slack":
			sawSlack = true
			if !strings.Contains(c.body, `"text"`) {
				t.Errorf("slack payload missing text field: %s", c.body)
			}
		case "/v2/enqueue":
			sawPagerDuty = true
			if !strings.Contains(c.body, "test-key") || !strings.Contains(c.body, `"trigger"`) {
				t.Errorf("pagerduty payload missing routing_key/trigger: %s", c.body)
			}
		default:
			t.Errorf("unexpected call path %q", c.path)
		}
	}
	if !sawSlack || !sawPagerDuty {
		t.Fatalf("expected both slack and pagerduty calls, got %+v", calls)
	}

	audit, err := st.ListAudit(ctx, org.ID, 20)
	if err != nil {
		t.Fatal(err)
	}
	var sawSlackAudit, sawPagerDutyAudit bool
	for _, e := range audit {
		if e.Action == "slack" {
			sawSlackAudit = true
		}
		if e.Action == "pagerduty" {
			sawPagerDutyAudit = true
		}
	}
	if !sawSlackAudit || !sawPagerDutyAudit {
		t.Fatalf("expected audit entries for slack and pagerduty, got %+v", audit)
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
