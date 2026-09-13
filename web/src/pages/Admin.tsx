import { FormEvent, useEffect, useState } from "react";
import { api, AdminUser, APIKeyInfo, Connector, SeverityPolicy } from "../lib/api";
import { fmt, Health } from "../components/Shell";
import { GroupedList, GroupedRow } from "../components/GroupedList";

type Audit = { id: string; actor: string; action: string; object: string; detail: string; created_at: string };
type Me = { id: string; email: string; display_name: string; role: string; organization_id: string };

const emptyInvite = { email: "", display_name: "", role: "viewer" };

function ConnectorIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M9 3v4M15 3v4M7 7h10a2 2 0 0 1 2 2v2a5 5 0 0 1-5 5h-4a5 5 0 0 1-5-5V9a2 2 0 0 1 2-2ZM12 16v5"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

const emptyPolicy = {
  name: "",
  match_kind: "capability",
  match_value: "",
  severity: "warning",
  runbook: "",
  priority: "50",
};

export default function Admin() {
  const [rows, setRows] = useState<Audit[]>([]);
  const [connectors, setConnectors] = useState<Connector[]>([]);
  const [policies, setPolicies] = useState<SeverityPolicy[]>([]);
  const [me, setMe] = useState<Me | null>(null);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [apiKeys, setApiKeys] = useState<APIKeyInfo[]>([]);
  const [tokenReveal, setTokenReveal] = useState<Record<string, string>>({});
  const [inviteReveal, setInviteReveal] = useState<Record<string, string>>({});
  const [keyReveal, setKeyReveal] = useState<Record<string, string>>({});
  const [msg, setMsg] = useState("");
  const [form, setForm] = useState(emptyPolicy);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [inviteForm, setInviteForm] = useState(emptyInvite);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [keyName, setKeyName] = useState("");

  async function load() {
    const [audit, cons, user, pols, keys] = await Promise.all([
      api<Audit[]>("/api/v1/audit"),
      api<Connector[]>("/api/v1/connectors"),
      api<Me>("/api/v1/auth/me"),
      api<SeverityPolicy[]>("/api/v1/severity-policies"),
      api<APIKeyInfo[]>("/api/v1/api-keys"),
    ]);
    setRows(audit);
    setConnectors(cons);
    setMe(user);
    setPolicies(pols);
    setApiKeys(keys);
    if (user.role === "admin") {
      try {
        setUsers(await api<AdminUser[]>("/api/v1/admin/users"));
      } catch {
        setUsers([]);
      }
    }
  }
  useEffect(() => { load(); }, []);

  async function invite(e: FormEvent) {
    e.preventDefault();
    setMsg("");
    try {
      const res = await api<{ user: AdminUser; invite_token: string }>("/api/v1/admin/users", {
        method: "POST",
        body: JSON.stringify({
          email: inviteForm.email.trim(),
          display_name: inviteForm.display_name.trim(),
          role: inviteForm.role,
        }),
      });
      setInviteReveal((m) => ({ ...m, [res.user.id]: res.invite_token }));
      setMsg(`Invited ${res.user.email} — share this link once: ${window.location.origin}/accept-invite?token=${res.invite_token}`);
      setInviteForm(emptyInvite);
      setInviteOpen(false);
      await load();
    } catch (ex) {
      setMsg(ex instanceof Error ? ex.message : "invite failed");
    }
  }

  async function setRole(u: AdminUser, role: string) {
    await api("/api/v1/admin/users", { method: "PATCH", body: JSON.stringify({ id: u.id, role }) });
    await load();
  }

  async function setActive(u: AdminUser, active: boolean) {
    if (!active && !confirm(`Deactivate ${u.email}? They will be signed out immediately.`)) return;
    await api("/api/v1/admin/users", { method: "PATCH", body: JSON.stringify({ id: u.id, active }) });
    await load();
  }

  async function createKey(e: FormEvent) {
    e.preventDefault();
    setMsg("");
    try {
      const res = await api<{ api_key: APIKeyInfo; token: string }>("/api/v1/api-keys", {
        method: "POST",
        body: JSON.stringify({ name: keyName.trim() }),
      });
      setKeyReveal((m) => ({ ...m, [res.api_key.id]: res.token }));
      setMsg(`New API key "${res.api_key.name}" — copy it now; it is shown once.`);
      setKeyName("");
      await load();
    } catch (ex) {
      setMsg(ex instanceof Error ? ex.message : "create key failed");
    }
  }

  async function revokeKey(k: APIKeyInfo) {
    if (!confirm(`Revoke API key "${k.name}"?`)) return;
    await api("/api/v1/api-keys", { method: "PATCH", body: JSON.stringify({ id: k.id, delete: true }) });
    await load();
  }

  async function rotate(c: Connector) {
    setMsg("");
    try {
      const res = await api<{ connector: Connector; token?: string }>("/api/v1/connectors", {
        method: "PATCH",
        body: JSON.stringify({ id: c.id, rotate_token: true }),
      });
      if (res.token) {
        setTokenReveal((m) => ({ ...m, [c.id]: res.token! }));
        setMsg(`New token for ${c.name} — copy it now; it is shown once.`);
      }
      await load();
    } catch (ex) {
      setMsg(ex instanceof Error ? ex.message : "rotate failed");
    }
  }

  function startEdit(p: SeverityPolicy) {
    setEditingId(p.id);
    setForm({
      name: p.name,
      match_kind: p.match_kind,
      match_value: p.match_value,
      severity: p.severity,
      runbook: p.runbook,
      priority: String(p.priority),
    });
  }

  function cancelEdit() {
    setEditingId(null);
    setForm(emptyPolicy);
  }

  async function savePolicy(e: FormEvent) {
    e.preventDefault();
    setMsg("");
    const body = {
      name: form.name.trim(),
      match_kind: form.match_kind,
      match_value: form.match_kind === "default" ? "" : form.match_value.trim(),
      severity: form.severity,
      runbook: form.runbook,
      priority: Number(form.priority) || 0,
    };
    try {
      if (editingId) {
        await api(`/api/v1/severity-policies/${editingId}`, { method: "PATCH", body: JSON.stringify(body) });
        setMsg("Policy updated.");
      } else {
        await api("/api/v1/severity-policies", { method: "POST", body: JSON.stringify(body) });
        setMsg("Policy created.");
      }
      cancelEdit();
      await load();
    } catch (ex) {
      setMsg(ex instanceof Error ? ex.message : "save failed");
    }
  }

  async function removePolicy(p: SeverityPolicy) {
    if (!confirm(`Delete policy “${p.name}”?`)) return;
    await api(`/api/v1/severity-policies/${p.id}`, { method: "DELETE" });
    if (editingId === p.id) cancelEdit();
    await load();
  }

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Administration</h1>
          <p className="lede">Workspace, users, severity policies, connector credentials, API keys, and audit history.</p>
        </div>
      </div>
      {msg && <p className="lede" style={{ marginBottom: 12 }}>{msg}</p>}

      <div className="grid stats" style={{ marginBottom: 16 }}>
        <div className="card">
          <h2>Workspace</h2>
          <div className="metric" style={{ fontSize: 18 }}>{me?.display_name || "—"}</div>
          <p className="lede">{me?.email} · {me?.role}</p>
          <p className="lede">Org {me?.organization_id}</p>
        </div>
        <div className="card">
          <h2>Connectors</h2>
          <div className="metric">{connectors.length}</div>
          <p className="lede">{connectors.filter((c) => c.status === "connected").length} connected</p>
        </div>
        <div className="card">
          <h2>Severity policies</h2>
          <div className="metric">{policies.length}</div>
          <p className="lede">Match capability or automation → severity + runbook</p>
        </div>
      </div>

      {me?.role === "admin" && (
        <div className="card" style={{ marginBottom: 16 }}>
          <div className="topbar" style={{ marginBottom: 12 }}>
            <h2>Users</h2>
            <button type="button" className="btn small accent" onClick={() => setInviteOpen((v) => !v)}>
              {inviteOpen ? "Cancel" : "Invite user"}
            </button>
          </div>
          {inviteOpen && (
            <form className="form-card" onSubmit={invite} style={{ marginBottom: 16, padding: 0, border: "none", background: "transparent" }}>
              <div className="form-grid">
                <label>Email<input type="email" value={inviteForm.email} onChange={(e) => setInviteForm({ ...inviteForm, email: e.target.value })} required /></label>
                <label>Display name<input value={inviteForm.display_name} onChange={(e) => setInviteForm({ ...inviteForm, display_name: e.target.value })} /></label>
                <label>Role
                  <select value={inviteForm.role} onChange={(e) => setInviteForm({ ...inviteForm, role: e.target.value })}>
                    <option value="viewer">viewer</option>
                    <option value="operator">operator</option>
                    <option value="admin">admin</option>
                  </select>
                </label>
              </div>
              <div className="row-actions" style={{ marginTop: 12 }}>
                <button className="btn accent" type="submit">Send invite</button>
              </div>
            </form>
          )}
          {users.length ? (
            <GroupedList>
              {users.map((u) => (
                <GroupedRow
                  key={u.id}
                  tone={u.active ? "info" : "stale"}
                  icon="●"
                  label={u.display_name || u.email}
                  description={
                    inviteReveal[u.id]
                      ? `${u.email} · invite token: ${inviteReveal[u.id]}`
                      : `${u.email} · joined ${fmt(u.created_at)}`
                  }
                  trailing={
                    <>
                      <select value={u.role} onChange={(e) => setRole(u, e.target.value)} disabled={u.id === me?.id}>
                        <option value="viewer">viewer</option>
                        <option value="operator">operator</option>
                        <option value="admin">admin</option>
                      </select>
                      <button
                        type="button"
                        className="btn small ghost"
                        disabled={u.id === me?.id}
                        onClick={() => setActive(u, !u.active)}
                      >
                        {u.active ? "Deactivate" : "Reactivate"}
                      </button>
                    </>
                  }
                />
              ))}
            </GroupedList>
          ) : (
            <p className="empty">No other users yet.</p>
          )}
        </div>
      )}

      <div className="card" style={{ marginBottom: 16 }}>
        <h2 style={{ marginBottom: 12 }}>Incident severity policies</h2>
        <p className="lede" style={{ marginBottom: 12 }}>
          Highest priority match wins. Use match kind <code>capability</code>, <code>automation</code>, or <code>default</code>.
        </p>
        <form className="form-card" onSubmit={savePolicy} style={{ marginBottom: 16, padding: 0, border: "none", background: "transparent" }}>
          <div className="form-grid">
            <label>Name<input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></label>
            <label>Match kind
              <select value={form.match_kind} onChange={(e) => setForm({ ...form, match_kind: e.target.value })}>
                <option value="capability">capability</option>
                <option value="automation">automation</option>
                <option value="default">default</option>
              </select>
            </label>
            {form.match_kind !== "default" && (
              <label>Match value<input value={form.match_value} onChange={(e) => setForm({ ...form, match_value: e.target.value })} placeholder="temperature / High temperature incident" required /></label>
            )}
            <label>Severity
              <select value={form.severity} onChange={(e) => setForm({ ...form, severity: e.target.value })}>
                <option value="info">info</option>
                <option value="warning">warning</option>
                <option value="critical">critical</option>
              </select>
            </label>
            <label>Priority<input type="number" value={form.priority} onChange={(e) => setForm({ ...form, priority: e.target.value })} /></label>
            <label style={{ gridColumn: "1 / -1" }}>Runbook
              <textarea rows={4} value={form.runbook} onChange={(e) => setForm({ ...form, runbook: e.target.value })} placeholder="Step-by-step response notes" />
            </label>
          </div>
          <div className="row-actions" style={{ marginTop: 12 }}>
            <button className="btn accent" type="submit">{editingId ? "Save policy" : "Add policy"}</button>
            {editingId && <button className="btn ghost" type="button" onClick={cancelEdit}>Cancel</button>}
          </div>
        </form>
        {policies.length ? (
          <GroupedList>
            {policies.map((p) => (
              <GroupedRow
                key={p.id}
                tone={p.severity === "critical" ? "bad" : p.severity === "warning" ? "warn" : "info"}
                icon="●"
                label={p.name}
                description={`${p.match_kind}${p.match_value ? `:${p.match_value}` : ""} · priority ${p.priority}`}
                trailing={
                  <>
                    <Health value={p.severity} />
                    <button type="button" className="btn small ghost" onClick={() => startEdit(p)}>Edit</button>
                    <button type="button" className="btn small ghost" onClick={() => removePolicy(p)}>Delete</button>
                  </>
                }
              />
            ))}
          </GroupedList>
        ) : (
          <p className="empty">No policies yet — defaults are seeded on bootstrap.</p>
        )}
      </div>

      <div style={{ marginBottom: 16 }}>
        <GroupedList title="Connector credentials">
          {connectors.map((c) => (
            <GroupedRow
              key={c.id}
              icon={<ConnectorIcon />}
              tone={c.status === "connected" ? "info" : "stale"}
              label={c.name}
              description={
                tokenReveal[c.id]
                  ? `${c.kind} · ${tokenReveal[c.id]}`
                  : `${c.kind} · ${c.token_hint || "no token"}`
              }
              trailing={
                <>
                  <span className={`pill ${c.status === "connected" ? "ok" : "stale"}`}>{c.status}</span>
                  {(c.kind === "http" || c.kind === "simulator" || c.kind === "device-agent") && (
                    <button type="button" className="btn small ghost" onClick={() => rotate(c)}>Rotate token</button>
                  )}
                </>
              }
            />
          ))}
          {!connectors.length && (
            <div className="settings-row"><span className="row-body"><span className="row-description">No connectors yet.</span></span></div>
          )}
        </GroupedList>
      </div>

      <div className="card" style={{ marginBottom: 16 }}>
        <h2 style={{ marginBottom: 12 }}>Your API keys</h2>
        <p className="lede" style={{ marginBottom: 12 }}>
          Long-lived, per-person credentials for scripts and automation — separate from connector tokens, which are shared per-integration.
        </p>
        <form className="form-card" onSubmit={createKey} style={{ marginBottom: 16, padding: 0, border: "none", background: "transparent" }}>
          <div className="row-actions">
            <input value={keyName} onChange={(e) => setKeyName(e.target.value)} placeholder="Key name, e.g. CI script" required style={{ flex: 1 }} />
            <button className="btn accent" type="submit">Create key</button>
          </div>
        </form>
        {apiKeys.length ? (
          <GroupedList>
            {apiKeys.map((k) => (
              <GroupedRow
                key={k.id}
                icon="🔑"
                label={k.name}
                description={keyReveal[k.id] || `${k.token_hint} · ${k.last_used_at ? `last used ${fmt(k.last_used_at)}` : "never used"}`}
                trailing={<button type="button" className="btn small ghost" onClick={() => revokeKey(k)}>Revoke</button>}
              />
            ))}
          </GroupedList>
        ) : (
          <p className="empty">No API keys yet.</p>
        )}
      </div>

      <div className="card table-wrap">
        <h2 style={{ marginBottom: 12 }}>Audit history</h2>
        <table>
          <thead><tr><th>When</th><th>Actor</th><th>Action</th><th>Object</th><th>Detail</th></tr></thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.id}><td>{fmt(r.created_at)}</td><td>{r.actor}</td><td>{r.action}</td><td>{r.object}</td><td>{r.detail}</td></tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
