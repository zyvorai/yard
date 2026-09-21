import { FormEvent, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, Asset, WorkOrder } from "../lib/api";
import { fmt } from "../components/Shell";

type Line = { id: string; kind: string; name: string; quantity: number; unit: string; unit_cost_cents: number };

export default function WorkOrders() {
  const [rows, setRows] = useState<WorkOrder[]>([]);
  const [assets, setAssets] = useState<Asset[]>([]);
  const [open, setOpen] = useState(false);
  const [err, setErr] = useState("");
  const [sel, setSel] = useState<WorkOrder | null>(null);
  const [lines, setLines] = useState<Line[]>([]);
  const [canWrite, setCanWrite] = useState(false);
  const [lineForm, setLineForm] = useState({ kind: "part", name: "", quantity: "1", unit: "ea", cost: "" });
  const [form, setForm] = useState({
    title: "", kind: "repair", priority: "normal", assignee: "", asset_id: "", notes: "", due_at: "", schedule_cron: "",
  });

  async function load() {
    const [w, a, me] = await Promise.all([
      api<WorkOrder[]>("/api/v1/work-orders"),
      api<Asset[]>("/api/v1/assets"),
      api<{ role: string }>("/api/v1/auth/me"),
    ]);
    setRows(w);
    setAssets(a);
    setCanWrite(me.role === "admin" || me.role === "operator");
  }
  useEffect(() => { load(); }, []);

  async function select(w: WorkOrder) {
    setSel(w);
    setLines(await api<Line[]>(`/api/v1/work-orders/${w.id}/lines`));
    setErr("");
  }

  async function addLine(e: FormEvent) {
    e.preventDefault();
    if (!sel) return;
    setErr("");
    const dollars = Number(lineForm.cost || "0");
    if (!lineForm.name.trim() || Number.isNaN(dollars) || dollars < 0) {
      setErr("Name and a non-negative cost are required");
      return;
    }
    try {
      await api(`/api/v1/work-orders/${sel.id}/lines`, {
        method: "POST",
        body: JSON.stringify({
          kind: lineForm.kind,
          name: lineForm.name.trim(),
          quantity: Number(lineForm.quantity) || 1,
          unit: lineForm.unit.trim(),
          unit_cost_cents: Math.round(dollars * 100),
        }),
      });
      setLineForm({ kind: "part", name: "", quantity: "1", unit: lineForm.kind === "labor" ? "h" : "ea", cost: "" });
      setLines(await api<Line[]>(`/api/v1/work-orders/${sel.id}/lines`));
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : "line failed");
    }
  }

  const total = lines.reduce((sum, l) => sum + l.quantity * l.unit_cost_cents, 0);

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
    if (form.schedule_cron.trim()) body.schedule_cron = form.schedule_cron.trim();
    try {
      await api("/api/v1/work-orders", { method: "POST", body: JSON.stringify(body) });
      setForm({ title: "", kind: "repair", priority: "normal", assignee: "", asset_id: "", notes: "", due_at: "", schedule_cron: "" });
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
            <label className="span-2">Schedule (optional)
              <input value={form.schedule_cron} onChange={(e) => setForm({ ...form, schedule_cron: e.target.value })} placeholder="0 8 * * 1" />
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
                <td><button type="button" className="btn small ghost" onClick={() => select(w)}>Parts</button> {w.status !== "done" && <button className="btn small" onClick={() => done(w.id)}>Complete</button>}</td>
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
      {sel && (
        <div className="card" style={{ marginTop: 16 }}>
          <h2>Parts and labor · {sel.title}</h2>
          <p className="lede">Total ${(total / 100).toFixed(2)}</p>
          {lines.map((l) => (
            <p key={l.id}>{l.kind}: {l.name} · {l.quantity} {l.unit} · ${((l.quantity * l.unit_cost_cents) / 100).toFixed(2)}</p>
          ))}
          {!lines.length && <p className="empty">No parts or labor yet.</p>}
          {canWrite && (
            <form onSubmit={addLine} className="form-grid" style={{ marginTop: 12 }}>
              <label>Kind
                <select value={lineForm.kind} onChange={(e) => setLineForm({ ...lineForm, kind: e.target.value, unit: e.target.value === "labor" ? "h" : "ea" })}>
                  <option value="part">part</option>
                  <option value="labor">labor</option>
                </select>
              </label>
              <label>Name<input value={lineForm.name} onChange={(e) => setLineForm({ ...lineForm, name: e.target.value })} required /></label>
              <label>Qty<input value={lineForm.quantity} onChange={(e) => setLineForm({ ...lineForm, quantity: e.target.value })} /></label>
              <label>Unit<input value={lineForm.unit} onChange={(e) => setLineForm({ ...lineForm, unit: e.target.value })} /></label>
              <label>Unit cost<input value={lineForm.cost} onChange={(e) => setLineForm({ ...lineForm, cost: e.target.value })} placeholder="12.50" /></label>
              <button className="btn small accent" type="submit">Add</button>
            </form>
          )}
        </div>
      )}
    </>
  );
}
