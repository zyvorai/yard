package api

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/idgen"
	"github.com/zyvorai/yard/internal/mail"
	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/queue"
	"github.com/zyvorai/yard/internal/secrets"
	"github.com/zyvorai/yard/internal/store"
)

func (s *Server) secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https:; img-src 'self' data: blob: https:; font-src 'self' data:; connect-src 'self' https:; worker-src 'self' blob:; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if strings.HasPrefix(strings.ToLower(s.Runtime.PublicURL), "https://") {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		s.applyCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) applyCORS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Yard-Token")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
	origin := r.Header.Get("Origin")
	if len(s.Runtime.CORSOrigins) == 0 {
		if s.Runtime.Mode != "production" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		return
	}
	for _, allowed := range s.Runtime.CORSOrigins {
		if origin != "" && origin == allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			return
		}
	}
}

func (s *Server) meta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	writeJSON(w, 200, map[string]any{
		"mode":         s.Runtime.Mode,
		"edition":      "community",
		"region":       os.Getenv("YARD_REGION"),
		"smtp":         mail.Configured(),
		"public_url":   s.Runtime.PublicURL,
		"insecure_tls": s.Runtime.Mode == "demo" || s.Runtime.AllowInsecureTLS,
		"timescale":    s.Store != nil && s.Store.Timescale,
	})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r)
	defer cancel()
	if err := s.Store.Ping(ctx); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	got, err := s.Store.SchemaVersion(ctx)
	if err != nil || got < store.LatestSchemaVersion() {
		http.Error(w, "migrations incomplete", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	var denied int64
	if s.Runtime.Egress != nil {
		denied = s.Runtime.Egress.Denials()
	}
	s.metrics.handler(w, denied)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	_ = s.Store.DeleteSessionToken(r.Context(), u.ID, bearer(r))
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) sessions(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListSessions(r.Context(), u.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodDelete:
		if err := s.Store.DeleteUserSessions(r.Context(), u.ID); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) sessionItem(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodDelete {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/auth/sessions/")
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if err := s.Store.DeleteSession(r.Context(), u.ID, id); err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) streamTicket(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	raw := idgen.Secret(24)
	sum := sha256.Sum256([]byte(raw))
	if err := s.Store.CreateStreamTicket(r.Context(), hex.EncodeToString(sum[:]), u.ID, u.OrganizationID, 60*time.Second); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ticket": raw, "expires_in": 60})
}

func (s *Server) putConnectorSecret(w http.ResponseWriter, r *http.Request, u *model.User) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/connectors/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	switch parts[1] {
	case "test":
		s.testConnector(w, r, u, parts[0])
	case "sync":
		s.syncConnector(w, r, u, parts[0])
	case "secret":
		s.rotateConnectorSecret(w, r, u, parts[0])
	default:
		writeJSON(w, 404, map[string]string{"error": "not found"})
	}
}

func (s *Server) rotateConnectorSecret(w http.ResponseWriter, r *http.Request, u *model.User, id string) {
	if r.Method != http.MethodPut {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	if !s.requireWrite(w, u) {
		return
	}
	c, err := s.Store.ConnectorByID(r.Context(), u.OrganizationID, id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	var in struct {
		Secret string `json:"secret"`
	}
	if err := readJSON(r, &in); err != nil || strings.TrimSpace(in.Secret) == "" {
		writeJSON(w, 400, map[string]string{"error": "secret required"})
		return
	}
	blob, err := secrets.Encrypt(s.Runtime.SecretKey, in.Secret)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "encrypt"})
		return
	}
	hint := secrets.Hint(in.Secret)
	if err := s.Store.PutConnectorSecret(r.Context(), c.ID, blob, hint); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "connector.secret_rotated", c.ID, c.Name)
	s.metrics.inc(&s.metrics.secretRotate)
	writeJSON(w, 200, map[string]any{"id": c.ID, "has_secret": true, "secret_hint": hint})
}

func (s *Server) testConnector(w http.ResponseWriter, r *http.Request, u *model.User, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	if !s.requireWrite(w, u) {
		return
	}
	c, err := s.Store.ConnectorByID(r.Context(), u.OrganizationID, id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if strings.TrimSpace(c.Endpoint) == "" {
		writeJSON(w, 400, map[string]string{"error": "endpoint required"})
		return
	}
	if err := s.Runtime.Egress.Validate(c.Endpoint); err != nil {
		_ = s.Store.RecordConnectorProbe(r.Context(), c.ID, err.Error(), 0, false)
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	client := s.Runtime.Egress.HTTPClient(8*time.Second, false)
	start := time.Now()
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, c.Endpoint, nil)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid endpoint"})
		return
	}
	resp, err := client.Do(req)
	ms := int(time.Since(start).Milliseconds())
	if err != nil {
		_ = s.Store.RecordConnectorProbe(r.Context(), c.ID, err.Error(), ms, false)
		writeJSON(w, 200, map[string]any{"ok": false, "error": err.Error(), "latency_ms": ms})
		return
	}
	resp.Body.Close()
	_ = s.Store.RecordConnectorProbe(r.Context(), c.ID, "", ms, false)
	_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "connector.test", c.ID, c.Name)
	writeJSON(w, 200, map[string]any{"ok": true, "status": resp.StatusCode, "latency_ms": ms})
}

