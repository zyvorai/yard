package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

type jobView struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Status      string    `json:"status"`
	Attempts    int       `json:"attempts"`
	MaxAttempts int       `json:"max_attempts"`
	ActionID    string    `json:"action_id,omitempty"`
	RequestedBy string    `json:"requested_by,omitempty"`
	Error       string    `json:"error,omitempty"`
	Result      string    `json:"result,omitempty"`
	RunAfter    time.Time `json:"run_after"`
	CreatedAt   time.Time `json:"created_at"`
}

func presentJob(j store.Job) jobView {
	return jobView{
		ID: j.ID, Kind: j.Kind, Status: j.Status,
		Attempts: j.Attempts, MaxAttempts: j.MaxAttempts,
		ActionID: actionIDFromPayload(j.Payload), RequestedBy: requestedByFromPayload(j.Payload),
		Error: j.Error, Result: j.Result,
		RunAfter: j.RunAfter, CreatedAt: j.CreatedAt,
	}
}

func actionIDFromPayload(payload string) string {
	return jobPayload(payload).ActionID
}

func requestedByFromPayload(payload string) string {
	return jobPayload(payload).RequestedBy
}

func jobPayload(payload string) struct {
	ActionID    string `json:"action_id"`
	RequestedBy string `json:"requested_by"`
} {
	var p struct {
		ActionID    string `json:"action_id"`
		RequestedBy string `json:"requested_by"`
	}
	_ = json.Unmarshal([]byte(payload), &p)
	return p
}

func (s *Server) jobs(w http.ResponseWriter, r *http.Request, u *model.User) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	list, err := s.Store.ListJobs(r.Context(), u.OrganizationID, 50)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	out := make([]jobView, 0, len(list))
	for _, j := range list {
		out = append(out, presentJob(j))
	}
	writeJSON(w, 200, out)
}

func (s *Server) jobItem(w http.ResponseWriter, r *http.Request, u *model.User) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/jobs/")
	id, op, ok := strings.Cut(rest, "/")
	if !ok || id == "" || (op != "retry" && op != "cancel" && op != "approve") || r.Method != http.MethodPost {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if !s.requireWrite(w, u) {
		return
	}
	job, err := s.Store.JobForOrg(r.Context(), u.OrganizationID, id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, 404, map[string]string{"error": "job not found"})
			return
		}
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	actionID := actionIDFromPayload(job.Payload)
	switch op {
	case "cancel":
		if err := s.Store.CancelQueuedJob(r.Context(), u.OrganizationID, id); err != nil {
			if err == sql.ErrNoRows {
				writeJSON(w, 409, map[string]string{"error": "only a queued or pending job can be cancelled"})
				return
			}
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		if actionID != "" {
			_ = s.Store.SetActionStatus(r.Context(), actionID, "cancelled", "cancelled by operator")
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "job.cancel", id, job.Kind)
	case "retry":
		if err := s.Store.RequeueDeadJob(r.Context(), u.OrganizationID, id, time.Now().UTC()); err != nil {
			if err == sql.ErrNoRows {
				writeJSON(w, 409, map[string]string{"error": "only a dead or failed job can be retried"})
				return
			}
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		if actionID != "" {
			_ = s.Store.SetActionStatus(r.Context(), actionID, "queued", "retry requested")
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "job.retry", id, job.Kind)
	case "approve":
		if requestedByFromPayload(job.Payload) == "" || requestedByFromPayload(job.Payload) == u.ID {
			writeJSON(w, 403, map[string]string{"error": "a different operator must approve this action"})
			return
		}
		if err := s.Store.ApproveJob(r.Context(), u.OrganizationID, id, time.Now().UTC()); err != nil {
			if err == sql.ErrNoRows {
				writeJSON(w, 409, map[string]string{"error": "only a pending job can be approved"})
				return
			}
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		if actionID != "" {
			_ = s.Store.SetActionStatus(r.Context(), actionID, "queued", "approved")
		}
		_ = s.Store.Audit(r.Context(), u.OrganizationID, u.Email, "job.approve", id, job.Kind)
	}
	updated, err := s.Store.JobForOrg(r.Context(), u.OrganizationID, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, presentJob(*updated))
}
