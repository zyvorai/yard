package api

import (
	"net/http"
	"strings"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

func needsPermit(wo *model.WorkOrder) bool {
	if wo.Kind == "permit" {
		return true
	}
	c := strings.TrimSpace(wo.Checklist)
	return c != "" && c != "[]" && c != "null"
}

func (s *Server) workOrderPermits(w http.ResponseWriter, r *http.Request, u *model.User, wo *model.WorkOrder, extra string) {
	rest := strings.Trim(strings.TrimPrefix(extra, "permits"), "/")
	if rest == "" && r.Method == http.MethodGet {
		list, err := s.Store.ListPermits(r.Context(), u.OrganizationID, wo.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
		return
	}
	if rest == "" && r.Method == http.MethodPost {
		if !s.requireWrite(w, u) {
			return
		}
		p := &store.Permit{OrganizationID: u.OrganizationID, WorkOrderID: wo.ID}
		if err := s.Store.CreatePermit(r.Context(), p); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, p)
		return
	}
	if r.Method == http.MethodPost && strings.HasSuffix(rest, "approve") {
		if !s.requireWrite(w, u) {
			return
		}
		id := strings.Trim(strings.TrimSuffix(rest, "approve"), "/")
		if err := s.Store.ApprovePermit(r.Context(), u.OrganizationID, id, u.ID); err != nil {
			writeJSON(w, 409, map[string]string{"error": "permit is not pending"})
			return
		}
		writeJSON(w, 200, map[string]string{"id": id, "status": "approved"})
		return
	}
	writeJSON(w, 404, map[string]string{"error": "not found"})
}
