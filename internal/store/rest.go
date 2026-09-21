package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/zyvorai/yard/internal/idgen"
	"github.com/zyvorai/yard/internal/model"
)

type InstallRow struct {
	ID            string    `json:"id"`
	AssetID       string    `json:"asset_id"`
	ParentAssetID string    `json:"parent_asset_id,omitempty"`
	Note          string    `json:"note"`
	InstalledAt   time.Time `json:"installed_at"`
}

type BOMLine struct {
	ID         string  `json:"id"`
	AssetID    string  `json:"asset_id"`
	PartNumber string  `json:"part_number"`
	Name       string  `json:"name"`
	Quantity   float64 `json:"quantity"`
	Unit       string  `json:"unit"`
}

type Catalog struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	TemplateIDs []string  `json:"template_ids"`
	CreatedAt   time.Time `json:"created_at"`
}

type TimeEntry struct {
	ID          string    `json:"id"`
	WorkOrderID string    `json:"work_order_id"`
	Actor       string    `json:"actor"`
	Minutes     int       `json:"minutes"`
	Note        string    `json:"note"`
	CreatedAt   time.Time `json:"created_at"`
}

type SavedView struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Query     string    `json:"query"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) AssetByNFC(ctx context.Context, orgID, nfc string) (*model.Asset, error) {
	return s.getAsset(ctx, `SELECT `+assetSelect+` FROM assets WHERE organization_id=? AND nfc_id=? AND (deleted_at IS NULL OR deleted_at='')`, orgID, nfc)
}

func (s *Store) SetAssetNFC(ctx context.Context, orgID, id, nfc string) error {
	_, err := s.exec(ctx, `UPDATE assets SET nfc_id=?, updated_at=? WHERE organization_id=? AND id=?`, nfc, now(), orgID, id)
	return err
}

func (s *Store) SetAssetWarranty(ctx context.Context, orgID, id string, purchased, warranty *time.Time, cents, months int) error {
	_, err := s.exec(ctx, `UPDATE assets SET purchased_at=?, warranty_expires_at=?, purchase_cents=?, useful_life_months=?, updated_at=? WHERE organization_id=? AND id=?`,
		ts(purchased), ts(warranty), cents, months, now(), orgID, id)
	return err
}

func (s *Store) RecordInstall(ctx context.Context, orgID, assetID string, parent *string, note string) error {
	parentID := ""
	if parent != nil {
		parentID = *parent
	}
	_, err := s.exec(ctx, `INSERT INTO install_history(id,organization_id,asset_id,parent_asset_id,note,installed_at) VALUES(?,?,?,?,?,?)`,
		idgen.New("ih"), orgID, assetID, nullEmpty(parentID), note, now())
	return err
}

func nullEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *Store) ListInstallHistory(ctx context.Context, orgID, assetID string) ([]InstallRow, error) {
	rows, err := s.query(ctx, `SELECT id,asset_id,parent_asset_id,note,installed_at FROM install_history WHERE organization_id=? AND asset_id=? ORDER BY installed_at DESC`, orgID, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InstallRow
	for rows.Next() {
		var row InstallRow
		var parent sql.NullString
		var at string
		if err := rows.Scan(&row.ID, &row.AssetID, &parent, &row.Note, &at); err != nil {
			return nil, err
		}
		if parent.Valid {
			row.ParentAssetID = parent.String
		}
		row.InstalledAt = parseTime(at)
		out = append(out, row)
	}
	if out == nil {
		out = []InstallRow{}
	}
	return out, rows.Err()
}

func (s *Store) AddBOMLine(ctx context.Context, orgID string, line *BOMLine) error {
	if line.ID == "" {
		line.ID = idgen.New("bom")
	}
	if line.Unit == "" {
		line.Unit = "ea"
	}
	_, err := s.exec(ctx, `INSERT INTO asset_bom(id,organization_id,asset_id,part_number,name,quantity,unit) VALUES(?,?,?,?,?,?,?)`,
		line.ID, orgID, line.AssetID, line.PartNumber, line.Name, line.Quantity, line.Unit)
	return err
}

func (s *Store) ListBOM(ctx context.Context, orgID, assetID string) ([]BOMLine, error) {
	rows, err := s.query(ctx, `SELECT id,asset_id,part_number,name,quantity,unit FROM asset_bom WHERE organization_id=? AND asset_id=? ORDER BY part_number`, orgID, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BOMLine
	for rows.Next() {
		var line BOMLine
		if err := rows.Scan(&line.ID, &line.AssetID, &line.PartNumber, &line.Name, &line.Quantity, &line.Unit); err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	if out == nil {
		out = []BOMLine{}
	}
	return out, rows.Err()
}

func (s *Store) CreateCatalog(ctx context.Context, orgID string, c *Catalog) error {
	if c.ID == "" {
		c.ID = idgen.New("cat")
	}
	c.CreatedAt = time.Now().UTC()
	if c.TemplateIDs == nil {
		c.TemplateIDs = []string{}
	}
	raw, _ := json.Marshal(c.TemplateIDs)
	_, err := s.exec(ctx, `INSERT INTO catalogs(id,organization_id,name,description,template_ids,created_at) VALUES(?,?,?,?,?,?)`,
		c.ID, orgID, c.Name, c.Description, string(raw), c.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListCatalogs(ctx context.Context, orgID string) ([]Catalog, error) {
	rows, err := s.query(ctx, `SELECT id,name,description,template_ids,created_at FROM catalogs WHERE organization_id=? ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Catalog
	for rows.Next() {
		var c Catalog
		var raw, created string
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &raw, &created); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(raw), &c.TemplateIDs)
		if c.TemplateIDs == nil {
			c.TemplateIDs = []string{}
		}
		c.CreatedAt = parseTime(created)
		out = append(out, c)
	}
	if out == nil {
		out = []Catalog{}
	}
	return out, rows.Err()
}

