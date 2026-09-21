package queue

import (
	"testing"
	"time"
)

func TestBackoffDoublesThenCaps(t *testing.T) {
	want := []time.Duration{
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		32 * time.Second,
		60 * time.Second,
		60 * time.Second,
	}
	for i, d := range want {
		got := Backoff(i + 1)
		if got != d {
			t.Fatalf("attempt %d: got %s want %s", i+1, got, d)
		}
	}
	if Backoff(0) != 2*time.Second {
		t.Fatal("attempts below 1 still wait 2s")
	}
}
