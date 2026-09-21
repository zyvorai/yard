const TOKEN_KEY = "yard.token";

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || "";
}

export function setToken(t: string) {
  localStorage.setItem(TOKEN_KEY, t);
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  const tok = getToken();
  if (tok) headers.set("Authorization", `Bearer ${tok}`);
  const res = await fetch(path, { ...init, headers });
  if (res.status === 401) {
    clearToken();
    if (!path.includes("/auth/login")) window.location.href = "/login";
    throw new Error("unauthorized");
  }
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  return res.json() as Promise<T>;
}

/** Authenticated download (CSV/JSON export). */
export async function downloadAuth(path: string, filename: string) {
  const headers = new Headers({ Accept: "*/*" });
  const tok = getToken();
  if (tok) headers.set("Authorization", `Bearer ${tok}`);
  const res = await fetch(path, { headers });
  if (!res.ok) throw new Error(await res.text() || res.statusText);
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

export type Asset = {
  id: string;
  name: string;
  external_ref: string;
  kind: string;
  status: string;
  health: string;
  manufacturer: string;
  model: string;
  serial: string;
  site_id?: string;
  parent_asset_id?: string;
  location_id?: string;
  template_id?: string;
  latitude?: number;
  longitude?: number;
  last_seen_at?: string;
  stale_after_sec: number;
};

export type Site = {
  id: string;
  name: string;
  kind: string;
  address: string;
  latitude?: number;
  longitude?: number;
};

export type Incident = {
  id: string;
  title: string;
  severity: string;
  status: string;
  owner: string;
  summary: string;
  resolution: string;
  runbook?: string;
  asset_id?: string;
  opened_at: string;
  resolved_at?: string;
};

export type SeverityPolicy = {
  id: string;
  name: string;
  match_kind: string;
  match_value: string;
  severity: string;
  runbook: string;
  priority: number;
  created_at: string;
};

export type WorkOrder = {
  id: string;
  title: string;
  kind: string;
  priority: string;
  status: string;
  assignee: string;
  notes: string;
  incident_id?: string;
  asset_id?: string;
  due_at?: string;
  schedule_cron?: string;
  created_at: string;
};

export type EventItem = {
  id: string;
  kind: string;
  severity: string;
  title: string;
  body: string;
  created_at: string;
};

export type Telemetry = {
  asset_id: string;
  asset_name: string;
  capability: string;
  value: number;
  value_kind?: string;
  value_text?: string;
  unit: string;
  quality: string;
  source: string;
  observed_at: string;
  received_at: string;
  fresh: boolean;
};

export type Observation = {
  id: string;
  asset_id: string;
  capability: string;
  value: number;
  value_kind?: string;
  value_text?: string;
  unit: string;
  quality: string;
  source: string;
  observed_at: string;
  received_at: string;
};

export type Overview = {
  assets_total: number;
  assets_healthy: number;
  assets_degraded: number;
  assets_critical: number;
  assets_stale: number;
  open_incidents: number;
  open_work_orders: number;
  active_connectors: number;
  recent_events: EventItem[];
  recent_activity: { id: string; actor: string; action: string; object: string; detail: string; created_at: string }[];
  health_by_kind: Record<string, number>;
};

export type Connector = {
  id: string;
  name: string;
  kind: string;
  status: string;
  endpoint: string;
  token_hint: string;
  actions: string;
  config: string;
  has_secret?: boolean;
  secret_hint?: string;
  last_sync_at?: string;
  last_error?: string;
  last_latency_ms?: number;
  sync_interval_sec?: number;
};

export type AdminUser = {
  id: string;
  organization_id: string;
  email: string;
  display_name: string;
  role: string;
  active: boolean;
  created_at: string;
};

export type APIKeyInfo = {
  id: string;
  user_id: string;
  name: string;
  token_hint: string;
  created_at: string;
  last_used_at?: string;
};

export type Automation = {
  id: string;
  name: string;
  enabled: boolean;
  trigger_kind: string;
  capability: string;
  operator: string;
  threshold: number;
  action: string;
  config: string;
};
