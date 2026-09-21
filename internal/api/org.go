package api

import (
	"net/http"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

func (s *Server) org(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		days, err := s.Store.RetentionDays(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		cents, err := s.Store.EnergyCentsPerKWh(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		grams, err := s.Store.CarbonGrams(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		ackMin, resolveMin, err := s.Store.SLAMinutes(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		tariffs, _ := s.Store.ListTariffs(r.Context(), u.OrganizationID)
		writeJSON(w, 200, map[string]any{"id": u.OrganizationID, "retention_days": days, "energy_cents_per_kwh": cents, "carbon_grams_per_kwh": grams, "ack_minutes": ackMin, "resolve_minutes": resolveMin, "tariffs": tariffs})
	case http.MethodPatch:
		if !s.requireAdmin(w, u) {
			return
		}
		var in struct {
			RetentionDays     *int            `json:"retention_days"`
			EnergyCentsPerKWh *int            `json:"energy_cents_per_kwh"`
			CarbonGramsPerKWh *int            `json:"carbon_grams_per_kwh"`
			AckMinutes        *int            `json:"ack_minutes"`
			ResolveMinutes    *int            `json:"resolve_minutes"`
			Tariffs           *[]store.Tariff `json:"tariffs"`
		}
		if err := readJSON(r, &in); err != nil || (in.RetentionDays == nil && in.EnergyCentsPerKWh == nil && in.CarbonGramsPerKWh == nil && in.AckMinutes == nil && in.ResolveMinutes == nil && in.Tariffs == nil) {
			writeJSON(w, 400, map[string]string{"error": "retention_days or energy_cents_per_kwh required"})
			return
		}
		if in.RetentionDays != nil {
			if *in.RetentionDays < 0 || *in.RetentionDays > 3650 {
				writeJSON(w, 400, map[string]string{"error": "retention_days must be 0 to 3650"})
				return
			}
			if err := s.Store.SetRetentionDays(r.Context(), u.OrganizationID, *in.RetentionDays); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "org.retention", u.OrganizationID, "")
		}
		if in.EnergyCentsPerKWh != nil {
			if *in.EnergyCentsPerKWh < 0 || *in.EnergyCentsPerKWh > 100000 {
				writeJSON(w, 400, map[string]string{"error": "energy_cents_per_kwh must be 0 to 100000"})
				return
			}
			if err := s.Store.SetEnergyCentsPerKWh(r.Context(), u.OrganizationID, *in.EnergyCentsPerKWh); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
		}
		if in.CarbonGramsPerKWh != nil {
			if *in.CarbonGramsPerKWh < 0 {
				writeJSON(w, 400, map[string]string{"error": "carbon_grams_per_kwh must be >= 0"})
				return
			}
			if err := s.Store.SetCarbonGrams(r.Context(), u.OrganizationID, *in.CarbonGramsPerKWh); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
		}
		if in.Tariffs != nil {
			if err := s.Store.ReplaceTariffs(r.Context(), u.OrganizationID, *in.Tariffs); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
		}
		if in.AckMinutes != nil || in.ResolveMinutes != nil {
			ackMin, resolveMin, err := s.Store.SLAMinutes(r.Context(), u.OrganizationID)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			if in.AckMinutes != nil {
				ackMin = *in.AckMinutes
			}
			if in.ResolveMinutes != nil {
				resolveMin = *in.ResolveMinutes
			}
			if ackMin < 0 || ackMin > 10080 || resolveMin < 0 || resolveMin > 10080 {
				writeJSON(w, 400, map[string]string{"error": "ack_minutes and resolve_minutes must be 0 to 10080"})
				return
			}
			if err := s.Store.SetSLAMinutes(r.Context(), u.OrganizationID, ackMin, resolveMin); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
		}
		days, _ := s.Store.RetentionDays(r.Context(), u.OrganizationID)
		cents, _ := s.Store.EnergyCentsPerKWh(r.Context(), u.OrganizationID)
		writeJSON(w, 200, map[string]any{"id": u.OrganizationID, "retention_days": days, "energy_cents_per_kwh": cents})
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}
