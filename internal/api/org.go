package api

import (
	"net/http"

	"github.com/zyvorai/yard/internal/model"
)

func (s *Server) org(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		days, err := s.Store.RetentionDays(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"id": u.OrganizationID, "retention_days": days})
	case http.MethodPatch:
		if !s.requireAdmin(w, u) {
			return
		}
		var in struct {
			RetentionDays *int `json:"retention_days"`
		}
		if err := readJSON(r, &in); err != nil || in.RetentionDays == nil {
			writeJSON(w, 400, map[string]string{"error": "retention_days required"})
			return
		}
		if *in.RetentionDays < 0 || *in.RetentionDays > 3650 {
			writeJSON(w, 400, map[string]string{"error": "retention_days must be 0 to 3650"})
			return
		}
		if err := s.Store.SetRetentionDays(r.Context(), u.OrganizationID, *in.RetentionDays); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "org.retention", u.OrganizationID, "")
		writeJSON(w, 200, map[string]any{"id": u.OrganizationID, "retention_days": *in.RetentionDays})
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}
