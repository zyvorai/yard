// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package secrets

import "testing"

func TestRequireCMKRefusesGeneratedKey(t *testing.T) {
	t.Setenv("YARD_REQUIRE_CMK", "1")
	if _, err := LoadOrCreate(t.TempDir(), ""); err == nil {
		t.Fatal("expected refusal")
	}
}
