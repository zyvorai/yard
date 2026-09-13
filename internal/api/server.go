package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/zyvorai/estate/internal/idgen"
	"github.com/zyvorai/estate/internal/jobs"
	"github.com/zyvorai/estate/internal/model"
	"github.com/zyvorai/estate/internal/sse"
	"github.com/zyvorai/estate/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	Store   *store.Store
	Engine  *jobs.Engine
	Hub     *sse.Hub
	Static  fs.FS
	Log     *log.Logger
	tokens  map[string]string
}

func New(st *store.Store, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}
	return &Server{
		Store:  st,
		Engine: &jobs.Engine{Store: st, Log: logger},
		Hub:    sse.New(),
		Log:    logger,
		tokens: map[string]string{},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ready")) })
	mux.HandleFunc("/api/v1/auth/login", s.login)
	mux.HandleFunc("/api/v1/auth/me", s.withUser(s.me))
	mux.HandleFunc("/api/v1/overview", s.withUser(s.overview))
	mux.HandleFunc("/api/v1/sites", s.withUser(s.sites))
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
	mux.HandleFunc("/api/v1/audit", s.withUser(s.audit))
	mux.HandleFunc("/api/v1/stream", s.withUser(s.stream))
	mux.HandleFunc("/api/v1/onboarding", s.withUser(s.onboarding))
	mux.HandleFunc("/api/v1/ingest/observations", s.withConnector(s.ingestObs))
	mux.HandleFunc("/api/v1/ingest/inventory", s.withConnector(s.ingestInv))
	mux.HandleFunc("/api/v1/ingest/events", s.withConnector(s.ingestEvt))
	if s.Static != nil {
		mux.Handle("/", s.spa())
	}
	return cors(mux)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Estate-Token")
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

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	return r.Header.Get("X-Estate-Token")
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
		writeJSON(w, 401, map[string]string{"error": "invalid credentials"})
		return
	}
	sess, err := s.Store.CreateSession(r.Context(), u.ID, 12*time.Hour)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "session"})
		return
	}
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
		var site model.Site
		if err := readJSON(r, &site); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		site.OrganizationID = u.OrganizationID
		if site.Kind == "" {
			site.Kind = "site"
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
		var a model.Asset
		if err := readJSON(r, &a); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		a.OrganizationID = u.OrganizationID
		if a.Kind == "" {
			a.Kind = "equipment"
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
	a, err := s.Store.AssetByID(r.Context(), u.OrganizationID, id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if len(parts) == 1 && r.Method == http.MethodGet {
		caps, _ := s.Store.ListCapabilities(r.Context(), a.ID)
		obs, _ := s.Store.ListObservations(r.Context(), u.OrganizationID, a.ID, "", 80)
		writeJSON(w, 200, map[string]any{"asset": a, "capabilities": caps, "observations": obs})
		return
	}
	if len(parts) >= 2 && parts[1] == "observations" {
		cap := r.URL.Query().Get("capability")
		obs, err := s.Store.ListObservations(r.Context(), u.OrganizationID, a.ID, cap, 400)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, obs)
		return
	}
	writeJSON(w, 404, map[string]string{"error": "not found"})
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
		if inc.Severity == "" {
			inc.Severity = "warning"
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
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	list, err := s.Store.ListConnectors(r.Context(), u.OrganizationID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, list)
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
		if in.ConnectorID != "" {
			connID = &in.ConnectorID
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
		_ = s.Store.CompleteAction(r.Context(), act.ID, "recorded", "Delegated to connector; outcome recorded locally.")
		act.Status = "recorded"
		act.Result = "Delegated to connector; outcome recorded locally."
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "action.request", act.ID, act.Action)
		writeJSON(w, 202, act)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) automations(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method == http.MethodGet {
		list, err := s.Store.ListAutomations(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	} else if r.Method == http.MethodPatch {
		var in struct {
			ID      string `json:"id"`
			Enabled *bool  `json:"enabled"`
		}
		if err := readJSON(r, &in); err != nil || in.ID == "" || in.Enabled == nil {
			writeJSON(w, 400, map[string]string{"error": "id and enabled required"})
			return
		}
		if err := s.Store.SetAutomationEnabled(r.Context(), u.OrganizationID, in.ID, *in.Enabled); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"id": in.ID, "enabled": *in.Enabled})
	} else {
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
	writeJSON(w, 200, map[string]any{
		"steps": []map[string]string{
			{"id": "workspace", "title": "Workspace ready", "body": "Northwind Operations is your first tenant."},
			{"id": "source", "title": "Connect a source", "body": "Use the included simulator or point a Device Agent gateway at a local API."},
			{"id": "discover", "title": "Discover assets", "body": "Inventory arrives as generic assets — devices are one kind."},
			{"id": "health", "title": "Inspect health", "body": "Freshness, quality, and thresholds stay visible even when data is stale."},
			{"id": "alert", "title": "Create an alert", "body": "Temperature or missed heartbeat opens an incident you can assign."},
		},
	})
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
	if v := os.Getenv("ESTATE_DATABASE_URL"); v != "" {
		return v
	}
	return "file:data/estate.db?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
}
