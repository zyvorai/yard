import { useEffect, useState } from "react";
import { api, Incident, IncidentNote, OnCall, WorkOrder } from "../lib/api";
import { fmt, Health } from "../components/Shell";
import { GroupedList, GroupedRow } from "../components/GroupedList";

export default function Incidents() {
  const [rows, setRows] = useState<Incident[]>([]);
  const [sel, setSel] = useState<Incident | null>(null);
  const [notes, setNotes] = useState<IncidentNote[]>([]);
  const [oncall, setOncall] = useState<OnCall[]>([]);
  const [msg, setMsg] = useState("");
  async function load() {
    setRows(await api<Incident[]>("/api/v1/incidents"));
    setOncall(await api<OnCall[]>("/api/v1/oncall"));
  }
  useEffect(() => { load(); }, []);
  useEffect(() => {
    if (!sel) { setNotes([]); return; }
    api<IncidentNote[]>(`/api/v1/incidents/${sel.id}/timeline`).then(setNotes).catch(() => setNotes([]));
  }, [sel?.id, sel?.status, sel?.flap_count]);

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
  async function takeOnCall() {
    const me = await api<{ id: string }>("/api/v1/auth/me");
    const start = new Date();
    const end = new Date(start.getTime() + 8 * 60 * 60 * 1000);
    await api("/api/v1/oncall", { method: "POST", body: JSON.stringify({ user_id: me.id, starts_at: start.toISOString(), ends_at: end.toISOString() }) });
    setMsg("You are on call for 8 hours.");
    load();
  }

  return (
    <>
      <div className="topbar"><div><h1>Incidents</h1><p className="lede">Acknowledge problems, assign owners, record resolution.</p></div>
        <button className="btn small" onClick={takeOnCall}>Take on-call</button>
      </div>
      {oncall.length > 0 && <p className="lede">On call: {oncall.map((o) => o.display_name || o.user_id).join(", ")}</p>}
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
              <GroupedList>
                <GroupedRow label="Severity" trailing={<Health value={sel.severity} />} />
                <GroupedRow label="Status" trailing={sel.status} />
                <GroupedRow label="Owner" trailing={sel.owner || "Unassigned"} />
                <GroupedRow label="Flaps" trailing={String(sel.flap_count || 0)} />
                {sel.parent_id && <GroupedRow label="Parent" trailing={sel.parent_id} />}
                <GroupedRow label="Ack due" trailing={sel.ack_breached ? "Breached" : fmt(sel.ack_due_at)} />
                <GroupedRow label="Resolve due" trailing={sel.resolve_breached ? "Breached" : fmt(sel.resolve_due_at)} />
              </GroupedList>
              {notes.length > 0 && (
                <div style={{ marginTop: 12 }}>
                  <h2 style={{ fontSize: 15, marginBottom: 6 }}>Timeline</h2>
                  {notes.map((n, i) => <p key={i} className="lede">{fmt(n.at)} · {n.kind} · {n.title}</p>)}
                </div>
              )}
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
