package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/zyvorai/yard/internal/idgen"
	"github.com/zyvorai/yard/internal/model"
)

func (s *Store) CreatePlaybook(ctx context.Context, p *model.Playbook) error {
	if p.ID == "" {
		p.ID = idgen.New("pb")
	}
	now := time.Now().UTC()
	p.CreatedAt, p.UpdatedAt = now, now
	_, err := s.exec(ctx, `INSERT INTO playbooks(id,organization_id,name,body,source_url,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`,
		p.ID, p.OrganizationID, p.Name, p.Body, p.SourceURL, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListPlaybooks(ctx context.Context, orgID string) ([]model.Playbook, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,name,body,source_url,created_at,updated_at FROM playbooks WHERE organization_id=? ORDER BY updated_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Playbook
	for rows.Next() {
		p, err := scanPlaybook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if out == nil {
		out = []model.Playbook{}
	}
	return out, rows.Err()
}

func (s *Store) ListSourcedPlaybooks(ctx context.Context) ([]model.Playbook, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,name,body,source_url,created_at,updated_at FROM playbooks WHERE source_url != '' ORDER BY updated_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Playbook
	for rows.Next() {
		p, err := scanPlaybook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if out == nil {
		out = []model.Playbook{}
	}
	return out, rows.Err()
}

func (s *Store) UpdatePlaybookBody(ctx context.Context, orgID, id, name, body string) error {
	res, err := s.exec(ctx, `UPDATE playbooks SET name=?, body=?, updated_at=? WHERE organization_id=? AND id=?`,
		name, body, now(), orgID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) PlaybookByID(ctx context.Context, orgID, id string) (*model.Playbook, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,name,body,source_url,created_at,updated_at FROM playbooks WHERE organization_id=? AND id=?`, orgID, id)
	p, err := scanPlaybook(row)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func scanPlaybook(sc interface{ Scan(...any) error }) (model.Playbook, error) {
	var p model.Playbook
	var created, updated string
	err := sc.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.Body, &p.SourceURL, &created, &updated)
	if err != nil {
		return p, err
	}
	p.CreatedAt, p.UpdatedAt = parseTime(created), parseTime(updated)
	return p, nil
}

func (s *Store) CreatePlaybookRun(ctx context.Context, r *model.PlaybookRun) error {
	if r.ID == "" {
		r.ID = idgen.New("pbr")
	}
	now := time.Now().UTC()
	r.CreatedAt, r.UpdatedAt = now, now
	dry := 0
	if r.DryRun {
		dry = 1
	}
	_, err := s.exec(ctx, `INSERT INTO playbook_runs(id,organization_id,playbook_id,asset_id,connector_id,status,requested_by,approved_by,step_index,dry_run,job_id,error,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.OrganizationID, r.PlaybookID, r.AssetID, r.ConnectorID, r.Status, r.RequestedBy, r.ApprovedBy, r.StepIndex, dry, r.JobID, r.Error,
		now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	return err
}

func (s *Store) PlaybookRunByID(ctx context.Context, orgID, id string) (*model.PlaybookRun, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,playbook_id,asset_id,connector_id,status,requested_by,approved_by,step_index,dry_run,job_id,error,created_at,updated_at
FROM playbook_runs WHERE organization_id=? AND id=?`, orgID, id)
	run, err := scanPlaybookRun(row)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *Store) SavePlaybookRun(ctx context.Context, r *model.PlaybookRun) error {
	r.UpdatedAt = time.Now().UTC()
	res, err := s.exec(ctx, `UPDATE playbook_runs SET status=?, approved_by=?, step_index=?, job_id=?, error=?, updated_at=? WHERE organization_id=? AND id=?`,
		r.Status, r.ApprovedBy, r.StepIndex, r.JobID, r.Error, r.UpdatedAt.Format(time.RFC3339Nano), r.OrganizationID, r.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func scanPlaybookRun(sc interface{ Scan(...any) error }) (model.PlaybookRun, error) {
	var r model.PlaybookRun
	var dry int
	var created, updated string
	err := sc.Scan(&r.ID, &r.OrganizationID, &r.PlaybookID, &r.AssetID, &r.ConnectorID, &r.Status, &r.RequestedBy, &r.ApprovedBy, &r.StepIndex, &dry, &r.JobID, &r.Error, &created, &updated)
	if err != nil {
		return r, err
	}
	r.DryRun = dry == 1
	r.CreatedAt, r.UpdatedAt = parseTime(created), parseTime(updated)
	return r, nil
}
