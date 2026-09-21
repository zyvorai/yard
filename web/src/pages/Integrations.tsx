import { FormEvent, useEffect, useState } from "react";
import { api, Connector } from "../lib/api";
import { fmt } from "../components/Shell";

type ActionResult = { id: string; status: string; result: string; action: string };
type ActionRow = { id: string; action: string; status: string; result: string; created_at: string; expires_at?: string };
type JobRow = {
  id: string;
  kind: string;
  status: string;
  attempts: number;
  max_attempts: number;
  action_id?: string;
  requested_by?: string;
  error?: string;
  result?: string;
  run_after: string;
  created_at: string;
};
type Me = { id: string; role: string };

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
  const [jobs, setJobs] = useState<JobRow[]>([]);
  const [canWrite, setCanWrite] = useState(false);
  const [meID, setMeID] = useState("");
  const [msg, setMsg] = useState("");
  const [busy, setBusy] = useState("");

  async function load() {
    const [c, a, j, me] = await Promise.all([
      api<Connector[]>("/api/v1/connectors"),
      api<ActionRow[]>("/api/v1/actions"),
      api<JobRow[]>("/api/v1/jobs"),
      api<Me>("/api/v1/auth/me"),
    ]);
    setRows(c);
    setActions(a);
    setJobs(j);
    setCanWrite(me.role === "admin" || me.role === "operator");
    setMeID(me.id);
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
      await api(`/api/v1/connectors/${c.id}/secret`, {
        method: "PUT",
        body: JSON.stringify({ secret: token }),
      });
      setMsg(`Auth token saved for ${c.name}`);
      await load();
    } catch (ex) {
      setMsg(ex instanceof Error ? ex.message : "token save failed");
    } finally {
      setBusy("");
    }
  }

  async function connectorOp(c: Connector, op: "test" | "sync") {
    setBusy(c.id + op);
    setMsg("");
    try {
      const res = await api<{ ok?: boolean; error?: string; latency_ms?: number; status?: string }>(`/api/v1/connectors/${c.id}/${op}`, { method: "POST" });
      if (op === "test") {
        setMsg(res.ok ? `${c.name} reachable in ${res.latency_ms ?? 0} ms` : `${c.name} test failed: ${res.error || "unreachable"}`);
      } else {
        setMsg(`${c.name} sync ${res.status || "queued"}`);
      }
      await load();
    } catch (ex) {
      setMsg(ex instanceof Error ? ex.message : `${op} failed`);
    } finally {
      setBusy("");
    }
  }

  async function saveInterval(c: Connector, seconds: number) {
    setBusy(c.id + "int");
    setMsg("");
    try {
      await api("/api/v1/connectors", { method: "PATCH", body: JSON.stringify({ id: c.id, sync_interval_sec: seconds }) });
      setMsg(seconds ? `${c.name} syncs every ${seconds}s` : `${c.name} sync schedule cleared`);
      await load();
    } catch (ex) {
      setMsg(ex instanceof Error ? ex.message : "interval save failed");
    } finally {
      setBusy("");
    }
  }
  async function jobOp(job: JobRow, op: "retry" | "cancel" | "approve") {
    setBusy(job.id + op);
    setMsg("");
    try {
      await api(`/api/v1/jobs/${job.id}/${op}`, { method: "POST" });
      setMsg(op === "retry" ? `Queued ${job.id} again` : op === "approve" ? `Approved ${job.id}` : `Cancelled ${job.id}`);
      await load();
    } catch (ex) {
      setMsg(ex instanceof Error ? ex.message : `${op} failed`);
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
                  hasToken={!!c.has_secret}
                  disabled={!!busy}
                  onSave={(tok) => saveAuthToken(c, tok)}
                />
              )}
              <p className="lede">Actions: {actions.join(", ") || "none"}</p>
              {c.token_hint && <p className="lede">Token {c.token_hint}</p>}
              <p className="lede">Last sync {fmt(c.last_sync_at)}{c.last_latency_ms ? ` · ${c.last_latency_ms} ms` : ""}</p>
              {c.last_error && <p className="lede">{c.last_error}</p>}
              {canWrite && (
                <div className="row-actions" style={{ marginTop: 8 }}>
                  <button type="button" className="btn small ghost" disabled={!!busy || !c.endpoint} onClick={() => connectorOp(c, "test")}>Test</button>
                  <button type="button" className="btn small ghost" disabled={!!busy || !c.endpoint} onClick={() => connectorOp(c, "sync")}>Sync now</button>
                  <button type="button" className="btn small ghost" disabled={!!busy} onClick={() => saveInterval(c, c.sync_interval_sec ? 0 : 300)}>
                    {c.sync_interval_sec ? "Stop schedule" : "Sync every 5m"}
                  </button>
                </div>
              )}
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
      <div className="card table-wrap" style={{ marginTop: 16 }}>
        <h2 style={{ marginBottom: 12 }}>Action jobs</h2>
        <p className="lede">Dead jobs can be retried. Queued jobs can be cancelled before a worker claims them.</p>
        <table>
          <thead><tr><th>Job</th><th>Status</th><th>Attempts</th><th>Error</th><th></th></tr></thead>
          <tbody>
            {jobs.slice(0, 20).map((j) => (
              <tr key={j.id}>
                <td>{j.kind}</td>
                <td><span className={`pill ${j.status === "succeeded" || j.status === "ok" ? "ok" : j.status === "dead" || j.status === "failed" || j.status === "cancelled" ? "bad" : "stale"}`}>{j.status}</span></td>
                <td>{j.attempts}/{j.max_attempts}</td>
                <td style={{ maxWidth: 280, overflow: "hidden", textOverflow: "ellipsis" }}>{j.error || j.result || "—"}</td>
                <td className="row-actions">
                  {canWrite && (j.status === "dead" || j.status === "failed") && (
                    <button type="button" className="btn small accent" disabled={!!busy} onClick={() => jobOp(j, "retry")}>Retry</button>
                  )}
                  {canWrite && j.status === "pending_approval" && j.requested_by !== meID && (
                    <button type="button" className="btn small accent" disabled={!!busy} onClick={() => jobOp(j, "approve")}>Approve</button>
                  )}
                  {canWrite && j.status === "queued" && (
                    <button type="button" className="btn small ghost" disabled={!!busy} onClick={() => jobOp(j, "cancel")}>Cancel</button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!jobs.length && <p className="empty">No durable jobs yet. A connector action creates one.</p>}
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
