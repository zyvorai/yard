package api

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/connectors"
	"github.com/zyvorai/yard/internal/idgen"
	"github.com/zyvorai/yard/internal/jobs"
	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/sse"
	"github.com/zyvorai/yard/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	Store    *store.Store
	Engine   *jobs.Engine
	Hub      *sse.Hub
	Static   fs.FS
	Log      *slog.Logger
	Dispatch *connectors.Dispatcher
	tokens   map[string]string
	limiter  *ingestLimiter
	metrics  *metrics
}

func New(st *store.Store, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	hub := sse.New()
	eng := &jobs.Engine{Store: st, Hub: hub, Log: logger}
	return &Server{
		Store:    st,
		Engine:   eng,
		Hub:      hub,
		Log:      logger,
		Dispatch: connectors.NewDispatcher(st),
		tokens:   map[string]string{},
		limiter:  newIngestLimiter(120, time.Minute),
		metrics:  &metrics{},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ready")) })
	mux.HandleFunc("/metrics", s.metrics.handler)
	mux.HandleFunc("/api/v1/auth/login", s.login)
	mux.HandleFunc("/api/v1/auth/me", s.withUser(s.me))
	mux.HandleFunc("/api/v1/overview", s.withUser(s.overview))
	mux.HandleFunc("/api/v1/sites", s.withUser(s.sites))
	mux.HandleFunc("/api/v1/sites/", s.withUser(s.siteItem))
	mux.HandleFunc("/api/v1/assets", s.withUser(s.assets))
	mux.HandleFunc("/api/v1/assets/", s.withUser(s.assetItem))
	mux.HandleFunc("/api/v1/telemetry", s.withUser(s.telemetry))
	mux.HandleFunc("/api/v1/events", s.withUser(s.events))
	mux.HandleFunc("/api/v1/incidents", s.withUser(s.incidents))
	mux.HandleFunc("/api/v1/incidents/", s.withUser(s.incidentItem))
	mux.HandleFunc("/api/v1/work-orders", s.withUser(s.workOrders))
	mux.HandleFunc("/api/v1/work-orders/", s.withUser(s.workOrderItem))
	mux.HandleFunc("/api/v1/connectors", s.withUser(s.connectors))
	mux.HandleFunc("/api/v1/actions", s.withUser(s.actions))
	mux.HandleFunc("/api/v1/automations", s.withUser(s.automations))
	mux.HandleFunc("/api/v1/severity-policies", s.withUser(s.severityPolicies))
	mux.HandleFunc("/api/v1/severity-policies/", s.withUser(s.severityPolicyItem))
	mux.HandleFunc("/api/v1/audit", s.withUser(s.audit))
	mux.HandleFunc("/api/v1/stream", s.withUser(s.stream))
	mux.HandleFunc("/api/v1/onboarding", s.withUser(s.onboarding))
	mux.HandleFunc("/api/v1/geocode", s.withUser(s.geocode))
	mux.HandleFunc("/api/v1/ingest/observations", s.rateIngest(s.withConnector(s.ingestObs)))
	mux.HandleFunc("/api/v1/ingest/inventory", s.rateIngest(s.withConnector(s.ingestInv)))
	mux.HandleFunc("/api/v1/ingest/events", s.rateIngest(s.withConnector(s.ingestEvt)))
	if s.Static != nil {
		mux.Handle("/", s.spa())
	}
	return s.requestLog(cors(mux))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Flush forwards to the underlying ResponseWriter's Flusher so wrapping it
// for request logging doesn't break SSE (internal/api's stream handler
// type-asserts http.Flusher and 500s if it's missing).
func (r *statusRecorder) Flush() {
	if fl, ok := r.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

// requestLog logs one structured line per request (method, path, status,
// duration) via the server's slog.Logger.
func (s *Server) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.Log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func (s *Server) rateIngest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := bearer(r)
		if key == "" {
			key = r.RemoteAddr
		}
		if !s.limiter.allow(key) {
			s.metrics.inc(&s.metrics.ingestReject)
			writeJSON(w, 429, map[string]string{"error": "ingest rate limit"})
			return
		}
		next(w, r)
	}
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Yard-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// parseRangeParam parses an RFC3339 timestamp query param, returning the
// zero time.Time (an unbounded edge) for an empty string.
func parseRangeParam(v string) (time.Time, error) {
	if v == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, v)
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	return r.Header.Get("X-Yard-Token")
}

type ctxKey int

const userKey ctxKey = 1
const connKey ctxKey = 2

func (s *Server) withUser(fn func(http.ResponseWriter, *http.Request, *model.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := bearer(r)
		if tok == "" {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		u, err := s.Store.SessionUser(r.Context(), tok)
		if err != nil {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		fn(w, r.WithContext(context.WithValue(r.Context(), userKey, u)), u)
	}
}

func (s *Server) withConnector(fn func(http.ResponseWriter, *http.Request, *model.Connector)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := bearer(r)
		if tok == "" {
			writeJSON(w, 401, map[string]string{"error": "connector token required"})
			return
		}
		sum := sha256.Sum256([]byte(tok))
		c, err := s.Store.ConnectorByTokenHash(r.Context(), hex.EncodeToString(sum[:]))
		if err != nil {
			writeJSON(w, 401, map[string]string{"error": "invalid connector token"})
			return
		}
		_ = s.Store.TouchConnector(r.Context(), c.ID)
		fn(w, r, c)
	}
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	u, err := s.Store.UserByEmail(r.Context(), in.Email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil {
		s.metrics.inc(&s.metrics.loginFail)
		writeJSON(w, 401, map[string]string{"error": "invalid credentials"})
		return
	}
	sess, err := s.Store.CreateSession(r.Context(), u.ID, 12*time.Hour)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "session"})
		return
	}
	s.metrics.inc(&s.metrics.loginOK)
	writeJSON(w, 200, map[string]any{"token": sess.Token, "user": u, "expires_at": sess.ExpiresAt})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request, u *model.User) {
	writeJSON(w, 200, u)
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request, u *model.User) {
	_ = s.Engine.MarkStale(r.Context(), u.OrganizationID)
	ov, err := s.Store.Overview(r.Context(), u.OrganizationID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, ov)
}

