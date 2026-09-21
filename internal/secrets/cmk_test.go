package secrets

import "testing"

func TestRequireCMKRefusesGeneratedKey(t *testing.T) {
	t.Setenv("YARD_REQUIRE_CMK", "1")
	if _, err := LoadOrCreate(t.TempDir(), ""); err == nil {
		t.Fatal("expected refusal")
	}
}
