import { FormEvent, useEffect, useState } from "react";
import { api, Automation } from "../lib/api";

export default function Automations() {
  const [rows, setRows] = useState<Automation[]>([]);
  const [open, setOpen] = useState(false);
  const [err, setErr] = useState("");
  const [form, setForm] = useState({
    name: "", trigger_kind: "threshold", capability: "temperature", operator: "gt",
    threshold: "75", action: "open_incident", webhook_url: "",
  });

  async function load() {
    setRows(await api<Automation[]>("/api/v1/automations"));
  }
  useEffect(() => { load(); }, []);

  async function toggle(a: Automation) {
    await api("/api/v1/automations", { method: "PATCH", body: JSON.stringify({ id: a.id, enabled: !a.enabled }) });
    await load();
  }

  async function remove(a: Automation) {
    if (!confirm(`Delete automation ${a.name}?`)) return;
    await api("/api/v1/automations", { method: "PATCH", body: JSON.stringify({ id: a.id, delete: true }) });
    await load();
  }

  async function create(e: FormEvent) {
    e.preventDefault();
    setErr("");
    const body: Record<string, unknown> = {
      name: form.name.trim(),
      trigger_kind: form.trigger_kind,
      capability: form.capability.trim(),
      operator: form.operator,
      threshold: Number(form.threshold) || 0,
      action: form.action,
      config: "{}",
    };
    if (form.action === "webhook") {
      body.config = JSON.stringify({ url: form.webhook_url.trim() });
    }
    try {
      await api("/api/v1/automations", { method: "POST", body: JSON.stringify(body) });
      setOpen(false);
      setForm({
        name: "", trigger_kind: "threshold", capability: "temperature", operator: "gt",
        threshold: "75", action: "open_incident", webhook_url: "",
      });
      await load();
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : "create failed");
    }
  }

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Automations</h1>
          <p className="lede">Thresholds, stale checks, notifications, and webhooks.</p>
        </div>
        <button type="button" className="btn accent" onClick={() => setOpen((v) => !v)}>{open ? "Cancel" : "New rule"}</button>
      </div>
      {open && (
        <form className="card form-card" onSubmit={create} style={{ marginBottom: 16 }}>
          <h2>New automation</h2>
          <div className="form-grid">
            <label className="span-2">Name<input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></label>
            <label>Trigger
              <select value={form.trigger_kind} onChange={(e) => setForm({ ...form, trigger_kind: e.target.value })}>
                <option value="threshold">threshold</option>
                <option value="stale">stale</option>
              </select>
            </label>
            <label>Capability<input value={form.capability} onChange={(e) => setForm({ ...form, capability: e.target.value })} /></label>
            <label>Operator
              <select value={form.operator} onChange={(e) => setForm({ ...form, operator: e.target.value })}>
                {["gt", "gte", "lt", "lte"].map((o) => <option key={o}>{o}</option>)}
              </select>
            </label>
            <label>Threshold<input type="number" step="any" value={form.threshold} onChange={(e) => setForm({ ...form, threshold: e.target.value })} /></label>
            <label>Action
              <select value={form.action} onChange={(e) => setForm({ ...form, action: e.target.value })}>
                <option value="open_incident">open_incident</option>
                <option value="notify">notify</option>
                <option value="webhook">webhook</option>
              </select>
            </label>
            {form.action === "webhook" && (
              <label className="span-2">Webhook URL<input value={form.webhook_url} onChange={(e) => setForm({ ...form, webhook_url: e.target.value })} placeholder="https://example.com/hook" required /></label>
            )}
          </div>
          {err && <p className="form-error">{err}</p>}
          <div className="row-actions" style={{ marginTop: 12 }}>
            <button className="btn accent" type="submit">Create</button>
          </div>
        </form>
      )}
      <div className="card table-wrap">
        <table>
          <thead><tr><th>Name</th><th>Trigger</th><th>Rule</th><th>Action</th><th></th></tr></thead>
          <tbody>
            {rows.map((a) => (
              <tr key={a.id}>
                <td>{a.name}</td>
                <td>{a.trigger_kind}</td>
                <td>{a.capability} {a.operator} {a.threshold}</td>
                <td>{a.action}</td>
                <td className="row-actions">
                  <button className="btn small ghost" onClick={() => toggle(a)}>{a.enabled ? "Enabled" : "Paused"}</button>
                  <button className="btn small ghost" onClick={() => remove(a)}>Delete</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
