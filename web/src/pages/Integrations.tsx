import { FormEvent, useEffect, useState } from "react";
import { api, Connector } from "../lib/api";
import { fmt } from "../components/Shell";

type ActionResult = { id: string; status: string; result: string; action: string };
type ActionRow = { id: string; action: string; status: string; result: string; created_at: string; expires_at?: string };

function parseActions(raw: string): string[] {
  try {
    const v = JSON.parse(raw || "[]");
    return Array.isArray(v) ? v.map(String) : [];
  } catch {
    return [];
  }
}

export default function Integrations() {
  const [rows, setRows] = useState<Connector[]>([]);
  const [actions, setActions] = useState<ActionRow[]>([]);
  const [msg, setMsg] = useState("");
  const [busy, setBusy] = useState("");

  async function load() {
    const [c, a] = await Promise.all([
      api<Connector[]>("/api/v1/connectors"),
      api<ActionRow[]>("/api/v1/actions"),
    ]);
    setRows(c);
    setActions(a);
  }
  useEffect(() => { load(); }, []);

  async function saveEndpoint(c: Connector, endpoint: string) {
    setBusy(c.id);
    setMsg("");
    try {
      await api(`/api/v1/connectors`, {
        method: "PATCH",
        body: JSON.stringify({ id: c.id, endpoint, status: endpoint ? "pending" : c.status }),
      });
      setMsg(`Updated ${c.name}`);
      await load();
    } catch (ex) {
      setMsg(ex instanceof Error ? ex.message : "update failed");
    } finally {
      setBusy("");
    }
  }

  async function saveAuthToken(c: Connector, token: string) {
    setBusy(c.id + "tok");
    setMsg("");
    try {
      let cfg: Record<string, unknown> = {};
      try { cfg = JSON.parse(c.config || "{}"); } catch { /* ignore */ }
      if (token) cfg.auth_token = token;
      else delete cfg.auth_token;
      await api(`/api/v1/connectors`, {
        method: "PATCH",
        body: JSON.stringify({ id: c.id, config: JSON.stringify(cfg) }),
      });
      setMsg(`Auth token saved for ${c.name}`);
      await load();
    } catch (ex) {
      setMsg(ex instanceof Error ? ex.message : "token save failed");
    } finally {
      setBusy("");
    }
  }

  async function runAction(c: Connector, action: string) {
    setBusy(c.id + action);
    setMsg("");
    try {
      const res = await api<ActionResult>("/api/v1/actions", {
        method: "POST",
        body: JSON.stringify({ connector_id: c.id, action, payload: "{}" }),
      });
      setMsg(`${action}: ${res.status} — ${res.result?.slice(0, 160) || ""}`);
      await load();
    } catch (ex) {
      setMsg(ex instanceof Error ? ex.message : "action failed");
    } finally {
      setBusy("");
    }
  }

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Integrations</h1>
          <p className="lede">Agents, HTTP ingestion, and optional Zyvor connectors. Device Agent can sync from here.</p>
        </div>
      </div>
      {msg && <p className="lede" style={{ marginBottom: 12 }}>{msg}</p>}
      <div className="grid" style={{ gridTemplateColumns: "repeat(auto-fill,minmax(300px,1fr))" }}>
        {rows.map((c) => {
          const actions = parseActions(c.actions);
          const executable = c.kind === "device-agent" || c.kind === "nodra" || c.kind === "fleet" || c.kind === "ota";
          return (
            <div className="card" key={c.id}>
              <h2>{c.kind}</h2>
              <div className="card-title">{c.name}</div>
              <p>
                <span className={`pill ${c.status === "connected" ? "ok" : c.status === "available" ? "info" : "stale"}`}>{c.status}</span>
              </p>
              <EndpointForm
                key={c.id + (c.endpoint || "")}
                initial={c.endpoint || ""}
                disabled={busy.startsWith(c.id)}
                onSave={(endpoint) => saveEndpoint(c, endpoint)}
              />
              {(c.kind === "nodra" || c.kind === "fleet" || c.kind === "ota") && (
                <AuthTokenForm
                  key={c.id + "-tok"}
                  hasToken={!!(() => { try { return JSON.parse(c.config || "{}").auth_token; } catch { return false; } })()}
                  disabled={!!busy}
                  onSave={(tok) => saveAuthToken(c, tok)}
                />
              )}
              <p className="lede">Actions: {actions.join(", ") || "none"}</p>
              {c.token_hint && <p className="lede">Token {c.token_hint}</p>}
              <p className="lede">Last sync {fmt(c.last_sync_at)}</p>
              {executable && (
                <div className="row-actions" style={{ marginTop: 10 }}>
                  {actions.map((a) => (
                    <button
                      key={a}
                      type="button"
                      className="btn small accent"
                      disabled={!!busy || !c.endpoint}
                      onClick={() => runAction(c, a)}
                    >
                      {a}
                    </button>
                  ))}
                </div>
              )}
              {!c.endpoint && (c.kind === "nodra" || c.kind === "fleet" || c.kind === "ota") && (
                <p className="lede">Save an endpoint to enable sync actions.</p>
              )}
            </div>
          );
        })}
        {!rows.length && (
          <p className="empty">No connectors yet. Finish <a href="/onboarding">Get started</a> or check seed data.</p>
        )}
      </div>
      <div className="card table-wrap" style={{ marginTop: 16 }}>
        <h2 style={{ marginBottom: 12 }}>Recent actions</h2>
        <table>
          <thead><tr><th>Action</th><th>Status</th><th>Result</th><th>When</th></tr></thead>
          <tbody>
            {actions.slice(0, 20).map((a) => (
              <tr key={a.id}>
                <td>{a.action}</td>
                <td><span className={`pill ${a.status === "done" || a.status === "ok" ? "ok" : a.status === "expired" || a.status === "failed" ? "bad" : "stale"}`}>{a.status}</span></td>
                <td style={{ maxWidth: 280, overflow: "hidden", textOverflow: "ellipsis" }}>{a.result || "—"}</td>
                <td>{fmt(a.created_at)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {!actions.length && <p className="empty">No remote actions yet. Run inventory.refresh from Device Agent.</p>}
      </div>
    </>
  );
}

function EndpointForm({ initial, disabled, onSave }: { initial: string; disabled: boolean; onSave: (v: string) => void }) {
  const [value, setValue] = useState(initial);
  function submit(e: FormEvent) {
    e.preventDefault();
    onSave(value.trim());
  }
  return (
    <form onSubmit={submit} className="endpoint-form">
      <label className="lede" style={{ display: "block", marginTop: 8 }}>
        Endpoint
        <input value={value} onChange={(e) => setValue(e.target.value)} placeholder="http://127.0.0.1:9188" disabled={disabled} />
      </label>
      <button className="btn small ghost" type="submit" disabled={disabled} style={{ marginTop: 8 }}>Save</button>
    </form>
  );
}

function AuthTokenForm({ hasToken, disabled, onSave }: { hasToken: boolean; disabled: boolean; onSave: (v: string) => void }) {
  const [value, setValue] = useState("");
  function submit(e: FormEvent) {
    e.preventDefault();
    onSave(value.trim());
    setValue("");
  }
  return (
    <form onSubmit={submit} className="endpoint-form">
      <label className="lede" style={{ display: "block", marginTop: 8 }}>
        API bearer token {hasToken ? "(set)" : "(required for sync)"}
        <input
          type="password"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder={hasToken ? "•••••••• (enter to replace)" : "paste token"}
          disabled={disabled}
          autoComplete="off"
        />
      </label>
      <button className="btn small ghost" type="submit" disabled={disabled || !value.trim()} style={{ marginTop: 8 }}>Save token</button>
    </form>
  );
}
