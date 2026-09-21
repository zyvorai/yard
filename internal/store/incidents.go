package store

import (
	"context"
	"sort"
	"time"

	"github.com/zyvorai/yard/internal/idgen"
	"github.com/zyvorai/yard/internal/model"
)

func (s *Store) applyIncidentDefaults(ctx context.Context, inc *model.Incident) error {
	if inc.AckDueAt == nil || inc.ResolveDueAt == nil {
		ackMin, resolveMin, err := s.SLAMinutes(ctx, inc.OrganizationID)
		if err != nil {
			return err
		}
		if inc.AckDueAt == nil {
			t := inc.OpenedAt.Add(time.Duration(ackMin) * time.Minute)
			inc.AckDueAt = &t
		}
		if inc.ResolveDueAt == nil {
			t := inc.OpenedAt.Add(time.Duration(resolveMin) * time.Minute)
			inc.ResolveDueAt = &t
		}
	}
	if inc.Owner == "" {
		u, err := s.OnCallAt(ctx, inc.OrganizationID, inc.OpenedAt)
		if err != nil {
			return err
		}
		if u != nil {
			inc.Owner = u.DisplayName
			if inc.Owner == "" {
				inc.Owner = u.Email
			}
		}
	}
	return nil
}

func markIncidentSLA(inc *model.Incident) {
	now := time.Now().UTC()
	if inc.AckDueAt != nil {
		if inc.AckedAt != nil {
			inc.AckBreached = inc.AckedAt.After(*inc.AckDueAt)
		} else if inc.Status != "resolved" {
			inc.AckBreached = now.After(*inc.AckDueAt)
		}
	}
	if inc.ResolveDueAt != nil {
		if inc.ResolvedAt != nil {
			inc.ResolveBreached = inc.ResolvedAt.After(*inc.ResolveDueAt)
		} else if inc.Status != "resolved" {
			inc.ResolveBreached = now.After(*inc.ResolveDueAt)
		}
	}
}

func (s *Store) SLAMinutes(ctx context.Context, orgID string) (ack, resolve int, err error) {
	err = s.queryRow(ctx, `SELECT ack_minutes, resolve_minutes FROM organizations WHERE id=?`, orgID).Scan(&ack, &resolve)
	return ack, resolve, err
}

func (s *Store) SetSLAMinutes(ctx context.Context, orgID string, ack, resolve int) error {
	_, err := s.exec(ctx, `UPDATE organizations SET ack_minutes=?, resolve_minutes=? WHERE id=?`, ack, resolve, orgID)
	return err
}

func (s *Store) ArmIncidentFlap(ctx context.Context, orgID, title, assetID string) error {
	_, err := s.exec(ctx, `UPDATE incidents SET flap_armed=1 WHERE organization_id=? AND title=? AND asset_id=? AND status IN ('open','ack')`, orgID, title, assetID)
	return err
}

func (s *Store) NoteIncidentFlap(ctx context.Context, orgID, id string) (bool, error) {
	res, err := s.exec(ctx, `UPDATE incidents SET flap_count=flap_count+1, flap_armed=0 WHERE organization_id=? AND id=? AND flap_armed=1`, orgID, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (s *Store) CreateOnCall(ctx context.Context, orgID string, row *model.OnCall) error {
	if row.ID == "" {
		row.ID = idgen.New("oc")
	}
	_, err := s.exec(ctx, `INSERT INTO oncall(id,organization_id,user_id,starts_at,ends_at) VALUES(?,?,?,?,?)`,
		row.ID, orgID, row.UserID, row.StartsAt.Format(time.RFC3339Nano), row.EndsAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListOnCall(ctx context.Context, orgID string) ([]model.OnCall, error) {
	rows, err := s.query(ctx, `SELECT id,user_id,starts_at,ends_at FROM oncall WHERE organization_id=? ORDER BY starts_at`, orgID)
	if err != nil {
		return nil, err
	}
	var raw []model.OnCall
	var stamps [][2]string
	for rows.Next() {
		var row model.OnCall
		var start, end string
		if err := rows.Scan(&row.ID, &row.UserID, &start, &end); err != nil {
			rows.Close()
			return nil, err
		}
		raw = append(raw, row)
		stamps = append(stamps, [2]string{start, end})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	out := make([]model.OnCall, 0, len(raw))
	for i, row := range raw {
		row.StartsAt = parseTime(stamps[i][0])
		row.EndsAt = parseTime(stamps[i][1])
		if u, err := s.UserByID(ctx, row.UserID); err == nil {
			row.DisplayName = u.DisplayName
		}
		out = append(out, row)
	}
	if out == nil {
		out = []model.OnCall{}
	}
	return out, nil
}

func (s *Store) OnCallAt(ctx context.Context, orgID string, at time.Time) (*model.User, error) {
	list, err := s.ListOnCall(ctx, orgID)
	if err != nil {
		return nil, err
	}
	at = at.UTC()
	var userID string
	var start time.Time
	for _, row := range list {
		if row.StartsAt.After(at) || !row.EndsAt.After(at) {
			continue
		}
		if userID == "" || row.StartsAt.After(start) {
			userID = row.UserID
			start = row.StartsAt
		}
	}
	if userID == "" {
		return nil, nil
	}
	u, err := s.UserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.OrganizationID != orgID {
		return nil, nil
	}
	return u, nil
}

func (s *Store) IncidentTimeline(ctx context.Context, orgID, id string) ([]model.IncidentNote, error) {
	inc, err := s.GetIncident(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	notes := []model.IncidentNote{{At: inc.OpenedAt, Kind: "opened", Title: inc.Title}}
	rows, err := s.query(ctx, `SELECT title, created_at FROM events WHERE organization_id=? AND kind='incident.flap' AND body=? ORDER BY created_at`, orgID, id)
	if err != nil {
		return nil, err
	}
	var flaps []model.IncidentNote
	for rows.Next() {
		var title, created string
		if err := rows.Scan(&title, &created); err != nil {
			rows.Close()
			return nil, err
		}
		flaps = append(flaps, model.IncidentNote{At: parseTime(created), Kind: "flap", Title: title})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	notes = append(notes, flaps...)
	if inc.AckedAt != nil {
		notes = append(notes, model.IncidentNote{At: *inc.AckedAt, Kind: "ack", Title: "Acknowledged"})
	}
	children, err := s.query(ctx, `SELECT title, opened_at FROM incidents WHERE organization_id=? AND parent_id=? ORDER BY opened_at`, orgID, id)
	if err != nil {
		return nil, err
	}
	for children.Next() {
		var title, opened string
		if err := children.Scan(&title, &opened); err != nil {
			children.Close()
			return nil, err
		}
		notes = append(notes, model.IncidentNote{At: parseTime(opened), Kind: "child", Title: title})
	}
	if err := children.Err(); err != nil {
		children.Close()
		return nil, err
	}
	children.Close()
	if inc.ResolvedAt != nil {
		notes = append(notes, model.IncidentNote{At: *inc.ResolvedAt, Kind: "resolved", Title: "Resolved"})
	}
	sort.Slice(notes, func(i, j int) bool {
		if notes[i].At.Equal(notes[j].At) {
			return noteRank(notes[i].Kind) < noteRank(notes[j].Kind)
		}
		return notes[i].At.Before(notes[j].At)
	})
	return notes, nil
}

func noteRank(kind string) int {
	switch kind {
	case "opened":
		return 0
	case "flap":
		return 1
	case "ack":
		return 2
	case "child":
		return 3
	default:
		return 4
	}
}
