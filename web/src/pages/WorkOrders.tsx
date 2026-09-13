import { FormEvent, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, Asset, WorkOrder } from "../lib/api";
import { fmt } from "../components/Shell";

export default function WorkOrders() {
  const [rows, setRows] = useState<WorkOrder[]>([]);
  const [assets, setAssets] = useState<Asset[]>([]);
  const [open, setOpen] = useState(false);
  const [err, setErr] = useState("");
  const [form, setForm] = useState({
    title: "", kind: "repair", priority: "normal", assignee: "", asset_id: "", notes: "", due_at: "",
  });

  async function load() {
    const [w, a] = await Promise.all([
      api<WorkOrder[]>("/api/v1/work-orders"),
      api<Asset[]>("/api/v1/assets"),
    ]);
    setRows(w);
    setAssets(a);
  }
  useEffect(() => { load(); }, []);

  async function done(id: string) {
    await api(`/api/v1/work-orders/${id}`, { method: "PATCH", body: JSON.stringify({ status: "done", notes: "Completed in the field." }) });
    await load();
  }

  async function create(e: FormEvent) {
    e.preventDefault();
    setErr("");
    if (!form.title.trim()) {
      setErr("Title is required");
      return;
    }
    const body: Record<string, unknown> = {
      title: form.title.trim(),
      kind: form.kind,
      priority: form.priority,
      assignee: form.assignee.trim(),
      notes: form.notes.trim(),
    };
    if (form.asset_id) body.asset_id = form.asset_id;
    if (form.due_at) body.due_at = new Date(form.due_at + "T17:00:00Z").toISOString();
    try {
      await api("/api/v1/work-orders", { method: "POST", body: JSON.stringify(body) });
      setForm({ title: "", kind: "repair", priority: "normal", assignee: "", asset_id: "", notes: "", due_at: "" });
      setOpen(false);
      await load();
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : "create failed");
    }
  }

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Work orders</h1>
          <p className="lede">Inspections, repairs, installations, and maintenance.</p>
        </div>
        <button type="button" className="btn accent" onClick={() => setOpen((v) => !v)}>{open ? "Cancel" : "New work order"}</button>
      </div>
      {open && (
        <form className="card form-card" onSubmit={create} style={{ marginBottom: 16 }}>
          <h2>New work order</h2>
          <div className="form-grid">
            <label className="span-2">Title<input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} required /></label>
            <label>Kind
              <select value={form.kind} onChange={(e) => setForm({ ...form, kind: e.target.value })}>
                {["repair", "inspection", "installation", "maintenance"].map((k) => <option key={k}>{k}</option>)}
              </select>
            </label>
            <label>Priority
              <select value={form.priority} onChange={(e) => setForm({ ...form, priority: e.target.value })}>
                {["low", "normal", "high", "critical"].map((k) => <option key={k}>{k}</option>)}
              </select>
            </label>
            <label>Assignee<input value={form.assignee} onChange={(e) => setForm({ ...form, assignee: e.target.value })} /></label>
            <label>Due date<input type="date" value={form.due_at} onChange={(e) => setForm({ ...form, due_at: e.target.value })} /></label>
            <label className="span-2">Asset
              <select value={form.asset_id} onChange={(e) => setForm({ ...form, asset_id: e.target.value })}>
                <option value="">None</option>
                {assets.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
              </select>
            </label>
            <label className="span-2">Notes<textarea value={form.notes} onChange={(e) => setForm({ ...form, notes: e.target.value })} rows={3} style={{ border: "1px solid var(--line)", borderRadius: 10, padding: 8, background: "var(--field-bg)", color: "var(--ink)" }} /></label>
          </div>
          {err && <p className="form-error">{err}</p>}
          <div className="row-actions" style={{ marginTop: 12 }}>
            <button className="btn accent" type="submit">Create</button>
          </div>
        </form>
      )}
      <div className="card table-wrap">
        <table>
          <thead><tr><th>Title</th><th>Kind</th><th>Priority</th><th>Status</th><th>Assignee</th><th>Due</th><th>Opened</th><th></th></tr></thead>
          <tbody>
            {rows.map((w) => (
              <tr key={w.id}>
                <td>{w.title}</td><td>{w.kind}</td><td>{w.priority}</td><td>{w.status}</td><td>{w.assignee || "—"}</td>
                <td>{w.due_at ? fmt(w.due_at) : "—"}</td><td>{fmt(w.created_at)}</td>
                <td>{w.status !== "done" && <button className="btn small" onClick={() => done(w.id)}>Complete</button>}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {!rows.length && (
          <p className="empty">
            No work orders yet.{" "}
            <button type="button" className="btn small accent" onClick={() => setOpen(true)}>Create one</button>
            {" "}or open from an <Link to="/incidents">incident</Link>.
          </p>
        )}
      </div>
    </>
  );
}
