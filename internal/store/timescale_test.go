package store

import (
	"os"
	"strings"
	"testing"
)

func TestTimescaleRequiresPostgres(t *testing.T) {
	t.Setenv("YARD_TIMESCALE", "1")
	_, err := Open("file:timescale-sqlite?mode=memory&cache=shared")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "YARD_TIMESCALE=1 requires a postgres:// database") {
		t.Fatalf("got %v", err)
	}
}

func TestTimescaleOffByDefault(t *testing.T) {
	_ = os.Unsetenv("YARD_TIMESCALE")
	st, err := Open("file:timescale-off?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if st.Timescale {
		t.Fatal("timescale should stay off without YARD_TIMESCALE=1")
	}
}
