package api

import (
	"net/http"
	"strings"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

func (s *Server) workOrderLines(w http.ResponseWriter, r *http.Request, u *model.User, wo *model.WorkOrder, rest string) {
	lineID := strings.Trim(strings.TrimPrefix(rest, "lines"), "/")
	if lineID == "" {
		switch r.Method {
		case http.MethodGet:
			list, err := s.Store.ListWorkOrderLines(r.Context(), u.OrganizationID, wo.ID)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, 200, list)
		case http.MethodPost:
			if !s.requireWrite(w, u) {
				return
			}
			var in store.WorkOrderLine
			if err := readJSON(r, &in); err != nil || strings.TrimSpace(in.Name) == "" {
				writeJSON(w, 400, map[string]string{"error": "name required"})
				return
			}
			if in.Kind != "part" && in.Kind != "labor" {
				writeJSON(w, 400, map[string]string{"error": "kind must be part or labor"})
				return
			}
			if in.Quantity < 0 || in.UnitCostCents < 0 {
				writeJSON(w, 400, map[string]string{"error": "quantity and cost must be >= 0"})
				return
			}
			in.OrganizationID = u.OrganizationID
			in.WorkOrderID = wo.ID
			in.Name = strings.TrimSpace(in.Name)
			if err := s.Store.CreateWorkOrderLine(r.Context(), &in); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "workorder.line", in.ID, in.Kind+" "+in.Name)
			writeJSON(w, 201, in)
		default:
			writeJSON(w, 405, map[string]string{"error": "method"})
		}
		return
	}
	if r.Method != http.MethodDelete {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	if !s.requireWrite(w, u) {
		return
	}
	if err := s.Store.DeleteWorkOrderLine(r.Context(), u.OrganizationID, wo.ID, lineID); err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, 200, map[string]string{"deleted": lineID})
}
