package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/zyvorai/yard/internal/connectors"
	"github.com/zyvorai/yard/internal/idgen"
	"github.com/zyvorai/yard/internal/store"
)

// Worker claims durable jobs and executes connector actions.
type Worker struct {
	Store    *store.Store
	Dispatch *connectors.Dispatcher
	Log      *slog.Logger
	Name     string
}

func (w *Worker) Start(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = 2 * time.Second
	}
	if w.Name == "" {
		w.Name = idgen.New("worker")
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_ = w.Tick(ctx)
			}
		}
	}()
}

func (w *Worker) Tick(ctx context.Context) error {
	job, err := w.Store.ClaimJob(ctx, w.Name)
	if err != nil || job == nil {
		return err
	}
	status, result, runErr := w.run(ctx, job)
	if runErr != nil {
		if job.Attempts >= job.MaxAttempts {
			return w.Store.FailJob(ctx, job.ID, status, result, runErr.Error(), true)
		}
		backoff := time.Duration(job.Attempts*job.Attempts) * time.Second
		if backoff < 2*time.Second {
			backoff = 2 * time.Second
		}
		return w.Store.RetryJob(ctx, job.ID, result, runErr.Error(), time.Now().UTC().Add(backoff))
	}
	return w.Store.CompleteJob(ctx, job.ID, status, result)
}

func (w *Worker) run(ctx context.Context, job *store.Job) (status, result string, err error) {
	switch job.Kind {
	case "remote_action":
		var p struct {
			ActionID    string `json:"action_id"`
			ConnectorID string `json:"connector_id"`
			Action      string `json:"action"`
			Payload     string `json:"payload"`
		}
		if json.Unmarshal([]byte(job.Payload), &p) != nil || p.ActionID == "" {
			return "failed", "", fmt.Errorf("invalid remote_action payload")
		}
		conn, err := w.Store.ConnectorByID(ctx, job.OrganizationID, p.ConnectorID)
		if err != nil {
			_ = w.Store.CompleteAction(ctx, p.ActionID, "failed", "connector not found")
			return "failed", "", err
		}
		_ = w.Store.SetActionStatus(ctx, p.ActionID, "running", "")
		st, res, err := w.Dispatch.Execute(ctx, job.OrganizationID, conn, p.Action, p.Payload)
		_ = w.Store.CompleteAction(ctx, p.ActionID, st, res)
		return st, res, err
	default:
		return "failed", "", fmt.Errorf("unknown job kind %q", job.Kind)
	}
}

// EnqueueRemoteAction records a queued action and a durable job.
func EnqueueRemoteAction(ctx context.Context, st *store.Store, orgID, actionID, connectorID, action, payload string) (*store.Job, error) {
	body, _ := json.Marshal(map[string]string{
		"action_id": actionID, "connector_id": connectorID, "action": action, "payload": payload,
	})
	job := &store.Job{
		OrganizationID: orgID,
		Kind:           "remote_action",
		Status:         "queued",
		MaxAttempts:    5,
		Payload:        string(body),
		RunAfter:       time.Now().UTC(),
	}
	if err := st.CreateJob(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}
