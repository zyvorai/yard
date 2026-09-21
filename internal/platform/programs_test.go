package platform

import "testing"

func TestProgramsOrdered(t *testing.T) {
	if len(Programs) < 13 {
		t.Fatalf("expected phases 4–16, got %d", len(Programs))
	}
	prev := 3
	for _, p := range Programs {
		if p.Order != prev+1 {
			t.Fatalf("order gap at %d after %d", p.Order, prev)
		}
		prev = p.Order
	}
}