func (s *Server) sites(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListSites(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		if !s.requireWrite(w, u) {
			return
		}
		var site model.Site
		if err := readJSON(r, &site); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		site.OrganizationID = u.OrganizationID
		if site.Kind == "" {
			site.Kind = "site"
		}
		if site.Timezone == "" {
			site.Timezone = "UTC"
		}
		if site.Name == "" {
			writeJSON(w, 400, map[string]string{"error": "name required"})
			return
		}
		if !validLatLng(site.Latitude, site.Longitude) {
			writeJSON(w, 400, map[string]string{"error": "latitude/longitude out of range"})
			return
		}
		if err := s.Store.UpsertSite(r.Context(), &site); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "site.create", site.ID, site.Name)
		s.Hub.Publish(u.OrganizationID, "site.created", site)
		writeJSON(w, 201, site)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) siteItem(w http.ResponseWriter, r *http.Request, u *model.User) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/sites/")
	id = strings.Trim(id, "/")
	if id == "" {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	site, err := s.Store.SiteByID(r.Context(), u.OrganizationID, id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, site)
	case http.MethodPatch:
		if !s.requireWrite(w, u) {
			return
		}
		var in model.Site
		if err := readJSON(r, &in); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		if in.Name != "" {
			site.Name = in.Name
		}
		if in.Kind != "" {
			site.Kind = in.Kind
		}
		if in.Address != "" || in.Name != "" {
			site.Address = in.Address
		}
		if in.Timezone != "" {
			site.Timezone = in.Timezone
		}
		if in.Latitude != nil {
			site.Latitude = in.Latitude
		}
		if in.Longitude != nil {
			site.Longitude = in.Longitude
		}
		if !validLatLng(site.Latitude, site.Longitude) {
			writeJSON(w, 400, map[string]string{"error": "latitude/longitude out of range"})
			return
		}
		if err := s.Store.UpdateSite(r.Context(), site); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "site.update", site.ID, site.Name)
		s.Hub.Publish(u.OrganizationID, "site.updated", site)
		writeJSON(w, 200, site)
	case http.MethodDelete:
		if !s.requireWrite(w, u) {
			return
		}
		if err := s.Store.DeleteSite(r.Context(), u.OrganizationID, id); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "site.delete", id, site.Name)
		writeJSON(w, 200, map[string]string{"deleted": id})
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) assets(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListAssets(r.Context(), u.OrganizationID, r.URL.Query().Get("q"), r.URL.Query().Get("kind"), r.URL.Query().Get("health"))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		if !s.requireWrite(w, u) {
			return
		}
		var a model.Asset
		if err := readJSON(r, &a); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		a.OrganizationID = u.OrganizationID
		if a.Kind == "" {
			a.Kind = "equipment"
		}
		if !validLatLng(a.Latitude, a.Longitude) {
			writeJSON(w, 400, map[string]string{"error": "latitude/longitude out of range"})
			return
		}
		if err := s.Store.UpsertAsset(r.Context(), &a); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "asset.create", a.ID, a.Name)
		s.Hub.Publish(u.OrganizationID, "asset.created", a)
		writeJSON(w, 201, a)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) assetItem(w http.ResponseWriter, r *http.Request, u *model.User) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
	parts := strings.Split(rest, "/")
	id := parts[0]
	if id == "" {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if id == "export" {
		s.assetsExport(w, r, u)
		return
	}
	if id == "import" {
		s.assetsImport(w, r, u)
		return
	}
	a, err := s.Store.AssetByID(r.Context(), u.OrganizationID, id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if len(parts) == 1 && r.Method == http.MethodGet {
		caps, _ := s.Store.ListCapabilities(r.Context(), a.ID)
		obs, _ := s.Store.ListObservations(r.Context(), u.OrganizationID, a.ID, "", time.Time{}, time.Time{}, 80)
		ev, _ := s.Store.ListEventsForAsset(r.Context(), u.OrganizationID, a.ID, 40)
		wos, _ := s.Store.ListWorkOrdersForAsset(r.Context(), u.OrganizationID, a.ID)
		writeJSON(w, 200, map[string]any{"asset": a, "capabilities": caps, "observations": obs, "events": ev, "work_orders": wos})
		return
	}
	if len(parts) == 1 && r.Method == http.MethodPatch {
		if !s.requireWrite(w, u) {
			return
		}
		var in model.Asset
		if err := readJSON(r, &in); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		if in.Name != "" {
			a.Name = in.Name
		}
		if in.Kind != "" {
			a.Kind = in.Kind
		}
		if in.Status != "" {
			a.Status = in.Status
		}
		if in.Manufacturer != "" {
			a.Manufacturer = in.Manufacturer
		}
		if in.Model != "" {
			a.Model = in.Model
		}
		if in.Serial != "" {
			a.Serial = in.Serial
		}
		if in.ExternalRef != "" {
			a.ExternalRef = in.ExternalRef
		}
		if in.SiteID != nil {
			a.SiteID = in.SiteID
		}
		if in.Latitude != nil {
			a.Latitude = in.Latitude
		}
		if in.Longitude != nil {
			a.Longitude = in.Longitude
		}
		if in.StaleAfterSec > 0 {
			a.StaleAfterSec = in.StaleAfterSec
		}
		if in.Metadata != "" {
			a.Metadata = in.Metadata
		}
		if !validLatLng(a.Latitude, a.Longitude) {
			writeJSON(w, 400, map[string]string{"error": "latitude/longitude out of range"})
			return
		}
		if err := s.Store.UpsertAsset(r.Context(), a); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "asset.update", a.ID, a.Name)
		s.Hub.Publish(u.OrganizationID, "asset.updated", a)
		writeJSON(w, 200, a)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		if !s.requireWrite(w, u) {
			return
		}
		if err := s.Store.DeleteAsset(r.Context(), u.OrganizationID, id); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "asset.delete", id, a.Name)
		writeJSON(w, 200, map[string]string{"deleted": id})
		return
	}
	if len(parts) >= 2 && parts[1] == "capabilities" {
		if r.Method == http.MethodGet {
			caps, err := s.Store.ListCapabilities(r.Context(), a.ID)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, 200, caps)
			return
		}
		if r.Method == http.MethodPut {
			if !s.requireWrite(w, u) {
				return
			}
			var caps []model.Capability
			if err := readJSON(r, &caps); err != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid json"})
				return
			}
			for i := range caps {
				caps[i].AssetID = a.ID
				if caps[i].Kind == "" {
					caps[i].Kind = "measurement"
				}
			}
			if err := s.Store.ReplaceCapabilities(r.Context(), a.ID, caps); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "asset.capabilities", a.ID, a.Name)
			out, _ := s.Store.ListCapabilities(r.Context(), a.ID)
			writeJSON(w, 200, out)
			return
		}
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	if len(parts) >= 2 && parts[1] == "observations" {
		cap := r.URL.Query().Get("capability")
		limit := 400
		from, err := parseRangeParam(r.URL.Query().Get("from"))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid from: " + err.Error()})
			return
		}
		to, err := parseRangeParam(r.URL.Query().Get("to"))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid to: " + err.Error()})
			return
		}
		obs, err := s.Store.ListObservations(r.Context(), u.OrganizationID, a.ID, cap, from, to, limit)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, obs)
		return
	}
	writeJSON(w, 404, map[string]string{"error": "not found"})
}

