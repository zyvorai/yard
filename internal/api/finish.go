package api

import (
	"crypto/subtle"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/totp"
)

func (s *Server) peerEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	want := os.Getenv("YARD_PEER_TOKEN")
	if want == "" {
		writeJSON(w, 404, map[string]string{"error": "peer token is not configured"})
		return
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	var body struct {
		Events []model.Event `json:"events"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	n := 0
	for i := range body.Events {
		ev := body.Events[i]
		if ev.OrganizationID == "" || ev.Kind == "" || ev.Title == "" {
			continue
		}
		if ok, err := s.Store.OrgExists(r.Context(), ev.OrganizationID); err != nil || !ok {
			continue
		}
		ev.Replicated = true
		if ev.Region == "" {
			ev.Region = os.Getenv("YARD_REGION")
		}
		if _, err := s.Store.InsertEvent(r.Context(), &ev); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		n++
	}
	writeJSON(w, 202, map[string]int{"accepted": n})
}

func (s *Server) totpSetup(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	secret, err := totp.GenerateSecret()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if err := s.Store.SetUserTOTP(r.Context(), u.ID, "pending:"+secret); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{
		"secret": secret,
		"url":    "otpauth://totp/Yard:" + u.Email + "?secret=" + secret + "&issuer=Yard",
	})
}

func (s *Server) totpConfirm(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	var in struct {
		Code string `json:"code"`
	}
	if err := readJSON(r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	stored, err := s.Store.UserTOTP(r.Context(), u.ID)
	if err != nil || !strings.HasPrefix(stored, "pending:") {
		writeJSON(w, 400, map[string]string{"error": "authenticator setup is not pending"})
		return
	}
	secret := strings.TrimPrefix(stored, "pending:")
	if !totp.Verify(secret, in.Code, time.Now()) {
		writeJSON(w, 401, map[string]string{"error": "otp required"})
		return
	}
	if err := s.Store.SetUserTOTP(r.Context(), u.ID, secret); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]bool{"enabled": true})
}

func (s *Server) telemetryQuery(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	q := r.URL.Query()
	var from, to time.Time
	if raw := q.Get("from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "from must be RFC3339"})
			return
		}
		from = parsed
	}
	if raw := q.Get("to"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "to must be RFC3339"})
			return
		}
		to = parsed
	}
	list, err := s.Store.ListObservations(r.Context(), u.OrganizationID, q.Get("asset_id"), q.Get("capability"), from, to, 400)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) complianceExport(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	if !s.requireAdmin(w, u) {
		return
	}
	users, err := s.Store.ListUsers(r.Context(), u.OrganizationID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	audit, err := s.Store.ListAudit(r.Context(), u.OrganizationID, 500)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	_, _ = io.WriteString(w, "section,a,b,c,d\nedition,community,,,\n")
	for _, person := range users {
		active := "false"
		if person.Active {
			active = "true"
		}
		_, _ = io.WriteString(w, "user,"+csvCell(person.Email)+","+csvCell(person.Role)+","+active+",\n")
	}
	for _, row := range audit {
		_, _ = io.WriteString(w, "audit,"+row.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")+","+csvCell(row.Actor)+","+csvCell(row.Action)+","+csvCell(row.Object)+"\n")
	}
}
