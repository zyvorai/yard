import { FormEvent, useEffect, useState } from "react";
import { api, Site } from "../lib/api";

const KINDS = ["factory", "warehouse", "office", "customer", "site"];

const empty = { name: "", kind: "factory", address: "", latitude: "", longitude: "" };

export default function Sites() {
  const [rows, setRows] = useState<Site[]>([]);
  const [open, setOpen] = useState(false);
  const [editId, setEditId] = useState<string | null>(null);
  const [err, setErr] = useState("");
  const [form, setForm] = useState(empty);

  async function load() {
    setRows(await api<Site[]>("/api/v1/sites"));
  }
  useEffect(() => { load(); }, []);

  function startEdit(s: Site) {
    setEditId(s.id);
    setForm({
      name: s.name,
      kind: s.kind,
      address: s.address || "",
      latitude: s.latitude != null ? String(s.latitude) : "",
      longitude: s.longitude != null ? String(s.longitude) : "",
    });
    setOpen(true);
    setErr("");
  }

  async function save(e: FormEvent) {
    e.preventDefault();
    setErr("");
    if (!form.name.trim()) {
      setErr("Name is required");
      return;
    }
    const body: Record<string, unknown> = {
      name: form.name.trim(),
      kind: form.kind,
      address: form.address.trim(),
      timezone: "UTC",
    };
    if (form.latitude) body.latitude = Number(form.latitude);
    if (form.longitude) body.longitude = Number(form.longitude);
    try {
      if (editId) {
        await api(`/api/v1/sites/${editId}`, { method: "PATCH", body: JSON.stringify(body) });
      } else {
        await api<Site>("/api/v1/sites", { method: "POST", body: JSON.stringify(body) });
      }
      setForm(empty);
      setEditId(null);
      setOpen(false);
      await load();
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : "save failed");
    }
  }

  async function remove(s: Site) {
    if (!confirm(`Delete site ${s.name}?`)) return;
    await api(`/api/v1/sites/${s.id}`, { method: "DELETE" });
    await load();
  }

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Sites</h1>
          <p className="lede">Factories, warehouses, offices, and customer locations.</p>
        </div>
        <button className="btn accent" type="button" onClick={() => { setOpen((v) => !v); setEditId(null); setForm(empty); setErr(""); }}>
          {open && !editId ? "Cancel" : "Add site"}
        </button>
      </div>
      {open && (
        <form className="card form-card" onSubmit={save} style={{ marginBottom: 16 }}>
          <h2>{editId ? "Edit site" : "New site"}</h2>
          <div className="form-grid">
            <label>Name<input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></label>
            <label>Kind
              <select value={form.kind} onChange={(e) => setForm({ ...form, kind: e.target.value })}>
                {KINDS.map((k) => <option key={k} value={k}>{k}</option>)}
              </select>
            </label>
            <label className="span-2">Address<input value={form.address} onChange={(e) => setForm({ ...form, address: e.target.value })} /></label>
            <label>Latitude<input type="number" step="any" value={form.latitude} onChange={(e) => setForm({ ...form, latitude: e.target.value })} /></label>
            <label>Longitude<input type="number" step="any" value={form.longitude} onChange={(e) => setForm({ ...form, longitude: e.target.value })} /></label>
          </div>
          {err && <p className="form-error">{err}</p>}
          <div className="row-actions" style={{ marginTop: 12 }}>
            <button className="btn accent" type="submit">{editId ? "Save" : "Create site"}</button>
            {editId && <button className="btn ghost" type="button" onClick={() => { setOpen(false); setEditId(null); }}>Cancel</button>}
          </div>
        </form>
      )}
      <div className="grid" style={{ gridTemplateColumns: "repeat(auto-fill,minmax(260px,1fr))" }}>
        {rows.map((s) => (
          <div className="card" key={s.id}>
            <h2>{s.kind}</h2>
            <div style={{ fontSize: 20, fontWeight: 650 }}>{s.name}</div>
            <p className="lede">{s.address || "No address"}</p>
            {(s.latitude != null && s.longitude != null) && (
              <p className="lede">{s.latitude.toFixed(4)}, {s.longitude.toFixed(4)}</p>
            )}
            <div className="row-actions" style={{ marginTop: 10 }}>
              <button type="button" className="btn small ghost" onClick={() => startEdit(s)}>Edit</button>
              <button type="button" className="btn small ghost" onClick={() => remove(s)}>Delete</button>
            </div>
          </div>
        ))}
        {!rows.length && <p className="empty">No sites yet. Add the first location.</p>}
      </div>
    </>
  );
}
