import { useEffect, useState } from "react";
import { api, Incident, WorkOrder } from "../lib/api";
import { fmt, Health } from "../components/Shell";

export default function Incidents() {
  const [rows, setRows] = useState<Incident[]>([]);
  const [sel, setSel] = useState<Incident | null>(null);
  const [msg, setMsg] = useState("");
  async function load() {
    setRows(await api<Incident[]>("/api/v1/incidents"));
  }
  useEffect(() => { load(); }, []);

  async function ack() {
    if (!sel) return;
    const next = await api<Incident>(`/api/v1/incidents/${sel.id}`, { method: "PATCH", body: JSON.stringify({ status: "ack", owner: "Operations Admin" }) });
    setSel(next); load();
  }
  async function assignWork() {
    if (!sel) return;
    await api<WorkOrder>("/api/v1/work-orders", {
      method: "POST",
      body: JSON.stringify({ title: `Maintenance for ${sel.title}`, kind: "maintenance", priority: "high", incident_id: sel.id, asset_id: sel.asset_id, assignee: "Maya Chen" }),
    });
    setMsg("Work order created.");
  }
  async function resolve() {
    if (!sel) return;
    const next = await api<Incident>(`/api/v1/incidents/${sel.id}`, {
      method: "PATCH",
      body: JSON.stringify({ status: "resolved", resolution: "Inspected on site; condition returned to band." }),
    });
    setSel(next); load();
  }

  return (
    <>
      <div className="topbar"><div><h1>Incidents</h1><p className="lede">Acknowledge problems, assign owners, record resolution.</p></div></div>
      <div className="split">
        <div className="card table-wrap">
          <table>
            <thead><tr><th>Title</th><th>Severity</th><th>Status</th><th>Opened</th></tr></thead>
            <tbody>
              {rows.map((i) => (
                <tr key={i.id} className={sel?.id === i.id ? "selected" : ""} onClick={() => { setSel(i); setMsg(""); }} style={{ cursor: "pointer" }}>
                  <td>{i.title}</td><td><Health value={i.severity} /></td><td>{i.status}</td><td>{fmt(i.opened_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {!rows.length && <p className="empty">No incidents. A temperature trip from the simulator will open one.</p>}
        </div>
        <aside className="panel">
          {!sel && <p className="empty">Select an incident.</p>}
          {sel && (
            <>
              <h1 style={{ fontSize: 22 }}>{sel.title}</h1>
              <p className="lede">{sel.summary}</p>
              <p><Health value={sel.severity} /> {sel.status} · {sel.owner || "unassigned"}</p>
              {sel.runbook && (
                <div style={{ marginTop: 12 }}>
                  <h2 style={{ fontSize: 15, marginBottom: 6 }}>Runbook</h2>
                  <pre className="runbook">{sel.runbook}</pre>
                </div>
              )}
              {sel.resolution && <p className="lede">Resolution: {sel.resolution}</p>}
              <div className="row-actions" style={{ marginTop: 16 }}>
                <button className="btn small" onClick={ack}>Acknowledge</button>
                <button className="btn small accent" onClick={assignWork}>Assign work order</button>
                <button className="btn small ghost" onClick={resolve}>Resolve</button>
              </div>
              {msg && <p className="lede" style={{ marginTop: 12 }}>{msg}</p>}
            </>
          )}
        </aside>
      </div>
    </>
  );
}
