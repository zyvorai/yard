package store

import (
	"testing"
	"time"
)

func TestWebAuthnChallengeStrongOneShotAndExpires(t *testing.T) {
	st, err := Open("file:webauthn?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := t.Context()
	ch, err := st.IssueWebAuthnChallenge(ctx, "u1", "login")
	if err != nil {
		t.Fatal(err)
	}
	if len(ch) < 64 {
		t.Fatalf("challenge too short: %q", ch)
	}
	if ok, err := st.ConsumeWebAuthnChallenge(ctx, "u1", ch, "login"); err != nil || !ok {
		t.Fatalf("first consume ok=%v err=%v", ok, err)
	}
	if ok, _ := st.ConsumeWebAuthnChallenge(ctx, "u1", ch, "login"); ok {
		t.Fatal("challenge reused")
	}
	old, err := st.IssueWebAuthnChallenge(ctx, "u1", "login")
	if err != nil {
		t.Fatal(err)
	}
	stale := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
	if _, err := st.exec(ctx, `UPDATE webauthn_challenges SET created_at=? WHERE challenge=?`, stale, old); err != nil {
		t.Fatal(err)
	}
	if ok, _ := st.ConsumeWebAuthnChallenge(ctx, "u1", old, "login"); ok {
		t.Fatal("expired challenge accepted")
	}
}
