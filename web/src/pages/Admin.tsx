import { useEffect, useState } from "react";
import { api, Connector } from "../lib/api";
import { fmt } from "../components/Shell";

type Audit = { id: string; actor: string; action: string; object: string; detail: string; created_at: string };
type Me = { id: string; email: string; display_name: string; role: string; organization_id: string };

export default function Admin() {
  const [rows, setRows] = useState<Audit[]>([]);
  const [connectors, setConnectors] = useState<Connector[]>([]);
  const [me, setMe] = useState<Me | null>(null);
  const [tokenReveal, setTokenReveal] = useState<Record<string, string>>({});
  const [msg, setMsg] = useState("");

  async function load() {
    const [audit, cons, user] = await Promise.all([
      api<Audit[]>("/api/v1/audit"),
      api<Connector[]>("/api/v1/connectors"),
      api<Me>("/api/v1/auth/me"),
    ]);
    setRows(audit);
    setConnectors(cons);
    setMe(user);
  }
  useEffect(() => { load(); }, []);

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

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Administration</h1>
          <p className="lede">Workspace, connector credentials, and audit history.</p>
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
      </div>

      <div className="card table-wrap" style={{ marginBottom: 16 }}>
        <h2 style={{ marginBottom: 12 }}>Connector credentials</h2>
        <table>
          <thead><tr><th>Name</th><th>Kind</th><th>Status</th><th>Hint</th><th></th></tr></thead>
          <tbody>
            {connectors.map((c) => (
              <tr key={c.id}>
                <td>{c.name}</td>
                <td>{c.kind}</td>
                <td><span className={`pill ${c.status === "connected" ? "ok" : "stale"}`}>{c.status}</span></td>
                <td>
                  {tokenReveal[c.id] ? <code style={{ fontSize: 12 }}>{tokenReveal[c.id]}</code> : (c.token_hint || "—")}
                </td>
                <td>
                  {(c.kind === "http" || c.kind === "simulator" || c.kind === "device-agent") && (
                    <button type="button" className="btn small ghost" onClick={() => rotate(c)}>Rotate token</button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
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
