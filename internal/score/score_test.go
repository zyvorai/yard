package score

import "testing"

func TestOutlierNeedsTwenty(t *testing.T) {
	sample := make([]float64, 20)
	for i := range sample {
		sample[i] = 10
	}
	if Outlier(sample[:19], 100) {
		t.Fatal("short sample flagged")
	}
	if Outlier(sample, 10) {
		t.Fatal("zero deviation flagged")
	}
	sample[0] = 11
	if !Outlier(sample, 100) {
		t.Fatal("expected outlier")
	}
	n, ev := Asset("critical", 1, false)
	if n != 45 || len(ev) < 2 {
		t.Fatalf("score %d %v", n, ev)
	}
}
