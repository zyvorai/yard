package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

func (s *Server) catalogs(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListCatalogs(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		if !s.requireWrite(w, u) {
			return
		}
		var c store.Catalog
		if err := readJSON(r, &c); err != nil || c.Name == "" {
			writeJSON(w, 400, map[string]string{"error": "name required"})
			return
		}
		if err := s.Store.CreateCatalog(r.Context(), u.OrganizationID, &c); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, c)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) savedViews(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListSavedViews(r.Context(), u.OrganizationID, u.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		var v store.SavedView
		if err := readJSON(r, &v); err != nil || v.Name == "" || v.Kind == "" {
			writeJSON(w, 400, map[string]string{"error": "name and kind required"})
			return
		}
		v.UserID = u.ID
		if err := s.Store.CreateSavedView(r.Context(), u.OrganizationID, &v); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, v)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) orgs(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	_ = s.Store.EnsureOrgMember(r.Context(), u.OrganizationID, u.ID, u.Role)
	list, err := s.Store.ListUserOrgs(r.Context(), u.ID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) switchOrg(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	var in struct {
		OrganizationID string `json:"organization_id"`
	}
	if err := readJSON(r, &in); err != nil || in.OrganizationID == "" {
		writeJSON(w, 400, map[string]string{"error": "organization_id required"})
		return
	}
	list, err := s.Store.ListUserOrgs(r.Context(), u.ID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	ok := false
	for _, o := range list {
		if o.ID == in.OrganizationID {
			ok = true
			break
		}
	}
	if !ok {
		writeJSON(w, 403, map[string]string{"error": "not a member"})
		return
	}
	if err := s.Store.SetUserOrg(r.Context(), u.ID, in.OrganizationID); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	u.OrganizationID = in.OrganizationID
	writeJSON(w, 200, u)
}

func (s *Server) mePrefs(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		widgetsRaw, _ := s.Store.UserWidgets(r.Context(), u.ID)
		locale, _ := s.Store.UserLocale(r.Context(), u.ID)
		var widgets []string
		_ = json.Unmarshal([]byte(defaultWidgets(widgetsRaw)), &widgets)
		if widgets == nil {
			widgets = []string{"health", "incidents", "work", "activity"}
		}
		writeJSON(w, 200, map[string]any{"widgets": widgets, "locale": locale})
	case http.MethodPatch:
		var in struct {
			Widgets *[]string `json:"widgets"`
			Locale  *string   `json:"locale"`
		}
		if err := readJSON(r, &in); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		if in.Widgets != nil {
			raw, _ := json.Marshal(in.Widgets)
			if err := s.Store.SetUserWidgets(r.Context(), u.ID, string(raw)); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
		}
		if in.Locale != nil {
			if err := s.Store.SetUserLocale(r.Context(), u.ID, *in.Locale); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
		}
		writeJSON(w, 200, map[string]string{"ok": "true"})
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func defaultWidgets(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return `["health","incidents","work","activity"]`
	}
	return raw
}

func (s *Server) i18n(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "en"
	}
	stringsEN := map[string]string{
		"app.name": "Yard", "nav.overview": "Overview", "nav.assets": "Assets", "nav.incidents": "Incidents",
		"nav.work": "Work orders", "action.sign_in": "Sign in",
	}
	stringsHI := map[string]string{
		"app.name": "Yard", "nav.overview": "अवलोकन", "nav.assets": "संपत्ति", "nav.incidents": "घटनाएँ",
		"nav.work": "कार्य आदेश", "action.sign_in": "साइन इन",
	}
	out := stringsEN
	if strings.HasPrefix(strings.ToLower(lang), "hi") {
		out = stringsHI
	}
	writeJSON(w, 200, map[string]any{"lang": lang, "strings": out})
}

func (s *Server) schedules(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	list, err := s.Store.ListWorkOrders(r.Context(), u.OrganizationID, "")
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	out := []model.WorkOrder{}
	for _, wo := range list {
		if wo.ScheduleCron != "" {
			out = append(out, wo)
		}
	}
	writeJSON(w, 200, out)
}

func (s *Server) telemetryExport(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}
	list, err := s.Store.ListObservations(r.Context(), u.OrganizationID, r.URL.Query().Get("asset_id"), r.URL.Query().Get("capability"), time.Time{}, time.Time{}, 500)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		_, _ = io.WriteString(w, "observed_at,asset_id,capability,value,value_kind,value_text,unit,quality,quality_reason,sequence_num,uncertainty,calibration_state\n")
		for _, o := range list {
			unc := ""
			if o.Uncertainty != nil {
				unc = fmt.Sprintf("%g", *o.Uncertainty)
			}
			_, _ = io.WriteString(w, o.ObservedAt.UTC().Format(time.RFC3339)+","+o.AssetID+","+csvCell(o.Capability)+","+fmt.Sprintf("%g", o.Value)+","+o.ValueKind+","+csvCell(o.ValueText)+","+csvCell(o.Unit)+","+csvCell(o.Quality)+","+csvCell(o.QualityReason)+","+fmt.Sprintf("%d", o.SequenceNum)+","+unc+","+csvCell(o.Calibration)+"\n")
		}
	case "prometheus":
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		for _, o := range list {
			if o.ValueKind != "" && o.ValueKind != "number" {
				continue
			}
			_, _ = io.WriteString(w, fmt.Sprintf("yard_observation{asset=%q,capability=%q} %g %d\n", o.AssetID, o.Capability, o.Value, o.ObservedAt.UnixMilli()))
		}
	case "otlp":
		points := make([]map[string]any, 0, len(list))
		for _, o := range list {
			points = append(points, map[string]any{
				"timeUnixNano": strconv.FormatInt(o.ObservedAt.UnixNano(), 10),
				"asDouble":     o.Value,
				"attributes": []map[string]any{
					{"key": "yard.asset", "value": map[string]string{"stringValue": o.AssetID}},
					{"key": "capability", "value": map[string]string{"stringValue": o.Capability}},
				},
			})
		}
		writeJSON(w, 200, map[string]any{"resourceMetrics": []map[string]any{{"scopeMetrics": []map[string]any{{"metrics": []map[string]any{{"name": "yard.observation", "gauge": map[string]any{"dataPoints": points}}}}}}}})
	default:
		writeJSON(w, 400, map[string]string{"error": "format must be csv, prometheus, or otlp"})
	}
}

