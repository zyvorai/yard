package store

import (
	"context"
	"strings"

	"github.com/zyvorai/yard/internal/idgen"
)

type Role struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	CanWrite       bool   `json:"can_write"`
}

func (s *Store) CreateRole(ctx context.Context, role *Role) error {
	if role.ID == "" {
		role.ID = idgen.New("role")
	}
	n := 0
	if role.CanWrite {
		n = 1
	}
	_, err := s.exec(ctx, `INSERT INTO roles(id,organization_id,name,can_write) VALUES(?,?,?,?)`, role.ID, role.OrganizationID, role.Name, n)
	return err
}

func (s *Store) RoleCanWrite(ctx context.Context, orgID, name string) (bool, error) {
	var n int
	err := s.queryRow(ctx, `SELECT can_write FROM roles WHERE organization_id=? AND name=?`, orgID, name).Scan(&n)
	if err != nil {
		return false, nil
	}
	return n == 1, nil
}

type Shift struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	UserID         string `json:"user_id"`
	StartsAt       string `json:"starts_at"`
	EndsAt         string `json:"ends_at"`
}

func (s *Store) CreateShift(ctx context.Context, sh *Shift) error {
	if sh.ID == "" {
		sh.ID = idgen.New("shift")
	}
	_, err := s.exec(ctx, `INSERT INTO shifts(id,organization_id,user_id,starts_at,ends_at) VALUES(?,?,?,?,?)`,
		sh.ID, sh.OrganizationID, sh.UserID, sh.StartsAt, sh.EndsAt)
	return err
}

func (s *Store) ListShifts(ctx context.Context, orgID string) ([]Shift, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,user_id,starts_at,ends_at FROM shifts WHERE organization_id=? ORDER BY starts_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Shift
	for rows.Next() {
		var sh Shift
		if err := rows.Scan(&sh.ID, &sh.OrganizationID, &sh.UserID, &sh.StartsAt, &sh.EndsAt); err != nil {
			return nil, err
		}
		out = append(out, sh)
	}
	if out == nil {
		out = []Shift{}
	}
	return out, rows.Err()
}

func (s *Store) SetUserSkills(ctx context.Context, orgID, userID, skills string) error {
	_, err := s.exec(ctx, `UPDATE users SET skills=? WHERE organization_id=? AND id=?`, skills, orgID, userID)
	return err
}

func (s *Store) SetWorkOrderSkill(ctx context.Context, orgID, id, skill string) error {
	_, err := s.exec(ctx, `UPDATE work_orders SET required_skill=? WHERE organization_id=? AND id=?`, skill, orgID, id)
	return err
}

func (s *Store) WorkOrderSkill(ctx context.Context, orgID, id string) (string, error) {
	var skill string
	err := s.queryRow(ctx, `SELECT required_skill FROM work_orders WHERE organization_id=? AND id=?`, orgID, id).Scan(&skill)
	return skill, err
}

func (s *Store) AssigneeHasSkill(ctx context.Context, orgID, assignee, skill string) (bool, error) {
	var skills string
	err := s.queryRow(ctx, `SELECT skills FROM users WHERE organization_id=? AND (id=? OR email=? OR display_name=?)`, orgID, assignee, assignee, assignee).Scan(&skills)
	if err != nil {
		return false, err
	}
	for _, part := range strings.Split(skills, ",") {
		if strings.TrimSpace(part) == skill {
			return true, nil
		}
	}
	return false, nil
}
