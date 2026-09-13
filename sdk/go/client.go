// Package yardclient is a thin, hand-authored Go client for the Yard API
// (see /openapi.yaml at the repo root for the full route/schema reference).
// It covers the core flows — auth, assets, sites, telemetry, incidents,
// work orders, automations — deliberately not a 1:1 wrapper of every route:
// the API surface is small enough that adding a call you need is a few
// lines, following the pattern of the ones already here.
//
// Authentication: pass either a session token (from Login) or a long-lived
// human API key (create one via the console's Admin > API keys, or
// POST /api/v1/api-keys) to WithToken. Both authenticate identically.
package yardclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to one Yard server as one authenticated identity.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// New returns a Client with an unset token — call Login, or use WithToken
// for a pre-existing session token or API key.
func New(baseURL string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTPClient: http.DefaultClient}
}

// WithToken returns a copy of c authenticated as the given session token or
// API key (e.g. "yard_key_...").
func (c *Client) WithToken(token string) *Client {
	cp := *c
	cp.Token = token
	return &cp
}

// APIError is returned for any non-2xx response.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("yard: %d: %s", e.StatusCode, e.Body)
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reqBody)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	if out != nil && len(respBody) > 0 {
		return json.Unmarshal(respBody, out)
	}
	return nil
}

// --- auth ---

type Session struct {
	Token     string    `json:"token"`
	User      User      `json:"user"`
	ExpiresAt time.Time `json:"expires_at"`
}

type User struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Email          string    `json:"email"`
	DisplayName    string    `json:"display_name"`
	Role           string    `json:"role"`
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"created_at"`
}

// Login authenticates and returns a new Client carrying the session token —
// it does not mutate c.
func (c *Client) Login(ctx context.Context, email, password string) (*Client, *Session, error) {
	var sess Session
	if err := c.do(ctx, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email": email, "password": password,
	}, &sess); err != nil {
		return nil, nil, err
	}
	return c.WithToken(sess.Token), &sess, nil
}

func (c *Client) Me(ctx context.Context) (*User, error) {
	var u User
	if err := c.do(ctx, http.MethodGet, "/api/v1/auth/me", nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// --- assets ---

type Asset struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	SiteID         *string    `json:"site_id,omitempty"`
	Name           string     `json:"name"`
	ExternalRef    string     `json:"external_ref"`
	Kind           string     `json:"kind"`
	Status         string     `json:"status"`
	Health         string     `json:"health"`
	Manufacturer   string     `json:"manufacturer"`
	Model          string     `json:"model"`
	Serial         string     `json:"serial"`
	Latitude       *float64   `json:"latitude,omitempty"`
	Longitude      *float64   `json:"longitude,omitempty"`
	LastSeenAt     *time.Time `json:"last_seen_at,omitempty"`
	StaleAfterSec  int        `json:"stale_after_sec"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// ListAssets. q/kind/health are optional filters — pass "" to omit.
func (c *Client) ListAssets(ctx context.Context, q, kind, health string) ([]Asset, error) {
	v := url.Values{}
	if q != "" {
		v.Set("q", q)
	}
	if kind != "" {
		v.Set("kind", kind)
	}
	if health != "" {
		v.Set("health", health)
	}
	path := "/api/v1/assets"
	if enc := v.Encode(); enc != "" {
		path += "?" + enc
	}
	var out []Asset
	return out, c.do(ctx, http.MethodGet, path, nil, &out)
}

type AssetDetail struct {
	Asset        Asset         `json:"asset"`
	Capabilities []Capability  `json:"capabilities"`
	Observations []Observation `json:"observations"`
	WorkOrders   []WorkOrder   `json:"work_orders"`
}

func (c *Client) GetAsset(ctx context.Context, id string) (*AssetDetail, error) {
	var out AssetDetail
	return &out, c.do(ctx, http.MethodGet, "/api/v1/assets/"+url.PathEscape(id), nil, &out)
}

