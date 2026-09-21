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

func TestTextAndBoolSkipNumericAutomations(t *testing.T) {
	st, eng, orgID, assetID := setup(t)
	ctx := context.Background()
	if err := st.CreateAutomation(ctx, &model.Automation{
		OrganizationID: orgID, Name: "Any status", Enabled: true,
		TriggerKind: "threshold", Capability: "status", Operator: "gt", Threshold: -1, Action: "open_incident",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateAutomation(ctx, &model.Automation{
		OrganizationID: orgID, Name: "Door open", Enabled: true,
		TriggerKind: "threshold", Capability: "door", Operator: "gt", Threshold: 0, Action: "open_incident",
	}); err != nil {
		t.Fatal(err)
	}
	_, ok, err := eng.IngestObservation(ctx, orgID, model.IngestObservation{
		AssetExternalRef: "SIM-TEMP-A", Capability: "status", ValueKind: "text", ValueText: "standby",
		ObservedAt: time.Now().UTC(), DedupeKey: "status",
	}, "sim")
	if err != nil || !ok {
		t.Fatalf("text: %v %v", err, ok)
	}
	_, ok, err = eng.IngestObservation(ctx, orgID, model.IngestObservation{
		AssetExternalRef: "SIM-TEMP-A", Capability: "door", ValueKind: "bool", ValueText: "true",
		ObservedAt: time.Now().UTC(), DedupeKey: "door",
	}, "sim")
	if err != nil || !ok {
		t.Fatalf("bool: %v %v", err, ok)
	}
	unc := 0.15
	_, ok, err = eng.IngestObservation(ctx, orgID, model.IngestObservation{
		AssetExternalRef: "SIM-TEMP-A", Capability: "payload", ValueKind: "json", ValueText: `{"rpm":1200}`,
		Quality: "uncertain", QualityReason: "sensor_warmup", Uncertainty: &unc, Calibration: "due", SequenceNum: 42,
		ObservedAt: time.Now().UTC(), DedupeKey: "payload-1",
	}, "sim")
	if err != nil || !ok {
		t.Fatalf("json meta: %v %v", err, ok)
	}
	_, ok, err = eng.IngestObservation(ctx, orgID, model.IngestObservation{
		AssetExternalRef: "SIM-TEMP-A", Capability: "mode", ValueKind: "enum", ValueText: "auto",
		ObservedAt: time.Now().UTC(), DedupeKey: "mode-1",
	}, "sim")
	if err != nil || !ok {
		t.Fatalf("enum: %v %v", err, ok)
	}
	_, ok, err = eng.IngestObservation(ctx, orgID, model.IngestObservation{
		AssetExternalRef: "SIM-TEMP-A", Capability: "trip", ValueKind: "event", ValueText: `{"code":"E42"}`,
		ObservedAt: time.Now().UTC(), DedupeKey: "trip-1",
	}, "sim")
	if err != nil || !ok {
		t.Fatalf("event: %v %v", err, ok)
	}
	_, ok, err = eng.IngestObservation(ctx, orgID, model.IngestObservation{
		AssetExternalRef: "SIM-TEMP-A", Capability: "photo", ValueKind: "ref", ValueText: "att_demo_photo",
		ObservedAt: time.Now().UTC(), DedupeKey: "photo-1",
	}, "sim")
	if err != nil || !ok {
		t.Fatalf("ref: %v %v", err, ok)
	}
	_, ok, err = eng.IngestObservation(ctx, orgID, model.IngestObservation{
		AssetExternalRef: "SIM-TEMP-A", Capability: "cycles", ValueKind: "counter", Value: 9,
		ObservedAt: time.Now().UTC(), DedupeKey: "cycles-1",
	}, "sim")
	if err != nil || !ok {
		t.Fatalf("counter: %v %v", err, ok)
	}
	if _, _, err := eng.IngestObservation(ctx, orgID, model.IngestObservation{
		AssetExternalRef: "SIM-TEMP-A", Capability: "note", ValueKind: "nope", ValueText: "x",
	}, "sim"); err == nil {
		t.Fatal("bad kind accepted")
	}
	list, err := st.ListObservations(ctx, orgID, assetID, "", time.Time{}, time.Time{}, 20)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]model.Observation{}
	for _, o := range list {
		got[o.Capability] = o
	}
	if got["status"].ValueKind != "text" || got["status"].ValueText != "standby" || got["status"].Value != 0 {
		t.Fatalf("text %+v", got["status"])
	}
	if got["door"].ValueKind != "bool" || got["door"].ValueText != "true" || got["door"].Value != 1 {
		t.Fatalf("bool %+v", got["door"])
	}
	if got["payload"].ValueKind != "json" || got["payload"].ValueText != `{"rpm":1200}` || got["payload"].SequenceNum != 42 || got["payload"].QualityReason != "sensor_warmup" || got["payload"].Calibration != "due" || got["payload"].Uncertainty == nil || *got["payload"].Uncertainty != 0.15 {
		t.Fatalf("json meta %+v", got["payload"])
	}
	if got["mode"].ValueKind != "enum" || got["mode"].ValueText != "auto" {
		t.Fatalf("enum %+v", got["mode"])
	}
	if got["trip"].ValueKind != "event" || got["trip"].ValueText != `{"code":"E42"}` {
		t.Fatalf("event %+v", got["trip"])
	}
	if got["photo"].ValueKind != "ref" || got["photo"].ValueText != "att_demo_photo" {
		t.Fatalf("ref %+v", got["photo"])
	}
	if got["cycles"].ValueKind != "counter" || got["cycles"].Value != 9 {
		t.Fatalf("counter %+v", got["cycles"])
	}
	incs, err := st.ListIncidents(ctx, orgID, "open")
	if err != nil {
		t.Fatal(err)
	}
	if len(incs) != 0 {
		t.Fatalf("non-numeric values opened %d incidents", len(incs))
	}
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

func TestAnomalyEventAfterTwentySamples(t *testing.T) {
	st, eng, orgID, assetID := setup(t)
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		v := 10.0
		if i == 19 {
			v = 11
		}
		_, ok, err := eng.IngestObservation(ctx, orgID, model.IngestObservation{
			AssetExternalRef: "SIM-TEMP-A", Capability: "pressure", Value: v,
			ObservedAt: time.Now().UTC().Add(time.Duration(i) * time.Second), DedupeKey: fmt.Sprintf("p%d", i),
		}, "sim")
		if err != nil || !ok {
			t.Fatalf("sample %d: %v", i, err)
		}
	}
	_, ok, err := eng.IngestObservation(ctx, orgID, model.IngestObservation{
		AssetExternalRef: "SIM-TEMP-A", Capability: "pressure", Value: 10,
		ObservedAt: time.Now().UTC().Add(30 * time.Second), DedupeKey: "flat",
	}, "sim")
	if err != nil || !ok {
		t.Fatal(err)
	}
	ev, err := st.ListEventsForAsset(ctx, orgID, assetID, 40)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ev {
		if e.Kind == "telemetry.anomaly" {
			t.Fatal("in-range value flagged")
		}
	}
	_, ok, err = eng.IngestObservation(ctx, orgID, model.IngestObservation{
		AssetExternalRef: "SIM-TEMP-A", Capability: "pressure", Value: 80,
		ObservedAt: time.Now().UTC().Add(40 * time.Second), DedupeKey: "spike",
	}, "sim")
	if err != nil || !ok {
		t.Fatal(err)
	}
	ev, err = st.ListEventsForAsset(ctx, orgID, assetID, 40)
	if err != nil {
		t.Fatal(err)
	}
	var saw bool
	for _, e := range ev {
		if e.Kind == "telemetry.anomaly" {
			saw = true
		}
	}
	if !saw {
		list, _ := st.ListObservations(ctx, orgID, assetID, "pressure", time.Time{}, time.Time{}, 21)
		t.Fatalf("expected anomaly event, observations %d first %v", len(list), list)
	}
}

func TestDebounceAndHysteresis(t *testing.T) {
	st, eng, orgID, _ := setup(t)
	ctx := context.Background()
	if err := st.CreateAutomation(ctx, &model.Automation{
		OrganizationID: orgID, Name: "Slow pressure", Enabled: true,
		TriggerKind: "threshold", Capability: "pressure", Operator: "gt", Threshold: 75,
		Action: "open_incident", Config: `{"debounce_sec":3600,"hysteresis":10}`,
	}); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)
	ingest := func(v float64, at time.Time, key string) {
		t.Helper()
		_, ok, err := eng.IngestObservation(ctx, orgID, model.IngestObservation{
			AssetExternalRef: "SIM-TEMP-A", Capability: "pressure", Value: v, ObservedAt: at, DedupeKey: key,
		}, "sim")
		if err != nil || !ok {
			t.Fatalf("ingest %s: %v %v", key, err, ok)
		}
	}
	ingest(80, t0, "p1")
	ingest(70, t0.Add(10*time.Minute), "p2")
	incs, err := st.ListIncidents(ctx, orgID, "open")
	if err != nil {
		t.Fatal(err)
	}
	if len(incs) != 0 {
		t.Fatalf("opened early: %d", len(incs))
	}
	ingest(82, t0.Add(2*time.Hour), "p3")
	incs, err = st.ListIncidents(ctx, orgID, "open")
	if err != nil {
		t.Fatal(err)
	}
	if len(incs) != 1 {
		t.Fatalf("want 1 incident, got %d", len(incs))
	}
}