func (s *Store) SetWorkOrderAssets(ctx context.Context, woID string, assetIDs []string) error {
	if _, err := s.exec(ctx, `DELETE FROM work_order_assets WHERE work_order_id=?`, woID); err != nil {
		return err
	}
	for _, id := range assetIDs {
		if id == "" {
			continue
		}
		if _, err := s.exec(ctx, `INSERT INTO work_order_assets(work_order_id,asset_id) VALUES(?,?)`, woID, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListWorkOrderAssets(ctx context.Context, woID string) ([]string, error) {
	rows, err := s.query(ctx, `SELECT asset_id FROM work_order_assets WHERE work_order_id=?`, woID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	if out == nil {
		out = []string{}
	}
	return out, rows.Err()
}

func (s *Store) AddTimeEntry(ctx context.Context, orgID string, e *TimeEntry) error {
	if e.ID == "" {
		e.ID = idgen.New("te")
	}
	e.CreatedAt = time.Now().UTC()
	_, err := s.exec(ctx, `INSERT INTO time_entries(id,organization_id,work_order_id,actor,minutes,note,created_at) VALUES(?,?,?,?,?,?,?)`,
		e.ID, orgID, e.WorkOrderID, e.Actor, e.Minutes, e.Note, e.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListTimeEntries(ctx context.Context, orgID, woID string) ([]TimeEntry, error) {
	rows, err := s.query(ctx, `SELECT id,work_order_id,actor,minutes,note,created_at FROM time_entries WHERE organization_id=? AND work_order_id=? ORDER BY created_at`, orgID, woID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TimeEntry
	for rows.Next() {
		var e TimeEntry
		var created string
		if err := rows.Scan(&e.ID, &e.WorkOrderID, &e.Actor, &e.Minutes, &e.Note, &created); err != nil {
			return nil, err
		}
		e.CreatedAt = parseTime(created)
		out = append(out, e)
	}
	if out == nil {
		out = []TimeEntry{}
	}
	return out, rows.Err()
}

func (s *Store) CreateSavedView(ctx context.Context, orgID string, v *SavedView) error {
	if v.ID == "" {
		v.ID = idgen.New("view")
	}
	v.CreatedAt = time.Now().UTC()
	if v.Query == "" {
		v.Query = "{}"
	}
	_, err := s.exec(ctx, `INSERT INTO saved_views(id,organization_id,user_id,name,kind,query,created_at) VALUES(?,?,?,?,?,?,?)`,
		v.ID, orgID, v.UserID, v.Name, v.Kind, v.Query, v.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListSavedViews(ctx context.Context, orgID, userID string) ([]SavedView, error) {
	rows, err := s.query(ctx, `SELECT id,user_id,name,kind,query,created_at FROM saved_views WHERE organization_id=? AND user_id=? ORDER BY name`, orgID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SavedView
	for rows.Next() {
		var v SavedView
		var created string
		if err := rows.Scan(&v.ID, &v.UserID, &v.Name, &v.Kind, &v.Query, &created); err != nil {
			return nil, err
		}
		v.CreatedAt = parseTime(created)
		out = append(out, v)
	}
	if out == nil {
		out = []SavedView{}
	}
	return out, rows.Err()
}

func (s *Store) EnsureOrgMember(ctx context.Context, orgID, userID, role string) error {
	_, err := s.exec(ctx, `INSERT INTO org_members(organization_id,user_id,role) VALUES(?,?,?)
ON CONFLICT(organization_id,user_id) DO UPDATE SET role=excluded.role`, orgID, userID, role)
	return err
}

func (s *Store) ListUserOrgs(ctx context.Context, userID string) ([]model.Organization, error) {
	rows, err := s.query(ctx, `SELECT o.id,o.name,o.slug,o.created_at FROM organizations o
JOIN org_members m ON m.organization_id=o.id WHERE m.user_id=?
UNION
SELECT o.id,o.name,o.slug,o.created_at FROM organizations o
JOIN users u ON u.organization_id=o.id WHERE u.id=?
ORDER BY name`, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	var out []model.Organization
	for rows.Next() {
		var o model.Organization
		var created string
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &created); err != nil {
			return nil, err
		}
		if seen[o.ID] {
			continue
		}
		seen[o.ID] = true
		o.CreatedAt = parseTime(created)
		out = append(out, o)
	}
	if out == nil {
		out = []model.Organization{}
	}
	return out, rows.Err()
}

func (s *Store) SetUserOrg(ctx context.Context, userID, orgID string) error {
	_, err := s.exec(ctx, `UPDATE users SET organization_id=? WHERE id=?`, orgID, userID)
	return err
}

func (s *Store) SetUserWidgets(ctx context.Context, userID, widgets string) error {
	_, err := s.exec(ctx, `UPDATE users SET widgets=? WHERE id=?`, widgets, userID)
	return err
}

func (s *Store) UserWidgets(ctx context.Context, userID string) (string, error) {
	var w string
	err := s.queryRow(ctx, `SELECT widgets FROM users WHERE id=?`, userID).Scan(&w)
	return w, err
}

func (s *Store) SetUserLocale(ctx context.Context, userID, locale string) error {
	_, err := s.exec(ctx, `UPDATE users SET locale=? WHERE id=?`, locale, userID)
	return err
}

func (s *Store) UserLocale(ctx context.Context, userID string) (string, error) {
	var locale string
	err := s.queryRow(ctx, `SELECT locale FROM users WHERE id=?`, userID).Scan(&locale)
	return locale, err
}

func (s *Store) ListAssetsInBBox(ctx context.Context, orgID string, minLng, minLat, maxLng, maxLat float64) ([]model.Asset, error) {
	rows, err := s.query(ctx, `SELECT `+assetSelect+` FROM assets WHERE organization_id=? AND (deleted_at IS NULL OR deleted_at='')
AND latitude IS NOT NULL AND longitude IS NOT NULL
AND longitude>=? AND longitude<=? AND latitude>=? AND latitude<=?
ORDER BY name`, orgID, minLng, maxLng, minLat, maxLat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Asset
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	if out == nil {
		out = []model.Asset{}
	}
	return out, rows.Err()
}

func ParseBBox(raw string) (minLng, minLat, maxLng, maxLat float64, ok bool) {
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return 0, 0, 0, 0, false
	}
	vals := make([]float64, 4)
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return 0, 0, 0, 0, false
		}
		vals[i] = f
	}
	return vals[0], vals[1], vals[2], vals[3], true
}