type Capability struct {
	ID       string   `json:"id"`
	AssetID  string   `json:"asset_id"`
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Unit     string   `json:"unit"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
	Writable bool     `json:"writable"`
}

type Observation struct {
	ID         string    `json:"id"`
	AssetID    string    `json:"asset_id"`
	Capability string    `json:"capability"`
	Value      float64   `json:"value"`
	Unit       string    `json:"unit"`
	Quality    string    `json:"quality"`
	Source     string    `json:"source"`
	ObservedAt time.Time `json:"observed_at"`
	ReceivedAt time.Time `json:"received_at"`
}

// ObservationRange lists an asset's observation history. capability, from,
// and to are optional (zero value omits that filter/bound).
func (c *Client) ObservationRange(ctx context.Context, assetID, capability string, from, to time.Time) ([]Observation, error) {
	v := url.Values{}
	if capability != "" {
		v.Set("capability", capability)
	}
	if !from.IsZero() {
		v.Set("from", from.UTC().Format(time.RFC3339))
	}
	if !to.IsZero() {
		v.Set("to", to.UTC().Format(time.RFC3339))
	}
	path := "/api/v1/assets/" + url.PathEscape(assetID) + "/observations"
	if enc := v.Encode(); enc != "" {
		path += "?" + enc
	}
	var out []Observation
	return out, c.do(ctx, http.MethodGet, path, nil, &out)
}

// --- sites ---

type Site struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	Kind           string    `json:"kind"`
	Address        string    `json:"address"`
	Latitude       *float64  `json:"latitude,omitempty"`
	Longitude      *float64  `json:"longitude,omitempty"`
	Timezone       string    `json:"timezone"`
	CreatedAt      time.Time `json:"created_at"`
}

func (c *Client) ListSites(ctx context.Context) ([]Site, error) {
	var out []Site
	return out, c.do(ctx, http.MethodGet, "/api/v1/sites", nil, &out)
}

// --- incidents ---

type Incident struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	AssetID        *string    `json:"asset_id,omitempty"`
	SiteID         *string    `json:"site_id,omitempty"`
	Title          string     `json:"title"`
	Severity       string     `json:"severity"`
	Status         string     `json:"status"`
	Owner          string     `json:"owner"`
	Summary        string     `json:"summary"`
	Resolution     string     `json:"resolution"`
	Runbook        string     `json:"runbook,omitempty"`
	OpenedAt       time.Time  `json:"opened_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

// ListIncidents. status filters (e.g. "open"); pass "" for all.
func (c *Client) ListIncidents(ctx context.Context, status string) ([]Incident, error) {
	path := "/api/v1/incidents"
	if status != "" {
		path += "?status=" + url.QueryEscape(status)
	}
	var out []Incident
	return out, c.do(ctx, http.MethodGet, path, nil, &out)
}

func (c *Client) ResolveIncident(ctx context.Context, id, resolution string) (*Incident, error) {
	var out Incident
	err := c.do(ctx, http.MethodPatch, "/api/v1/incidents/"+url.PathEscape(id)+"/resolve", map[string]string{
		"status": "resolved", "resolution": resolution,
	}, &out)
	return &out, err
}

// --- work orders ---

type WorkOrder struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	AssetID        *string    `json:"asset_id,omitempty"`
	SiteID         *string    `json:"site_id,omitempty"`
	IncidentID     *string    `json:"incident_id,omitempty"`
	Title          string     `json:"title"`
	Kind           string     `json:"kind"`
	Priority       string     `json:"priority"`
	Status         string     `json:"status"`
	Assignee       string     `json:"assignee"`
	Notes          string     `json:"notes"`
	DueAt          *time.Time `json:"due_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (c *Client) ListWorkOrders(ctx context.Context) ([]WorkOrder, error) {
	var out []WorkOrder
	return out, c.do(ctx, http.MethodGet, "/api/v1/work-orders", nil, &out)
}

// --- automations ---

type Automation struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	Enabled        bool      `json:"enabled"`
	TriggerKind    string    `json:"trigger_kind"`
	Capability     string    `json:"capability"`
	Operator       string    `json:"operator"`
	Threshold      float64   `json:"threshold"`
	Action         string    `json:"action"`
	Config         string    `json:"config"`
	CreatedAt      time.Time `json:"created_at"`
}

func (c *Client) ListAutomations(ctx context.Context) ([]Automation, error) {
	var out []Automation
	return out, c.do(ctx, http.MethodGet, "/api/v1/automations", nil, &out)
}

func (c *Client) CreateAutomation(ctx context.Context, a Automation) (*Automation, error) {
	var out Automation
	return &out, c.do(ctx, http.MethodPost, "/api/v1/automations", a, &out)
}

// --- connector-authenticated ingest (use a connector token, not a user session/API key) ---

type IngestObservation struct {
	AssetExternalRef string    `json:"asset_external_ref"`
	Capability       string    `json:"capability"`
	Value            float64   `json:"value"`
	Unit             string    `json:"unit"`
	Source           string    `json:"source,omitempty"`
	ObservedAt       time.Time `json:"observed_at,omitempty"`
	DedupeKey        string    `json:"dedupe_key,omitempty"`
}

func (c *Client) IngestObservations(ctx context.Context, obs []IngestObservation) error {
	return c.do(ctx, http.MethodPost, "/api/v1/ingest/observations", obs, nil)
}