func (s *Server) assetsExport(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	list, err := s.Store.ListAssets(r.Context(), u.OrganizationID, r.URL.Query().Get("q"), r.URL.Query().Get("kind"), r.URL.Query().Get("health"))
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "" {
		format = "json"
	}
	switch format {
	case "json":
		w.Header().Set("Content-Disposition", `attachment; filename="assets.json"`)
		writeJSON(w, 200, list)
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="assets.csv"`)
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"name", "external_ref", "kind", "status", "health", "manufacturer", "model", "serial", "site_id", "latitude", "longitude", "stale_after_sec", "metadata"})
		for _, a := range list {
			_ = cw.Write([]string{
				a.Name, a.ExternalRef, a.Kind, a.Status, a.Health, a.Manufacturer, a.Model, a.Serial,
				derefS(a.SiteID), fmtFloat(a.Latitude), fmtFloat(a.Longitude), strconv.Itoa(a.StaleAfterSec), a.Metadata,
			})
		}
		cw.Flush()
	default:
		writeJSON(w, 400, map[string]string{"error": "format must be json or csv"})
	}
}

func (s *Server) assetsImport(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	if !s.requireWrite(w, u) {
		return
	}
	ct := r.Header.Get("Content-Type")
	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "" {
		if strings.Contains(ct, "text/csv") || strings.Contains(ct, "application/csv") {
			format = "csv"
		} else {
			format = "json"
		}
	}
	var rows []model.Asset
	switch format {
	case "json":
		body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "read body"})
			return
		}
		if err := json.Unmarshal(body, &rows); err != nil {
			var one model.Asset
			if err2 := json.Unmarshal(body, &one); err2 != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid json array or object"})
				return
			}
			rows = []model.Asset{one}
		}
	case "csv":
		body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "read body"})
			return
		}
		cr := csv.NewReader(strings.NewReader(string(body)))
		records, err := cr.ReadAll()
		if err != nil || len(records) < 2 {
			writeJSON(w, 400, map[string]string{"error": "invalid csv"})
			return
		}
		header := map[string]int{}
		for i, h := range records[0] {
			header[strings.ToLower(strings.TrimSpace(h))] = i
		}
		col := func(row []string, name string) string {
			i, ok := header[name]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		for _, row := range records[1:] {
			a := model.Asset{
				Name:          col(row, "name"),
				ExternalRef:   col(row, "external_ref"),
				Kind:          col(row, "kind"),
				Status:        col(row, "status"),
				Manufacturer:  col(row, "manufacturer"),
				Model:         col(row, "model"),
				Serial:        col(row, "serial"),
				Metadata:      col(row, "metadata"),
				StaleAfterSec: atoiDefault(col(row, "stale_after_sec"), 90),
			}
			if site := col(row, "site_id"); site != "" {
				a.SiteID = &site
			}
			if lat := parseFloatPtr(col(row, "latitude")); lat != nil && validLatLng(lat, nil) {
				a.Latitude = lat
			}
			if lng := parseFloatPtr(col(row, "longitude")); lng != nil && validLatLng(nil, lng) {
				a.Longitude = lng
			}
			if a.Name == "" && a.ExternalRef == "" {
				continue
			}
			if a.Name == "" {
				a.Name = a.ExternalRef
			}
			rows = append(rows, a)
		}
	default:
		writeJSON(w, 400, map[string]string{"error": "format must be json or csv"})
		return
	}
	created, updated := 0, 0
	var out []model.Asset
	for i := range rows {
		a, action, err := s.Store.ImportAssetRow(r.Context(), u.OrganizationID, &rows[i])
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		if action == "created" {
			created++
		} else {
			updated++
		}
		out = append(out, *a)
	}
	_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "asset.import", fmt.Sprintf("%d", len(out)), fmt.Sprintf("created=%d updated=%d format=%s", created, updated, format))
	writeJSON(w, 200, map[string]any{"created": created, "updated": updated, "assets": out})
}

func derefS(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func fmtFloat(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}

func parseFloatPtr(s string) *float64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func validLatLng(lat, lng *float64) bool {
	if lat != nil && (*lat < -90 || *lat > 90) {
		return false
	}
	if lng != nil && (*lng < -180 || *lng > 180) {
		return false
	}
	return true
}

func atoiDefault(s string, d int) int {
	if s == "" {
		return d
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return d
	}
	return n
}

func (s *Server) severityPolicies(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListSeverityPolicies(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		if !s.requireWrite(w, u) {
			return
		}
		var p model.SeverityPolicy
		if err := readJSON(r, &p); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		p.OrganizationID = u.OrganizationID
		if p.Name == "" {
			writeJSON(w, 400, map[string]string{"error": "name required"})
			return
		}
		if err := s.Store.CreateSeverityPolicy(r.Context(), &p); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "severity_policy.create", p.ID, p.Name)
		writeJSON(w, 201, p)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) severityPolicyItem(w http.ResponseWriter, r *http.Request, u *model.User) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/severity-policies/")
	if id == "" {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	p, err := s.Store.GetSeverityPolicy(r.Context(), u.OrganizationID, id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, p)
	case http.MethodPatch:
		if !s.requireWrite(w, u) {
			return
		}
		var in model.SeverityPolicy
		if err := readJSON(r, &in); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		if in.Name != "" {
			p.Name = in.Name
		}
		if in.MatchKind != "" {
			p.MatchKind = in.MatchKind
		}
		p.MatchValue = in.MatchValue
		if in.Severity != "" {
			p.Severity = in.Severity
		}
		p.Runbook = in.Runbook
		p.Priority = in.Priority
		if err := s.Store.UpdateSeverityPolicy(r.Context(), p); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "severity_policy.update", p.ID, p.Name)
		writeJSON(w, 200, p)
	case http.MethodDelete:
		if !s.requireWrite(w, u) {
			return
		}
		if err := s.Store.DeleteSeverityPolicy(r.Context(), u.OrganizationID, id); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "severity_policy.delete", id, p.Name)
		writeJSON(w, 200, map[string]string{"deleted": id})
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) telemetry(w http.ResponseWriter, r *http.Request, u *model.User) {
	pts, err := s.Store.LatestTelemetry(r.Context(), u.OrganizationID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, pts)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request, u *model.User) {
	list, err := s.Store.ListEvents(r.Context(), u.OrganizationID, 120)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) incidents(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListIncidents(r.Context(), u.OrganizationID, r.URL.Query().Get("status"))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		var inc model.Incident
		if err := readJSON(r, &inc); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		inc.OrganizationID = u.OrganizationID
		if inc.Severity == "" || inc.Runbook == "" {
			sev, rb := s.Store.ResolveSeverity(r.Context(), u.OrganizationID, "", "")
			if inc.Severity == "" {
				inc.Severity = sev
			}
			if inc.Runbook == "" {
				inc.Runbook = rb
			}
		}
		if err := s.Store.CreateIncident(r.Context(), &inc); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_, _ = s.Store.InsertEvent(r.Context(), &model.Event{OrganizationID: u.OrganizationID, AssetID: inc.AssetID, Kind: "incident.opened", Severity: inc.Severity, Title: inc.Title, Body: inc.Summary})
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "incident.open", inc.ID, inc.Title)
		s.Hub.Publish(u.OrganizationID, "incident.opened", inc)
		writeJSON(w, 201, inc)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) incidentItem(w http.ResponseWriter, r *http.Request, u *model.User) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/")
	id = strings.TrimSuffix(id, "/resolve")
	inc, err := s.Store.GetIncident(r.Context(), u.OrganizationID, strings.Split(id, "/")[0])
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if r.Method == http.MethodGet && !strings.Contains(r.URL.Path, "/resolve") {
		writeJSON(w, 200, inc)
		return
	}
	if r.Method == http.MethodPatch || strings.HasSuffix(r.URL.Path, "/resolve") {
		var in map[string]string
		_ = readJSON(r, &in)
		if v := in["status"]; v != "" {
			inc.Status = v
		}
		if v := in["owner"]; v != "" {
			inc.Owner = v
		}
		if v := in["resolution"]; v != "" {
			inc.Resolution = v
		}
		if v := in["summary"]; v != "" {
			inc.Summary = v
		}
		if inc.Status == "resolved" && inc.ResolvedAt == nil {
			now := time.Now().UTC()
			inc.ResolvedAt = &now
		}
		if err := s.Store.UpdateIncident(r.Context(), inc); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "incident.update", inc.ID, inc.Status+" "+inc.Resolution)
		s.Hub.Publish(u.OrganizationID, "incident.updated", inc)
		writeJSON(w, 200, inc)
		return
	}
	writeJSON(w, 405, map[string]string{"error": "method"})
}

func (s *Server) workOrders(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListWorkOrders(r.Context(), u.OrganizationID, r.URL.Query().Get("status"))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		if !s.requireWrite(w, u) {
			return
		}
		var wo model.WorkOrder
		if err := readJSON(r, &wo); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		wo.OrganizationID = u.OrganizationID
		if wo.Kind == "" {
			wo.Kind = "maintenance"
		}
		if wo.Priority == "" {
			wo.Priority = "normal"
		}
		if wo.Title == "" {
			writeJSON(w, 400, map[string]string{"error": "title required"})
			return
		}
		if err := s.Store.CreateWorkOrder(r.Context(), &wo); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "workorder.create", wo.ID, wo.Title)
		s.Hub.Publish(u.OrganizationID, "workorder.created", wo)
		writeJSON(w, 201, wo)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) workOrderItem(w http.ResponseWriter, r *http.Request, u *model.User) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/work-orders/")
	wo, err := s.Store.GetWorkOrder(r.Context(), u.OrganizationID, id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, 200, wo)
		return
	}
	if r.Method == http.MethodPatch {
		if !s.requireWrite(w, u) {
			return
		}
		var in map[string]string
		_ = readJSON(r, &in)
		if v := in["status"]; v != "" {
			wo.Status = v
		}
		if v := in["assignee"]; v != "" {
			wo.Assignee = v
		}
		if v := in["notes"]; v != "" {
			wo.Notes = v
		}
		if v := in["priority"]; v != "" {
			wo.Priority = v
		}
		if v, ok := in["due_at"]; ok {
			if v == "" {
				wo.DueAt = nil
			} else if t, err := time.Parse(time.RFC3339, v); err == nil {
				wo.DueAt = &t
			} else if t, err := time.Parse("2006-01-02", v); err == nil {
				wo.DueAt = &t
			}
		}
		if err := s.Store.UpdateWorkOrder(r.Context(), wo); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "workorder.update", wo.ID, wo.Status)
		s.Hub.Publish(u.OrganizationID, "workorder.updated", wo)
		writeJSON(w, 200, wo)
		return
	}
	writeJSON(w, 405, map[string]string{"error": "method"})
}

func (s *Server) connectors(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListConnectors(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPatch:
		var in struct {
			ID          string  `json:"id"`
			Name        *string `json:"name"`
			Endpoint    *string `json:"endpoint"`
			Status      *string `json:"status"`
			Config      *string `json:"config"`
			RotateToken bool    `json:"rotate_token"`
		}
		if err := readJSON(r, &in); err != nil || in.ID == "" {
			writeJSON(w, 400, map[string]string{"error": "id required"})
			return
		}
		c, err := s.Store.ConnectorByID(r.Context(), u.OrganizationID, in.ID)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not found"})
			return
		}
		if in.Name != nil {
			c.Name = *in.Name
		}
		if in.Endpoint != nil {
			c.Endpoint = *in.Endpoint
		}
		if in.Status != nil {
			c.Status = *in.Status
		}
		if in.Config != nil {
			c.Config = *in.Config
		}
		if err := s.Store.UpdateConnector(r.Context(), u.OrganizationID, c); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		out := map[string]any{"connector": c}
		if in.RotateToken {
			tok := "yard_" + c.Kind + "_" + idgen.Secret(16)
			sum := sha256.Sum256([]byte(tok))
			hint := tok
			if len(hint) > 12 {
				hint = hint[:12] + "…"
			}
			if err := s.Store.SetConnectorToken(r.Context(), u.OrganizationID, c.ID, hex.EncodeToString(sum[:]), hint); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			c.TokenHint = hint
			c.Status = "connected"
			out["connector"] = c
			out["token"] = tok
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "connector.update", c.ID, c.Name)
		writeJSON(w, 200, out)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) actions(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListActions(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		var in struct {
			AssetID        string `json:"asset_id"`
			ConnectorID    string `json:"connector_id"`
			Action         string `json:"action"`
			Payload        string `json:"payload"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := readJSON(r, &in); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		if in.Action == "" {
			writeJSON(w, 400, map[string]string{"error": "action required"})
			return
		}
		if in.IdempotencyKey == "" {
			in.IdempotencyKey = r.Header.Get("Idempotency-Key")
		}
		if in.IdempotencyKey == "" {
			in.IdempotencyKey = idgen.New("idem")
		}
		if in.Payload == "" {
			in.Payload = "{}"
		}
		var assetID *string
		if in.AssetID != "" {
			assetID = &in.AssetID
		}
		var connID *string
		var conn *model.Connector
		if in.ConnectorID != "" {
			connID = &in.ConnectorID
			var err error
			conn, err = s.Store.ConnectorByID(r.Context(), u.OrganizationID, in.ConnectorID)
			if err != nil {
				writeJSON(w, 404, map[string]string{"error": "connector not found"})
				return
			}
		}
		act := &model.ActionRequest{
			OrganizationID: u.OrganizationID,
			AssetID:        assetID,
			ConnectorID:    connID,
			Action:         in.Action,
			IdempotencyKey: in.IdempotencyKey,
			Status:         "accepted",
			Payload:        in.Payload,
			ExpiresAt:      time.Now().UTC().Add(15 * time.Minute),
		}
		if err := s.Store.CreateAction(r.Context(), act); err != nil {
			writeJSON(w, 409, map[string]string{"error": err.Error()})
			return
		}
		status, result := "recorded", "No connector selected; outcome recorded locally."
		if conn != nil && s.Dispatch != nil {
			st, res, err := s.Dispatch.Execute(r.Context(), u.OrganizationID, conn, in.Action, in.Payload)
			status = st
			result = res
			if err != nil && result == "" {
				result = err.Error()
			}
		}
		_ = s.Store.CompleteAction(r.Context(), act.ID, status, result)
		act.Status = status
		act.Result = result
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "action.request", act.ID, act.Action)
		code := 202
		if status == "failed" {
			code = 502
		}
		writeJSON(w, code, act)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) automations(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListAutomations(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		if !s.requireWrite(w, u) {
			return
		}
		var a model.Automation
		if err := readJSON(r, &a); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		a.OrganizationID = u.OrganizationID
		if a.Name == "" || a.Action == "" {
			writeJSON(w, 400, map[string]string{"error": "name and action required"})
			return
		}
		if a.TriggerKind == "" {
			a.TriggerKind = "threshold"
		}
		if a.Config == "" {
			a.Config = "{}"
		}
		a.Enabled = true
		if err := s.Store.CreateAutomation(r.Context(), &a); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "automation.create", a.ID, a.Name)
		writeJSON(w, 201, a)
	case http.MethodPatch:
		if !s.requireWrite(w, u) {
			return
		}
		var in struct {
			ID          string   `json:"id"`
			Enabled     *bool    `json:"enabled"`
			Name        *string  `json:"name"`
			TriggerKind *string  `json:"trigger_kind"`
			Capability  *string  `json:"capability"`
			Operator    *string  `json:"operator"`
			Threshold   *float64 `json:"threshold"`
			Action      *string  `json:"action"`
			Config      *string  `json:"config"`
			Delete      bool     `json:"delete"`
		}
		if err := readJSON(r, &in); err != nil || in.ID == "" {
			writeJSON(w, 400, map[string]string{"error": "id required"})
			return
		}
		if in.Delete {
			if err := s.Store.DeleteAutomation(r.Context(), u.OrganizationID, in.ID); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, 200, map[string]string{"deleted": in.ID})
			return
		}
		list, err := s.Store.ListAutomations(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		var cur *model.Automation
		for i := range list {
			if list[i].ID == in.ID {
				cur = &list[i]
				break
			}
		}
		if cur == nil {
			writeJSON(w, 404, map[string]string{"error": "not found"})
			return
		}
		if in.Enabled != nil && in.Name == nil && in.Action == nil && in.TriggerKind == nil && in.Capability == nil && in.Operator == nil && in.Threshold == nil && in.Config == nil {
			if err := s.Store.SetAutomationEnabled(r.Context(), u.OrganizationID, in.ID, *in.Enabled); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, 200, map[string]any{"id": in.ID, "enabled": *in.Enabled})
			return
		}
		if in.Enabled != nil {
			cur.Enabled = *in.Enabled
		}
		if in.Name != nil {
			cur.Name = *in.Name
		}
		if in.TriggerKind != nil {
			cur.TriggerKind = *in.TriggerKind
		}
		if in.Capability != nil {
			cur.Capability = *in.Capability
		}
		if in.Operator != nil {
			cur.Operator = *in.Operator
		}
		if in.Threshold != nil {
			cur.Threshold = *in.Threshold
		}
		if in.Action != nil {
			cur.Action = *in.Action
		}
		if in.Config != nil {
			cur.Config = *in.Config
		}
		if err := s.Store.UpdateAutomation(r.Context(), cur); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, cur)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) audit(w http.ResponseWriter, r *http.Request, u *model.User) {
	list, err := s.Store.ListAudit(r.Context(), u.OrganizationID, 200)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) stream(w http.ResponseWriter, r *http.Request, u *model.User) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, 500, map[string]string{"error": "stream unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := s.Hub.Subscribe(u.OrganizationID)
	defer s.Hub.Unsubscribe(ch)
	_, _ = w.Write([]byte("event: ready\ndata: {}\n\n"))
	fl.Flush()
	tick := time.NewTicker(25 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			_, _ = w.Write([]byte(": ping\n\n"))
			fl.Flush()
		case msg, ok := <-ch:
			if !ok {
				return
			}
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(msg)
			_, _ = w.Write([]byte("\n\n"))
			fl.Flush()
		}
	}
}

