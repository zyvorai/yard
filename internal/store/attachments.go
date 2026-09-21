package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/zyvorai/yard/internal/idgen"
)

type Attachment struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	AssetID        string    `json:"asset_id"`
	Name           string    `json:"name"`
	ContentType    string    `json:"content_type"`
	SizeBytes      int64     `json:"size_bytes"`
	CreatedBy      string    `json:"created_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func (s *Store) CreateAttachment(ctx context.Context, a *Attachment) error {
	if a.ID == "" {
		a.ID = idgen.New("file")
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	if a.ContentType == "" {
		a.ContentType = "application/octet-stream"
	}
	_, err := s.exec(ctx, `INSERT INTO attachments(id,organization_id,asset_id,name,content_type,size_bytes,created_by,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		a.ID, a.OrganizationID, a.AssetID, a.Name, a.ContentType, a.SizeBytes, a.CreatedBy, a.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListAttachments(ctx context.Context, orgID, assetID string) ([]Attachment, error) {
	rows, err := s.query(ctx, `SELECT id,organization_id,asset_id,name,content_type,size_bytes,created_by,created_at FROM attachments WHERE organization_id=? AND asset_id=? ORDER BY created_at`, orgID, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attachment
	for rows.Next() {
		a, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	if out == nil {
		out = []Attachment{}
	}
	return out, rows.Err()
}

func (s *Store) AttachmentByID(ctx context.Context, orgID, assetID, id string) (*Attachment, error) {
	row := s.queryRow(ctx, `SELECT id,organization_id,asset_id,name,content_type,size_bytes,created_by,created_at FROM attachments WHERE organization_id=? AND asset_id=? AND id=?`, orgID, assetID, id)
	return scanAttachment(row)
}

func scanAttachment(sc interface{ Scan(...any) error }) (*Attachment, error) {
	var a Attachment
	var created string
	if err := sc.Scan(&a.ID, &a.OrganizationID, &a.AssetID, &a.Name, &a.ContentType, &a.SizeBytes, &a.CreatedBy, &created); err != nil {
		return nil, err
	}
	a.CreatedAt = parseTime(created)
	return &a, nil
}

func (s *Store) DeleteAttachment(ctx context.Context, orgID, assetID, id string) error {
	res, err := s.exec(ctx, `DELETE FROM attachments WHERE organization_id=? AND asset_id=? AND id=?`, orgID, assetID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
