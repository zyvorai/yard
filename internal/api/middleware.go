package api

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zyvorai/yard/internal/model"
)

func roleOK(u *model.User, write bool) bool {
	role := strings.ToLower(u.Role)
	if role == "" {
		role = "admin"
	}
	if !write {
		return true
	}
	return role == "admin" || role == "operator"
}

func (s *Server) requireWrite(w http.ResponseWriter, u *model.User) bool {
	if roleOK(u, true) {
		return true
	}
	writeJSON(w, 403, map[string]string{"error": "forbidden: viewer cannot mutate"})
	return false
}

type ingestLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func newIngestLimiter(limit int, window time.Duration) *ingestLimiter {
	return &ingestLimiter{hits: map[string][]time.Time{}, limit: limit, window: window}
}

func (l *ingestLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cut := now.Add(-l.window)
	arr := l.hits[key]
	kept := arr[:0]
	for _, t := range arr {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.limit {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

type metrics struct {
	mu            sync.Mutex
	ingestOK      int64
	ingestReject  int64
	loginOK       int64
	loginFail     int64
	staleRuns     int64
}

func (m *metrics) inc(field *int64) {
	m.mu.Lock()
	*field++
	m.mu.Unlock()
}

func (m *metrics) handler(w http.ResponseWriter, _ *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte(
		"# HELP yard_ingest_ok_total Accepted ingest requests\n" +
			"# TYPE yard_ingest_ok_total counter\n" +
			"yard_ingest_ok_total " + itoa(m.ingestOK) + "\n" +
			"# HELP yard_ingest_reject_total Rejected ingest requests\n" +
			"# TYPE yard_ingest_reject_total counter\n" +
			"yard_ingest_reject_total " + itoa(m.ingestReject) + "\n" +
			"# HELP yard_login_ok_total Successful logins\n" +
			"# TYPE yard_login_ok_total counter\n" +
			"yard_login_ok_total " + itoa(m.loginOK) + "\n" +
			"# HELP yard_login_fail_total Failed logins\n" +
			"# TYPE yard_login_fail_total counter\n" +
			"yard_login_fail_total " + itoa(m.loginFail) + "\n",
	))
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