func (s *Server) onboarding(w http.ResponseWriter, r *http.Request, u *model.User) {
	assets, _ := s.Store.ListAssets(r.Context(), u.OrganizationID, "", "", "")
	incidents, _ := s.Store.ListIncidents(r.Context(), u.OrganizationID, "")
	connectors, _ := s.Store.ListConnectors(r.Context(), u.OrganizationID)
	connected := false
	for _, c := range connectors {
		if c.Status == "connected" || c.Kind == "simulator" || c.Kind == "http-ingest" {
			connected = true
			break
		}
	}
	steps := []map[string]any{
		{"id": "workspace", "title": "Workspace ready", "body": "Your organization is seeded and ready.", "done": true},
		{"id": "source", "title": "Connect a source", "body": "Use the included simulator or a Device Agent gateway.", "done": connected, "href": "/integrations"},
		{"id": "discover", "title": "Discover assets", "body": "Inventory arrives as assets — devices are one kind.", "done": len(assets) > 0, "href": "/assets"},
		{"id": "health", "title": "Inspect health", "body": "Open Overview or Map to see freshness and health.", "done": len(assets) > 0, "href": "/"},
		{"id": "alert", "title": "Create an alert", "body": "Temperature or missed heartbeat opens an incident you can assign.", "done": len(incidents) > 0, "href": "/incidents"},
	}
	doneN := 0
	for _, st := range steps {
		if st["done"] == true {
			doneN++
		}
	}
	writeJSON(w, 200, map[string]any{"steps": steps, "completed": doneN, "total": len(steps)})
}