func TestFlapCountAfterClear(t *testing.T) {
	st, eng, orgID, _ := setup(t)
	ctx := context.Background()
	if err := st.CreateAutomation(ctx, &model.Automation{
		OrganizationID: orgID, Name: "Pressure flap", Enabled: true,
		TriggerKind: "threshold", Capability: "pressure", Operator: "gt", Threshold: 75,
		Action: "open_incident", Config: `{"hysteresis":10}`,
	}); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)
	ingest := func(v float64, at time.Time, key string) {
		t.Helper()
		_, ok, err := eng.IngestObservation(ctx, orgID, model.IngestObservation{
			AssetExternalRef: "SIM-TEMP-A", Capability: "pressure", Value: v, ObservedAt: at, DedupeKey: key,
		}, "sim")
		if err != nil || !ok {
			t.Fatalf("ingest %s: %v %v", key, err, ok)
		}
	}
	ingest(80, t0, "f1")
	ingest(82, t0.Add(time.Minute), "f2")
	incs, err := st.ListIncidents(ctx, orgID, "open")
	if err != nil {
		t.Fatal(err)
	}
	var inc *model.Incident
	for i := range incs {
		if incs[i].Title == "Pressure flap on Thermal load" {
			inc = &incs[i]
		}
	}
	if inc == nil || inc.FlapCount != 0 {
		t.Fatalf("first trips should not flap: %+v", inc)
	}
	ingest(60, t0.Add(2*time.Minute), "f3")
	ingest(90, t0.Add(3*time.Minute), "f4")
	ingest(91, t0.Add(4*time.Minute), "f5")
	got, err := st.GetIncident(ctx, orgID, inc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FlapCount != 1 {
		t.Fatalf("flap count %d", got.FlapCount)
	}
	notes, err := st.IncidentTimeline(ctx, orgID, inc.ID)
	if err != nil {
		t.Fatal(err)
	}
	flaps := 0
	for _, n := range notes {
		if n.Kind == "flap" {
			flaps++
		}
	}
	if flaps != 1 || notes[0].Kind != "opened" {
		t.Fatalf("timeline %+v", notes)
	}
}