func (s *Server) assetBOM(w http.ResponseWriter, r *http.Request, u *model.User, a *model.Asset) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListBOM(r.Context(), u.OrganizationID, a.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		if !s.requireWrite(w, u) {
			return
		}
		var line store.BOMLine
		if err := readJSON(r, &line); err != nil || line.PartNumber == "" || line.Name == "" {
			writeJSON(w, 400, map[string]string{"error": "part_number and name required"})
			return
		}
		line.AssetID = a.ID
		if err := s.Store.AddBOMLine(r.Context(), u.OrganizationID, &line); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, line)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) assetInstalls(w http.ResponseWriter, r *http.Request, u *model.User, a *model.Asset) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	list, err := s.Store.ListInstallHistory(r.Context(), u.OrganizationID, a.ID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, list)
}

func (s *Server) workOrderExtras(w http.ResponseWriter, r *http.Request, u *model.User, wo *model.WorkOrder, extra string) bool {
	switch {
	case extra == "assets":
		switch r.Method {
		case http.MethodGet:
			ids, err := s.Store.ListWorkOrderAssets(r.Context(), wo.ID)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return true
			}
			writeJSON(w, 200, ids)
		case http.MethodPut:
			if !s.requireWrite(w, u) {
				return true
			}
			var in struct {
				AssetIDs []string `json:"asset_ids"`
			}
			if err := readJSON(r, &in); err != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid json"})
				return true
			}
			if err := s.Store.SetWorkOrderAssets(r.Context(), wo.ID, in.AssetIDs); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return true
			}
			writeJSON(w, 200, in.AssetIDs)
		default:
			writeJSON(w, 405, map[string]string{"error": "method"})
		}
		return true
	case extra == "time":
		switch r.Method {
		case http.MethodGet:
			list, err := s.Store.ListTimeEntries(r.Context(), u.OrganizationID, wo.ID)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return true
			}
			writeJSON(w, 200, list)
		case http.MethodPost:
			if !s.requireWrite(w, u) {
				return true
			}
			var e store.TimeEntry
			if err := readJSON(r, &e); err != nil || e.Minutes <= 0 {
				writeJSON(w, 400, map[string]string{"error": "minutes required"})
				return true
			}
			e.WorkOrderID = wo.ID
			if e.Actor == "" {
				e.Actor = u.Email
			}
			if err := s.Store.AddTimeEntry(r.Context(), u.OrganizationID, &e); err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return true
			}
			writeJSON(w, 201, e)
		default:
			writeJSON(w, 405, map[string]string{"error": "method"})
		}
		return true
	case extra == "print":
		if r.Method != http.MethodGet {
			writeJSON(w, 405, map[string]string{"error": "method"})
			return true
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!doctype html><html><head><title>"+wo.Title+"</title></head><body>")
		_, _ = io.WriteString(w, "<h1>"+wo.Title+"</h1><p>Status: "+wo.Status+" · Priority: "+wo.Priority+" · Assignee: "+wo.Assignee+"</p>")
		_, _ = io.WriteString(w, "<p>"+wo.Notes+"</p></body></html>")
		return true
	}
	return false
}
