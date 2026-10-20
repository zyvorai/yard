package mail

import (
	"strings"
	"testing"
)

func TestBuildMessageBlocksHeaderInjection(t *testing.T) {
	if _, err := buildMessage("yard@localhost", "a@b.c\r\nBcc: evil@x.y", "s", "b"); err == nil {
		t.Fatal("expected CR/LF in recipient to be rejected")
	}
	msg, err := buildMessage("yard@localhost", "a@b.c", "hi\r\nBcc: evil@x.y", "body")
	if err != nil {
		t.Fatal(err)
	}
	head, _, _ := strings.Cut(string(msg), "\r\n\r\n")
	if strings.Contains(head, "\r\nBcc:") {
		t.Fatalf("injected header: %q", head)
	}
}