func (s *Server) syncConnector(w http.ResponseWriter, r *http.Request, u *model.User, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	if !s.requireWrite(w, u) {
		return
	}
	c, err := s.Store.ConnectorByID(r.Context(), u.OrganizationID, id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if strings.TrimSpace(c.Endpoint) == "" {
		writeJSON(w, 400, map[string]string{"error": "endpoint required"})
		return
	}
	action := "inventory.refresh"
	var list []string
	if json.Unmarshal([]byte(c.Actions), &list) == nil && len(list) > 0 {
		action = list[0]
		for _, a := range list {
			if a == "inventory.refresh" {
				action = a
				break
			}
		}
	}
	act := &model.ActionRequest{
		OrganizationID: u.OrganizationID,
		ConnectorID:    &c.ID,
		Action:         action,
		IdempotencyKey: idgen.New("idem"),
		Status:         "queued",
		Payload:        "{}",
		ExpiresAt:      time.Now().UTC().Add(15 * time.Minute),
	}
	if err := s.Store.CreateAction(r.Context(), act); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	job, err := queue.EnqueueRemoteAction(r.Context(), s.Store, u.OrganizationID, act.ID, c.ID, action, "{}", u.ID, "queued")
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "connector.sync", c.ID, action)
	writeJSON(w, 202, map[string]any{"action_id": act.ID, "job_id": job.ID, "status": job.Status})
}

func (s *Server) connectorSecret(ctx context.Context, conn *model.Connector) (string, error) {
	if conn == nil {
		return "", nil
	}
	blob, _, err := s.Store.GetConnectorSecret(ctx, conn.ID)
	if err == nil && blob != "" {
		return secrets.Decrypt(s.Runtime.SecretKey, blob)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	clean, secret := stripSecretFields(conn.Config)
	if secret == "" {
		return "", nil
	}
	blob, err = secrets.Encrypt(s.Runtime.SecretKey, secret)
	if err != nil {
		return "", err
	}
	if err := s.Store.PutConnectorSecret(ctx, conn.ID, blob, secrets.Hint(secret)); err != nil {
		return "", err
	}
	conn.Config = clean
	_ = s.Store.UpdateConnector(ctx, conn.OrganizationID, conn)
	return secret, nil
}

func stripSecretFields(raw string) (clean, secret string) {
	if strings.TrimSpace(raw) == "" {
		return "{}", ""
	}
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return raw, ""
	}
	for _, key := range []string{"auth_token", "token"} {
		if v, ok := m[key].(string); ok && v != "" && secret == "" {
			secret = v
		}
		delete(m, key)
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}", secret
	}
	return string(b), secret
}

func stripActionSecrets(raw string) string {
	clean, _ := stripSecretFields(raw)
	if clean == "" {
		return "{}"
	}
	return clean
}

func configHasSecretField(raw string) bool {
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return false
	}
	for _, key := range []string{"auth_token", "token"} {
		if v, ok := m[key].(string); ok && v != "" {
			return true
		}
	}
	return false
}

func tlsInsecureRequested(raw string) bool {
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return false
	}
	switch v := m["tls_insecure"].(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	default:
		return false
	}
}

func (s *Server) presentConnector(ctx context.Context, c *model.Connector) {
	clean, secret := stripSecretFields(c.Config)
	if secret != "" {
		if blob, err := secrets.Encrypt(s.Runtime.SecretKey, secret); err == nil {
			_ = s.Store.PutConnectorSecret(ctx, c.ID, blob, secrets.Hint(secret))
			c.Config = clean
			_ = s.Store.UpdateConnector(ctx, c.OrganizationID, c)
		}
	} else {
		c.Config = clean
	}
	if _, hint, err := s.Store.GetConnectorSecret(ctx, c.ID); err == nil {
		c.HasSecret = true
		c.SecretHint = hint
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func contextWithTimeout(r *http.Request) (ctx context.Context, cancel func()) {
	return context.WithTimeout(r.Context(), 2*time.Second)
}
