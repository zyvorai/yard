package jobs

import (
	"context"
	"testing"
	"time"
)

func TestMQTTTopicBecomesObservation(t *testing.T) {
	st, eng, orgID, assetID := setup(t)
	n, err := ApplyMQTT(context.Background(), eng, orgID, "yard/SIM-TEMP-A/observations", []byte(`{"capability":"temperature","value":11}`))
	if err != nil || n != 1 {
		t.Fatalf("apply %v %d", err, n)
	}
	list, err := st.ListObservations(context.Background(), orgID, assetID, "temperature", time.Time{}, time.Time{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Value != 11 || list[0].Source != "mqtt" {
		t.Fatalf("%+v", list)
	}
}
