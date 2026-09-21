package api

import (
	"net/http"
	"time"

	"github.com/zyvorai/yard/internal/model"
)

func (s *Server) oncall(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListOnCall(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		if !s.requireWrite(w, u) {
			return
		}
		var row model.OnCall
		if err := readJSON(r, &row); err != nil || row.UserID == "" || row.EndsAt.IsZero() || !row.EndsAt.After(row.StartsAt) {
			writeJSON(w, 400, map[string]string{"error": "user_id, starts_at, and ends_at required"})
			return
		}
		person, err := s.Store.UserByID(r.Context(), row.UserID)
		if err != nil || person.OrganizationID != u.OrganizationID {
			writeJSON(w, 400, map[string]string{"error": "user not found"})
			return
		}
		if row.StartsAt.IsZero() {
			row.StartsAt = time.Now().UTC()
		}
		if err := s.Store.CreateOnCall(r.Context(), u.OrganizationID, &row); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		row.DisplayName = person.DisplayName
		writeJSON(w, 201, row)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}
