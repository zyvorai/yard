package store

import (
	"context"
	"errors"
	"testing"
)

func TestSQLiteLeaderAlwaysRuns(t *testing.T) {
	st, err := Open("file:leader?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ran := false
	err = st.WithLeader(context.Background(), func(context.Context) error {
		ran = true
		return nil
	})
	if err != nil || !ran {
		t.Fatalf("ran=%v err=%v", ran, err)
	}
	if !errors.Is(ErrNotLeader, ErrNotLeader) {
		t.Fatal("sentinel")
	}
}

func TestLiveEventRoundTrip(t *testing.T) {
	st, err := Open("file:live?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	id, err := st.InsertLiveEvent(context.Background(), "org", "asset.health", `{"id":"a"}`, "yard_a")
	if err != nil {
		t.Fatal(err)
	}
	ev, err := st.LiveEvent(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Origin != "yard_a" || ev.Kind != "asset.health" {
		t.Fatalf("%+v", ev)
	}
}
