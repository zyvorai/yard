package api

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zyvorai/yard/internal/model"
)

// roleOK reports whether u's role satisfies a requested access level. A
// missing/unrecognized role is treated as the most restrictive (read-only)
// rather than defaulting to admin — this used to silently grant an empty
// Role write access, which is the wrong failure direction for an
// authorization check.
func roleOK(u *model.User, write bool) bool {
	if !write {
		return true
	}
	role := strings.ToLower(u.Role)
	return role == "admin" || role == "operator"
}

func (s *Server) requireWrite(w http.ResponseWriter, u *model.User) bool {
	if roleOK(u, true) {
		return true
	}
	if ok, err := s.Store.RoleCanWrite(context.Background(), u.OrganizationID, u.Role); err == nil && ok {
		return true
	}
	writeJSON(w, 403, map[string]string{"error": "forbidden: viewer cannot mutate"})
	return false
}

// requireAdmin gates actions that affect other users' accounts (invite,
// role changes, deactivation) — stricter than requireWrite, which also
// allows the "operator" role.
func (s *Server) requireAdmin(w http.ResponseWriter, u *model.User) bool {
	if strings.ToLower(u.Role) == "admin" {
		return true
	}
	writeJSON(w, 403, map[string]string{"error": "forbidden: admin role required"})
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

func (l *ingestLimiter) over(key string) bool {
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
	l.hits[key] = kept
	return len(kept) >= l.limit
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
	mu           sync.Mutex
	ingestOK     int64
	ingestReject int64
	loginOK      int64
	loginFail    int64
	loginLockout int64
	secretRotate int64
	staleRuns    int64
	version      string
}

func (m *metrics) inc(field *int64) {
	m.mu.Lock()
	*field++
	m.mu.Unlock()
}

func (m *metrics) handler(w http.ResponseWriter, egressDenials int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ver := m.version
	if ver == "" {
		ver = "dev"
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte(
		"# HELP yard_build_info Build information\n" +
			"# TYPE yard_build_info gauge\n" +
			"yard_build_info{version=\"" + ver + "\"} 1\n" +
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
			"yard_login_fail_total " + itoa(m.loginFail) + "\n" +
			"# HELP yard_login_lockout_total Login attempts rejected by the failure limiter\n" +
			"# TYPE yard_login_lockout_total counter\n" +
			"yard_login_lockout_total " + itoa(m.loginLockout) + "\n" +
			"# HELP yard_secret_rotations_total Connector secret rotations\n" +
			"# TYPE yard_secret_rotations_total counter\n" +
			"yard_secret_rotations_total " + itoa(m.secretRotate) + "\n" +
			"# HELP yard_egress_denied_total Outbound requests refused by the egress policy\n" +
			"# TYPE yard_egress_denied_total counter\n" +
			"yard_egress_denied_total " + itoa(egressDenials) + "\n",
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
