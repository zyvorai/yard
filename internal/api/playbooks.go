package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/playbook"
	"github.com/zyvorai/yard/internal/queue"
)

func (s *Server) playbooks(w http.ResponseWriter, r *http.Request, u *model.User) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.Store.ListPlaybooks(r.Context(), u.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		if !s.requireWrite(w, u) {
			return
		}
		body, source, err := s.readPlaybookBody(r)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		doc, err := playbook.Parse(body)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		pb := &model.Playbook{OrganizationID: u.OrganizationID, Name: doc.Name, Body: body, SourceURL: source}
		if err := s.Store.CreatePlaybook(r.Context(), pb); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, pb)
	default:
		writeJSON(w, 405, map[string]string{"error": "method"})
	}
}

func (s *Server) playbookItem(w http.ResponseWriter, r *http.Request, u *model.User) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/playbooks/"), "/")
	id, op, ok := strings.Cut(rest, "/")
	if !ok || op != "run" || r.Method != http.MethodPost || id == "" {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if !s.requireWrite(w, u) {
		return
	}
	pb, err := s.Store.PlaybookByID(r.Context(), u.OrganizationID, id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	doc, err := playbook.Parse(pb.Body)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	var in struct {
		AssetID     string `json:"asset_id"`
		ConnectorID string `json:"connector_id"`
		DryRun      bool   `json:"dry_run"`
	}
	if err := readJSON(r, &in); err != nil || in.AssetID == "" {
		writeJSON(w, 400, map[string]string{"error": "asset_id required"})
		return
	}
	if _, err := s.Store.AssetByID(r.Context(), u.OrganizationID, in.AssetID); err != nil {
		writeJSON(w, 404, map[string]string{"error": "asset not found"})
		return
	}
	run := &model.PlaybookRun{
		OrganizationID: u.OrganizationID,
		PlaybookID:     pb.ID,
		AssetID:        in.AssetID,
		ConnectorID:    in.ConnectorID,
		RequestedBy:    u.ID,
		DryRun:         in.DryRun,
		Status:         "dry_run",
	}
	if in.DryRun {
		if err := s.Store.CreatePlaybookRun(r.Context(), run); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"run": run, "steps": doc.Steps})
		return
	}
	if in.ConnectorID == "" {
		writeJSON(w, 400, map[string]string{"error": "connector_id required"})
		return
	}
	if _, err := s.Store.ConnectorByID(r.Context(), u.OrganizationID, in.ConnectorID); err != nil {
		writeJSON(w, 404, map[string]string{"error": "connector not found"})
		return
	}
	status := "queued"
	for _, step := range doc.Steps {
		if queue.NeedsApproval(step.Action) {
			status = "pending_approval"
			break
		}
	}
	run.Status = status
	if err := s.Store.CreatePlaybookRun(r.Context(), run); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	job, err := queue.EnqueuePlaybook(r.Context(), s.Store, u.OrganizationID, run.ID, u.ID, status)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	run.JobID = job.ID
	_ = s.Store.SavePlaybookRun(r.Context(), run)
	writeJSON(w, 202, map[string]any{"run": run, "steps": doc.Steps})
}

func (s *Server) playbookRunItem(w http.ResponseWriter, r *http.Request, u *model.User) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/playbook-runs/"), "/")
	id, op, ok := strings.Cut(rest, "/")
	if !ok || op != "approve" || r.Method != http.MethodPost || id == "" {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if !s.requireWrite(w, u) {
		return
	}
	run, err := s.Store.PlaybookRunByID(r.Context(), u.OrganizationID, id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if run.RequestedBy == "" || run.RequestedBy == u.ID {
		writeJSON(w, 403, map[string]string{"error": "a different operator must approve this action"})
		return
	}
	if run.Status != "pending_approval" || run.JobID == "" {
		writeJSON(w, 409, map[string]string{"error": "only a pending run can be approved"})
		return
	}
	if err := s.Store.ApproveJob(r.Context(), u.OrganizationID, run.JobID, time.Now().UTC()); err != nil {
		writeJSON(w, 409, map[string]string{"error": "only a pending job can be approved"})
		return
	}
	run.Status = "queued"
	run.ApprovedBy = u.ID
	if err := s.Store.SavePlaybookRun(r.Context(), run); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, run)
}

func (s *Server) readPlaybookBody(r *http.Request) (body, source string, err error) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 256<<10))
	if err != nil {
		return "", "", err
	}
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "yaml") || (len(raw) > 0 && raw[0] != '{' && raw[0] != '[') {
		return string(raw), "", nil
	}
	var in struct {
		Body      string `json:"body"`
		SourceURL string `json:"source_url"`
	}
	if err := readJSONBytes(raw, &in); err != nil {
		return "", "", errors.New("body or source_url required")
	}
	if in.SourceURL != "" && in.Body == "" {
		fetched, err := s.fetchPlaybook(r, in.SourceURL)
		if err != nil {
			return "", "", err
		}
		return fetched, in.SourceURL, nil
	}
	if in.Body == "" {
		return "", "", errors.New("body or source_url required")
	}
	return in.Body, in.SourceURL, nil
}

func readJSONBytes(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func (s *Server) fetchPlaybook(r *http.Request, rawURL string) (string, error) {
	if s.Runtime.Egress == nil {
		return "", errors.New("egress policy unavailable")
	}
	if err := s.Runtime.Egress.Validate(rawURL); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := s.Runtime.Egress.HTTPClient(8*time.Second, false).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", errors.New("playbook fetch failed")
	}
	buf, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return "", err
	}
	return string(buf), nil
}