type nominatimResult struct {
	DisplayName string `json:"display_name"`
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
}

type geocodeResult struct {
	DisplayName string  `json:"display_name"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
}

func (s *Server) geocode(w http.ResponseWriter, r *http.Request, u *model.User) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, 400, map[string]string{"error": "q required"})
		return
	}
	if !s.limiter.allow("geocode") {
		writeJSON(w, 429, map[string]string{"error": "geocode rate limit"})
		return
	}
	upstream := "https://nominatim.openstreetmap.org/search?format=json&limit=5&q=" + url.QueryEscape(q)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstream, nil)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": "geocode upstream failed"})
		return
	}
	req.Header.Set("User-Agent", "Yard/1.0 (https://github.com/zyvorai/yard)")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": "geocode upstream failed"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeJSON(w, 502, map[string]string{"error": "geocode upstream failed"})
		return
	}
	var raw []nominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		writeJSON(w, 502, map[string]string{"error": "geocode upstream failed"})
		return
	}
	results := make([]geocodeResult, 0, len(raw))
	for _, item := range raw {
		lat, errLat := strconv.ParseFloat(item.Lat, 64)
		lon, errLon := strconv.ParseFloat(item.Lon, 64)
		if errLat != nil || errLon != nil {
			continue
		}
		results = append(results, geocodeResult{DisplayName: item.DisplayName, Lat: lat, Lon: lon})
	}
	writeJSON(w, 200, map[string]any{"results": results})
}

func (s *Server) ingestObs(w http.ResponseWriter, r *http.Request, c *model.Connector) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	var batch []model.IngestObservation
	dec := json.NewDecoder(r.Body)
	var first json.RawMessage
	if err := dec.Decode(&first); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	if len(first) > 0 && first[0] == '[' {
		if err := json.Unmarshal(first, &batch); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid array"})
			return
		}
	} else {
		var one model.IngestObservation
		if err := json.Unmarshal(first, &one); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid object"})
			return
		}
		batch = []model.IngestObservation{one}
	}
	accepted, skipped := 0, 0
	for _, in := range batch {
		obs, inserted, err := s.Engine.IngestObservation(r.Context(), c.OrganizationID, in, c.Kind)
		if err != nil {
			skipped++
			continue
		}
		if inserted {
			accepted++
			s.Hub.Publish(c.OrganizationID, "observation", obs)
		} else {
			skipped++
		}
	}
	s.metrics.inc(&s.metrics.ingestOK)
	writeJSON(w, 202, map[string]int{"accepted": accepted, "duplicate_or_unknown": skipped})
}

func (s *Server) ingestInv(w http.ResponseWriter, r *http.Request, c *model.Connector) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	var in model.IngestInventory
	if err := readJSON(r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	if !validLatLng(in.Latitude, in.Longitude) {
		writeJSON(w, 400, map[string]string{"error": "latitude/longitude out of range"})
		return
	}
	a, err := s.Engine.IngestInventory(r.Context(), c.OrganizationID, in, c.Kind)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	s.Hub.Publish(c.OrganizationID, "inventory", a)
	writeJSON(w, 202, a)
}

func (s *Server) ingestEvt(w http.ResponseWriter, r *http.Request, c *model.Connector) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	var e model.Event
	if err := readJSON(r, &e); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	e.OrganizationID = c.OrganizationID
	if e.Kind == "" {
		e.Kind = "external"
	}
	if e.Severity == "" {
		e.Severity = "info"
	}
	ok, err := s.Store.InsertEvent(r.Context(), &e)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if ok {
		s.Hub.Publish(c.OrganizationID, "event", e)
	}
	writeJSON(w, 202, map[string]any{"accepted": ok, "event": e})
}

func (s *Server) spa() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		f, err := s.Static.Open(p)
		if err != nil {
			index, err2 := s.Static.Open("index.html")
			if err2 != nil {
				http.NotFound(w, r)
				return
			}
			defer index.Close()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.Copy(w, index)
			return
		}
		defer f.Close()
		stat, _ := f.Stat()
		if stat != nil && stat.IsDir() {
			index, err2 := s.Static.Open("index.html")
			if err2 != nil {
				http.NotFound(w, r)
				return
			}
			defer index.Close()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.Copy(w, index)
			return
		}
		http.ServeContent(w, r, p, time.Time{}, f.(io.ReadSeeker))
	})
}

func DefaultDSN() string {
	if v := os.Getenv("YARD_DATABASE_URL"); v != "" {
		return v
	}
	return "file:data/yard.db?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
}
