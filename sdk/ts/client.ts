/**
 * A thin, hand-authored TypeScript client for the Yard API (see
 * /openapi.yaml at the repo root for the full route/schema reference).
 * Framework-free — plain fetch, usable from Node 18+ or a browser.
 *
 * Covers the core flows (auth, assets, sites, telemetry, incidents, work
 * orders, automations) rather than every route: the API surface is small
 * enough that adding a call you need is a few lines, following the pattern
 * of the ones already here.
 *
 * Authentication: pass either a session token (from login()) or a
 * long-lived human API key (create one via the console's Admin > API keys,
 * or POST /api/v1/api-keys) to the constructor or withToken(). Both
 * authenticate identically.
 */

export class YardAPIError extends Error {
  constructor(public status: number, public body: string) {
    super(`yard: ${status}: ${body}`);
    this.name = "YardAPIError";
  }
}

export interface Session {
  token: string;
  user: User;
  expires_at: string;
}

export interface User {
  id: string;
  organization_id: string;
  email: string;
  display_name: string;
  role: string;
  active: boolean;
  created_at: string;
}

export interface Asset {
  id: string;
  organization_id: string;
  site_id?: string;
  name: string;
  external_ref: string;
  kind: string;
  status: string;
  health: string;
  manufacturer: string;
  model: string;
  serial: string;
  latitude?: number;
  longitude?: number;
  last_seen_at?: string;
  stale_after_sec: number;
  created_at: string;
  updated_at: string;
}

export interface Capability {
  id: string;
  asset_id: string;
  name: string;
  kind: string;
  unit: string;
  min?: number;
  max?: number;
  writable: boolean;
}

export interface Observation {
  id: string;
  asset_id: string;
  capability: string;
  value: number;
  unit: string;
  quality: string;
  source: string;
  observed_at: string;
  received_at: string;
}

export interface AssetDetail {
  asset: Asset;
  capabilities: Capability[];
  observations: Observation[];
  work_orders: WorkOrder[];
}

export interface Site {
  id: string;
  organization_id: string;
  name: string;
  kind: string;
  address: string;
  latitude?: number;
  longitude?: number;
  timezone: string;
  created_at: string;
}

export interface Incident {
  id: string;
  organization_id: string;
  asset_id?: string;
  site_id?: string;
  title: string;
  severity: string;
  status: string;
  owner: string;
  summary: string;
  resolution: string;
  runbook?: string;
  opened_at: string;
  resolved_at?: string;
}

export interface WorkOrder {
  id: string;
  organization_id: string;
  asset_id?: string;
  site_id?: string;
  incident_id?: string;
  title: string;
  kind: string;
  priority: string;
  status: string;
  assignee: string;
  notes: string;
  due_at?: string;
  created_at: string;
  updated_at: string;
}

export interface Automation {
  id?: string;
  organization_id?: string;
  name: string;
  enabled: boolean;
  trigger_kind: string;
  capability: string;
  operator: string;
  threshold: number;
  action: string;
  config?: string;
  created_at?: string;
}

export interface IngestObservation {
  asset_external_ref: string;
  capability: string;
  value: number;
  unit?: string;
  source?: string;
  observed_at?: string;
  dedupe_key?: string;
}

export class YardClient {
  constructor(private baseURL: string, private token: string = "") {
    this.baseURL = baseURL.replace(/\/+$/, "");
  }

  /** Returns a new client authenticated as the given session token or API key. */
  withToken(token: string): YardClient {
    return new YardClient(this.baseURL, token);
  }

  private async request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const headers: Record<string, string> = {};
    if (body !== undefined) headers["Content-Type"] = "application/json";
    if (this.token) headers["Authorization"] = `Bearer ${this.token}`;
    const resp = await fetch(`${this.baseURL}${path}`, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
    const text = await resp.text();
    if (!resp.ok) throw new YardAPIError(resp.status, text);
    return text ? (JSON.parse(text) as T) : (undefined as T);
  }

  // --- auth ---

  /** Logs in and returns a new client carrying the session token, plus the session itself. */
  async login(email: string, password: string): Promise<{ client: YardClient; session: Session }> {
    const session = await this.request<Session>("POST", "/api/v1/auth/login", { email, password });
    return { client: this.withToken(session.token), session };
  }

  me(): Promise<User> {
    return this.request("GET", "/api/v1/auth/me");
  }

  // --- assets ---

  listAssets(filter: { q?: string; kind?: string; health?: string } = {}): Promise<Asset[]> {
    const params = new URLSearchParams();
    for (const [k, v] of Object.entries(filter)) if (v) params.set(k, v);
    const qs = params.toString();
    return this.request("GET", `/api/v1/assets${qs ? `?${qs}` : ""}`);
  }

  getAsset(id: string): Promise<AssetDetail> {
    return this.request("GET", `/api/v1/assets/${encodeURIComponent(id)}`);
  }

  /** Observation history for one asset. capability/from/to are optional (RFC3339 for from/to). */
  observationRange(
    assetId: string,
    opts: { capability?: string; from?: string; to?: string } = {},
  ): Promise<Observation[]> {
    const params = new URLSearchParams();
    for (const [k, v] of Object.entries(opts)) if (v) params.set(k, v);
    const qs = params.toString();
    return this.request("GET", `/api/v1/assets/${encodeURIComponent(assetId)}/observations${qs ? `?${qs}` : ""}`);
  }

  // --- sites ---

  listSites(): Promise<Site[]> {
    return this.request("GET", "/api/v1/sites");
  }

  // --- incidents ---

  listIncidents(status?: string): Promise<Incident[]> {
    return this.request("GET", `/api/v1/incidents${status ? `?status=${encodeURIComponent(status)}` : ""}`);
  }

  resolveIncident(id: string, resolution: string): Promise<Incident> {
    return this.request("PATCH", `/api/v1/incidents/${encodeURIComponent(id)}/resolve`, {
      status: "resolved",
      resolution,
    });
  }

  // --- work orders ---

  listWorkOrders(): Promise<WorkOrder[]> {
    return this.request("GET", "/api/v1/work-orders");
  }

  // --- automations ---

  listAutomations(): Promise<Automation[]> {
    return this.request("GET", "/api/v1/automations");
  }

  createAutomation(a: Automation): Promise<Automation> {
    return this.request("POST", "/api/v1/automations", a);
  }

  // --- connector-authenticated ingest (use a connector token, not a user session/API key) ---

  ingestObservations(observations: IngestObservation[]): Promise<void> {
    return this.request("POST", "/api/v1/ingest/observations", observations);
  }
}
