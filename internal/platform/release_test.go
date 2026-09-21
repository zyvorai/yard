package platform

import (
	"os"
	"strings"
	"testing"
)

func TestReleaseSignsChecksums(t *testing.T) {
	raw, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "cosign sign-blob") {
		t.Fatal("workflow does not sign the checksum file")
	}
	if strings.Contains(text, "BEGIN PRIVATE KEY") || strings.Contains(text, "echo signature") {
		t.Fatal("workflow contains a stand-in signature")
	}
}
