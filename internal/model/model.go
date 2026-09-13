package model

import "time"

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Email          string    `json:"email"`
	DisplayName    string    `json:"display_name"`
	Role           string    `json:"role"`
	Active         bool      `json:"active"`
	PasswordHash   string    `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
}

// APIKey is a long-lived, human-issued credential (as opposed to a
// Connector's machine-ingest token): self-service, scoped to one user,
// same hash-at-rest + one-time-reveal + short hint shape as connector
// tokens.
type APIKey struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	UserID         string     `json:"user_id"`
	Name           string     `json:"name"`
	TokenHash      string     `json:"-"`
	TokenHint      string     `json:"token_hint"`
	CreatedAt      time.Time  `json:"created_at"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
}

type Session struct {
	Token     string    `json:"token"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

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
	Metadata       string     `json:"metadata"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
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
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	AssetID        string    `json:"asset_id"`
	Capability     string    `json:"capability"`
	Value          float64   `json:"value"`
	Unit           string    `json:"unit"`
	Quality        string    `json:"quality"`
	Source         string    `json:"source"`
	ObservedAt     time.Time `json:"observed_at"`
	ReceivedAt     time.Time `json:"received_at"`
	DedupeKey      string    `json:"dedupe_key,omitempty"`
}

type Event struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	AssetID        *string   `json:"asset_id,omitempty"`
	SiteID         *string   `json:"site_id,omitempty"`
	Kind           string    `json:"kind"`
	Severity       string    `json:"severity"`
	Title          string    `json:"title"`
	Body           string    `json:"body"`
	DedupeKey      string    `json:"dedupe_key,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

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

// SeverityPolicy maps automation/capability matches to severity + runbook text.
type SeverityPolicy struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	MatchKind      string    `json:"match_kind"`  // capability | automation | default
	MatchValue     string    `json:"match_value"` // capability name, automation name, or ""
	Severity       string    `json:"severity"`    // info | warning | critical
	Runbook        string    `json:"runbook"`
	Priority       int       `json:"priority"`
	CreatedAt      time.Time `json:"created_at"`
}

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

type Connector struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	Name           string     `json:"name"`
	Kind           string     `json:"kind"`
	Status         string     `json:"status"`
	Endpoint       string     `json:"endpoint"`
	TokenHint      string     `json:"token_hint"`
	TokenHash      string     `json:"-"`
	Actions        string     `json:"actions"`
	Config         string     `json:"config"`
	LastSyncAt     *time.Time `json:"last_sync_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type ActionRequest struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	AssetID        *string    `json:"asset_id,omitempty"`
	ConnectorID    *string    `json:"connector_id,omitempty"`
	Action         string     `json:"action"`
	IdempotencyKey string     `json:"idempotency_key"`
	Status         string     `json:"status"`
	Payload        string     `json:"payload"`
	Result         string     `json:"result"`
	ExpiresAt      time.Time  `json:"expires_at"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

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

type AuditEntry struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Actor          string    `json:"actor"`
	Action         string    `json:"action"`
	Object         string    `json:"object"`
	Detail         string    `json:"detail"`
	CreatedAt      time.Time `json:"created_at"`
}

type Overview struct {
	AssetsTotal      int            `json:"assets_total"`
	AssetsHealthy    int            `json:"assets_healthy"`
	AssetsDegraded   int            `json:"assets_degraded"`
	AssetsCritical   int            `json:"assets_critical"`
	AssetsStale      int            `json:"assets_stale"`
	OpenIncidents    int            `json:"open_incidents"`
	OpenWorkOrders   int            `json:"open_work_orders"`
	ActiveConnectors int            `json:"active_connectors"`
	RecentEvents     []Event        `json:"recent_events"`
	RecentActivity   []AuditEntry   `json:"recent_activity"`
	HealthByKind     map[string]int `json:"health_by_kind"`
}

type TelemetryPoint struct {
	AssetID    string    `json:"asset_id"`
	AssetName  string    `json:"asset_name"`
	Capability string    `json:"capability"`
	Value      float64   `json:"value"`
	Unit       string    `json:"unit"`
	Quality    string    `json:"quality"`
	Source     string    `json:"source"`
	ObservedAt time.Time `json:"observed_at"`
	ReceivedAt time.Time `json:"received_at"`
	Fresh      bool      `json:"fresh"`
}

type IngestObservation struct {
	AssetExternalRef string    `json:"asset_external_ref"`
	AssetID          string    `json:"asset_id"`
	Capability       string    `json:"capability"`
	Value            float64   `json:"value"`
	Unit             string    `json:"unit"`
	Quality          string    `json:"quality"`
	Source           string    `json:"source"`
	ObservedAt       time.Time `json:"observed_at"`
	DedupeKey        string    `json:"dedupe_key"`
	Latitude         *float64  `json:"latitude"`
	Longitude        *float64  `json:"longitude"`
}

type IngestInventory struct {
	ExternalRef  string   `json:"external_ref"`
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model"`
	Serial       string   `json:"serial"`
	SiteName     string   `json:"site_name"`
	Capabilities []string `json:"capabilities"`
	Latitude     *float64 `json:"latitude"`
	Longitude    *float64 `json:"longitude"`
	Metadata     string   `json:"metadata"`
}
