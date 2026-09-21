package jobs

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/egress"
	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

func TestSyncPlaybookRefetch(t *testing.T) {
	st, eng, orgID, _ := setup(t)
	body := "name: First\nsteps:\n  - action: diagnostics.read\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body == "" {
			http.Error(w, "no", 500)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	pb := &model.Playbook{OrganizationID: orgID, Name: "First", Body: body, SourceURL: srv.URL}
	if err := st.CreatePlaybook(t.Context(), pb); err != nil {
		t.Fatal(err)
	}
	eng.Policy = egress.Demo()
	body = "name: Second\nsteps:\n  - action: diagnostics.read\n"
	if err := eng.SyncPlaybooks(t.Context()); err != nil {
		t.Fatal(err)
	}
	got, err := st.PlaybookByID(t.Context(), orgID, pb.ID)
	if err != nil || got.Name != "Second" {
		t.Fatalf("%+v %v", got, err)
	}
	body = ""
	if err := eng.SyncPlaybooks(t.Context()); err != nil {
		t.Fatal(err)
	}
	got, _ = st.PlaybookByID(t.Context(), orgID, pb.ID)
	if got.Name != "Second" {
		t.Fatalf("failed fetch replaced body: %s", got.Body)
	}
}

func TestGeofenceEnterOnce(t *testing.T) {
	st, eng, orgID, assetID := setup(t)
	lat, lng := 19.0, 75.0
	if err := st.CreateGeofence(t.Context(), &store.Geofence{
		OrganizationID: orgID, Name: "Yard", Latitude: lat, Longitude: lng, RadiusM: 200,
	}); err != nil {
		t.Fatal(err)
	}
	ingest := func(key string, la, ln float64) {
		t.Helper()
		_, _, err := eng.IngestObservation(t.Context(), orgID, model.IngestObservation{
			AssetID: assetID, Capability: "heartbeat", Value: 1, ObservedAt: time.Now().UTC(),
			DedupeKey: key, Latitude: &la, Longitude: &ln,
		}, "sim")
		if err != nil {
			t.Fatal(err)
		}
	}
	ingest("in1", lat, lng)
	ingest("in2", lat, lng)
	far := 20.0
	ingest("out", far, lng)
	ev, err := st.ListEvents(t.Context(), orgID, 20)
	if err != nil {
		t.Fatal(err)
	}
	enter, exit := 0, 0
	for _, e := range ev {
		if e.Kind == "geofence.enter" {
			enter++
		}
		if e.Kind == "geofence.exit" {
			exit++
		}
	}
	if enter != 1 || exit != 1 {
		t.Fatalf("enter %d exit %d", enter, exit)
	}
}
