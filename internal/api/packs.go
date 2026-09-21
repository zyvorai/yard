package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
	"github.com/zyvorai/yard/packs"
)

func (s *Server) importPack(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	if !s.requireAdmin(w, u) {
		return
	}
	name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/packs/"), "/")
	raw, err := packs.Read(name)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "unknown pack"})
		return
	}
	file, err := packs.Parse(raw)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	var templateIDs []string
	for _, t := range file.Templates {
		caps, err := json.Marshal(t.Capabilities)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		row := &store.AssetTemplate{OrganizationID: u.OrganizationID, Name: t.Name, Kind: t.Kind, Capabilities: string(caps)}
		if err := s.Store.CreateAssetTemplate(r.Context(), row); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		templateIDs = append(templateIDs, row.ID)
	}
	panels := make([]model.DashboardPanel, 0, len(file.Dashboard.Capabilities))
	for _, cap := range file.Dashboard.Capabilities {
		panels = append(panels, model.DashboardPanel{Capability: cap, Title: cap})
	}
	dash := &model.Dashboard{OrganizationID: u.OrganizationID, Name: file.Dashboard.Name, Panels: panels}
	if err := s.Store.CreateDashboard(r.Context(), dash); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	var automationIDs []string
	for _, rule := range file.Automations {
		row := &model.Automation{
			OrganizationID: u.OrganizationID,
			Name:           rule.Name,
			Enabled:        true,
			TriggerKind:    "threshold",
			Capability:     rule.Capability,
			Operator:       rule.Operator,
			Threshold:      rule.Threshold,
			Action:         rule.Action,
		}
		if err := s.Store.CreateAutomation(r.Context(), row); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		automationIDs = append(automationIDs, row.ID)
	}
	wo := &model.WorkOrder{OrganizationID: u.OrganizationID, Title: file.WorkOrder, Kind: "maintenance"}
	if err := s.Store.CreateWorkOrder(r.Context(), wo); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"pack": file.Name, "template_ids": templateIDs, "dashboard_id": dash.ID, "automation_ids": automationIDs, "work_order_id": wo.ID})
}

func (s *Server) costReport(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	out, err := s.Store.CostReport(r.Context(), u.OrganizationID, time.Now().UTC())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, out)
}
