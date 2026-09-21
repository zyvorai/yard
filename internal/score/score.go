package score

import "math"

// Asset returns 0–100 and the facts that produced it.
func Asset(health string, openIncidents int, stale bool) (int, []string) {
	n := 100
	evidence := []string{"base 100"}
	switch health {
	case "critical":
		n -= 40
		evidence = append(evidence, "health critical")
	case "degraded":
		n -= 20
		evidence = append(evidence, "health degraded")
	case "stale":
		n -= 30
		evidence = append(evidence, "health stale")
	case "unknown":
		n -= 10
		evidence = append(evidence, "health unknown")
	default:
		evidence = append(evidence, "health "+health)
	}
	if openIncidents > 0 {
		cut := openIncidents * 15
		if cut > 45 {
			cut = 45
		}
		n -= cut
		evidence = append(evidence, "open incidents")
	}
	if stale && health != "stale" {
		n -= 20
		evidence = append(evidence, "stale")
	}
	if n < 0 {
		n = 0
	}
	return n, evidence
}

// Outlier reports whether value is more than 3 standard deviations from sample.
func Outlier(sample []float64, value float64) bool {
	if len(sample) < 20 {
		return false
	}
	var sum float64
	for _, v := range sample {
		sum += v
	}
	mean := sum / float64(len(sample))
	var ss float64
	for _, v := range sample {
		d := v - mean
		ss += d * d
	}
	std := math.Sqrt(ss / float64(len(sample)))
	if std == 0 {
		return false
	}
	return math.Abs(value-mean) > 3*std
}
