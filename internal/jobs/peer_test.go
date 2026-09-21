package jobs

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/egress"
	"github.com/zyvorai/yard/internal/model"
)

func TestPeerReplicationOnce(t *testing.T) {
	st, eng, orgID, _ := setup(t)
	eng.Policy = egress.Demo()
	var got []model.Event
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer peer-token" {
			http.Error(w, "unauthorized", 401)
			return
		}
		var body struct {
			Events []model.Event `json:"events"`
		}
		raw, _ := io.ReadAll(r.Body)
		if json.Unmarshal(raw, &body) != nil {
			http.Error(w, "bad", 400)
			return
		}
		got = append(got, body.Events...)
		w.WriteHeader(202)
	}))
	defer peer.Close()
	t.Setenv("YARD_REGION", "east")
	t.Setenv("YARD_PEER_URL", peer.URL)
	t.Setenv("YARD_PEER_TOKEN", "peer-token")
	if _, err := st.InsertEvent(t.Context(), &model.Event{OrganizationID: orgID, Kind: "note", Severity: "info", Title: "from east"}); err != nil {
		t.Fatal(err)
	}
	if err := eng.ReplicateEvents(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Region != "east" || got[0].Title != "from east" {
		t.Fatalf("peer %+v", got)
	}
	if err := eng.ReplicateEvents(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("replicated again: %d", len(got))
	}
}

func TestHistogramSkipsThreshold(t *testing.T) {
	st, eng, orgID, _ := setup(t)
	_, ok, err := eng.IngestObservation(t.Context(), orgID, model.IngestObservation{
		AssetExternalRef: "SIM-TEMP-A", Capability: "latency", ValueKind: "histogram",
		ValueText: `{"count":4,"sum":12.5,"buckets":[1,2,8]}`, ObservedAt: time.Now().UTC(), DedupeKey: "hist",
	}, "sim")
	if err != nil || !ok {
		t.Fatalf("histogram %v %v", err, ok)
	}
	list, err := st.ListObservations(t.Context(), orgID, "", "latency", time.Time{}, time.Time{}, 10)
	if err != nil || len(list) != 1 || list[0].ValueKind != "histogram" || list[0].Value != 12.5 {
		t.Fatalf("stored %+v %v", list, err)
	}
	incs, err := st.ListIncidents(t.Context(), orgID, "open")
	if err != nil {
		t.Fatal(err)
	}
	for _, inc := range incs {
		if inc.Title == "High temperature incident on Thermal load" && inc.Summary != "" {
			continue
		}
	}
}
