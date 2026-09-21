package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/zyvorai/yard/internal/connectors"
	"github.com/zyvorai/yard/internal/idgen"
	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/playbook"
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
	_ = w.Store.WithLeader(ctx, func(ctx context.Context) error {
		return w.scheduleDueSync(ctx)
	})
	job, err := w.Store.ClaimJob(ctx, w.Name)
	if err != nil || job == nil {
		return err
	}
	status, result, runErr := w.run(ctx, job)
	if runErr != nil {
		if job.Attempts >= job.MaxAttempts {
			return w.Store.FailJob(ctx, job.ID, status, result, runErr.Error(), true)
		}
		return w.Store.RetryJob(ctx, job.ID, result, runErr.Error(), time.Now().UTC().Add(Backoff(job.Attempts)))
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
		start := time.Now()
		st, res, err := w.Dispatch.Execute(ctx, job.OrganizationID, conn, p.Action, p.Payload)
		_ = w.Store.RecordConnectorProbe(ctx, conn.ID, errString(err), int(time.Since(start).Milliseconds()), err == nil)
		_ = w.Store.CompleteAction(ctx, p.ActionID, st, res)
		return st, res, err
	case "playbook":
		return w.runPlaybook(ctx, job)
	default:
		return "failed", "", fmt.Errorf("unknown job kind %q", job.Kind)
	}
}

// Backoff doubles from 2s on each attempt and stops at 60s.
// attempts is the count already recorded on the job (1 after the first claim).
func Backoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	shift := attempts - 1
	if shift > 5 {
		shift = 5
	}
	d := 2 * time.Second * time.Duration(uint(1)<<shift)
	if d > 60*time.Second {
		return 60 * time.Second
	}
	return d
}

// EnqueueRemoteAction records a durable job for a connector action.
// status is queued, or pending_approval when a second person must approve.
func EnqueueRemoteAction(ctx context.Context, st *store.Store, orgID, actionID, connectorID, action, payload, requestedBy, status string) (*store.Job, error) {
	if status == "" {
		status = "queued"
	}
	body, _ := json.Marshal(map[string]string{
		"action_id": actionID, "connector_id": connectorID, "action": action, "payload": payload, "requested_by": requestedBy,
	})
	job := &store.Job{
		OrganizationID: orgID,
		Kind:           "remote_action",
		Status:         status,
		MaxAttempts:    5,
		Payload:        string(body),
		RunAfter:       time.Now().UTC(),
	}
	if err := st.CreateJob(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

// EnqueuePlaybook records one job that runs every step of a playbook run.
func EnqueuePlaybook(ctx context.Context, st *store.Store, orgID, runID, requestedBy, status string) (*store.Job, error) {
	if status == "" {
		status = "queued"
	}
	body, _ := json.Marshal(map[string]string{"run_id": runID, "requested_by": requestedBy})
	job := &store.Job{
		OrganizationID: orgID,
		Kind:           "playbook",
		Status:         status,
		MaxAttempts:    5,
		Payload:        string(body),
		RunAfter:       time.Now().UTC(),
	}
	if err := st.CreateJob(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

func (w *Worker) runPlaybook(ctx context.Context, job *store.Job) (string, string, error) {
	var p struct {
		RunID string `json:"run_id"`
	}
	if json.Unmarshal([]byte(job.Payload), &p) != nil || p.RunID == "" {
		return "failed", "", fmt.Errorf("invalid playbook payload")
	}
	run, err := w.Store.PlaybookRunByID(ctx, job.OrganizationID, p.RunID)
	if err != nil {
		return "failed", "", err
	}
	pb, err := w.Store.PlaybookByID(ctx, job.OrganizationID, run.PlaybookID)
	if err != nil {
		return "failed", "", err
	}
	doc, err := playbook.Parse(pb.Body)
	if err != nil {
		return "failed", "", err
	}
	conn, err := w.Store.ConnectorByID(ctx, job.OrganizationID, run.ConnectorID)
	if err != nil {
		return "failed", "", err
	}
	for i := run.StepIndex; i < len(doc.Steps); i++ {
		step := doc.Steps[i]
		act := &model.ActionRequest{
			OrganizationID: job.OrganizationID,
			AssetID:        &run.AssetID,
			ConnectorID:    &run.ConnectorID,
			Action:         step.Action,
			IdempotencyKey: idgen.New("idem"),
			Status:         "running",
			Payload:        step.Payload,
			ExpiresAt:      time.Now().UTC().Add(15 * time.Minute),
		}
		if err := w.Store.CreateAction(ctx, act); err != nil {
			return "failed", "", err
		}
		st, res, err := w.Dispatch.Execute(ctx, job.OrganizationID, conn, step.Action, step.Payload)
		if err != nil {
			_ = w.Store.CompleteAction(ctx, act.ID, "failed", err.Error())
			run.StepIndex = i
			run.Status = "failed"
			run.Error = err.Error()
			_ = w.Store.SavePlaybookRun(ctx, run)
			return "failed", res, err
		}
		_ = w.Store.CompleteAction(ctx, act.ID, st, res)
		run.StepIndex = i + 1
		_ = w.Store.SavePlaybookRun(ctx, run)
	}
	run.Status = "completed"
	run.Error = ""
	_ = w.Store.SavePlaybookRun(ctx, run)
	return "completed", "playbook finished", nil
}

// NeedsApproval reports connector actions that must be approved by a second operator.
func NeedsApproval(action string) bool {
	switch action {
	case "lifecycle.request", "update.delegate", "reboot", "shutdown", "wipe", "firmware.update", "power.off", "factory.reset":
		return true
	default:
		return false
	}
}

func (w *Worker) scheduleDueSync(ctx context.Context) error {
	due, err := w.Store.ConnectorsDueSync(ctx, time.Now().UTC())
	if err != nil {
		return err
	}
	for _, c := range due {
		if c.Endpoint == "" {
			continue
		}
		open, err := w.Store.OpenJobForConnector(ctx, c.OrganizationID, c.ID)
		if err != nil || open {
			continue
		}
		action := syncAction(c.Actions)
		act := &model.ActionRequest{
			OrganizationID: c.OrganizationID,
			ConnectorID:    &c.ID,
			Action:         action,
			IdempotencyKey: idgen.New("idem"),
			Status:         "queued",
			Payload:        "{}",
			ExpiresAt:      time.Now().UTC().Add(15 * time.Minute),
		}
		if err := w.Store.CreateAction(ctx, act); err != nil {
			continue
		}
		if _, err := EnqueueRemoteAction(ctx, w.Store, c.OrganizationID, act.ID, c.ID, action, "{}", "", "queued"); err != nil {
			continue
		}
	}
	return nil
}

func syncAction(actions string) string {
	var list []string
	if json.Unmarshal([]byte(actions), &list) == nil {
		for _, a := range list {
			if a == "inventory.refresh" || a == "telemetry.receive" {
				return a
			}
		}
		if len(list) > 0 && !NeedsApproval(list[0]) {
			return list[0]
		}
	}
	return "inventory.refresh"
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
