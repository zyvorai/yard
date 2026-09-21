package maintain

import (
	"testing"
	"time"
)

func TestDueMatchesMondayMorning(t *testing.T) {
	// 2026-09-21 is a Monday.
	at := time.Date(2026, 9, 21, 8, 0, 30, 0, time.UTC)
	ok, err := Due("0 8 * * 1", at)
	if err != nil || !ok {
		t.Fatalf("due=%v err=%v", ok, err)
	}
	ok, err = Due("0 8 * * 1", at.Add(time.Hour))
	if err != nil || ok {
		t.Fatalf("later due=%v err=%v", ok, err)
	}
	if _, err := Due("*/5 * * * *", at); err == nil {
		t.Fatal("steps are not accepted")
	}
}
