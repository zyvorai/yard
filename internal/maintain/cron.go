package maintain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Due reports whether a five-field cron expression matches t in UTC.
// Fields are minute, hour, day of month, month, and weekday (Sunday is 0).
// Each field is * or a comma-separated list of numbers.
func Due(expr string, t time.Time) (bool, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return false, fmt.Errorf("schedule must be five fields: minute hour day month weekday")
	}
	t = t.UTC()
	vals := []int{t.Minute(), t.Hour(), t.Day(), int(t.Month()), int(t.Weekday())}
	for i, field := range fields {
		ok, err := matchField(field, vals[i])
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

func matchField(field string, val int) (bool, error) {
	if field == "*" {
		return true, nil
	}
	for _, part := range strings.Split(field, ",") {
		n, err := strconv.Atoi(part)
		if err != nil {
			return false, fmt.Errorf("unsupported schedule field %q", field)
		}
		if n == val {
			return true, nil
		}
	}
	return false, nil
}
