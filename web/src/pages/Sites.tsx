import { FormEvent, useEffect, useState } from "react";
import { api, Site } from "../lib/api";

const KINDS = ["factory", "warehouse", "office", "customer", "site"];

export default function Sites() {
  const [rows, setRows] = useState<Site[]>([]);
  const [open, setOpen] = useState(false);
  const [err, setErr] = useState("");
  const [form, setForm] = useState({ name: "", kind: "factory", address: "", latitude: "", longitude: "" });

  async function load() {
    setRows(await api<Site[]>("/api/v1/sites"));
  }
  useEffect(() => { load(); }, []);

  async function create(e: FormEvent) {
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
      await api<Site>("/api/v1/sites", { method: "POST", body: JSON.stringify(body) });
      setForm({ name: "", kind: "factory", address: "", latitude: "", longitude: "" });
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
          <h1>Sites</h1>
          <p className="lede">Factories, warehouses, offices, and customer locations.</p>
        </div>
        <button className="btn accent" type="button" onClick={() => setOpen((v) => !v)}>
          {open ? "Cancel" : "Add site"}
        </button>
      </div>
      {open && (
        <form className="card form-card" onSubmit={create} style={{ marginBottom: 16 }}>
          <h2>New site</h2>
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
            <button className="btn accent" type="submit">Create site</button>
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
          </div>
        ))}
        {!rows.length && <p className="empty">No sites yet. Add the first location.</p>}
      </div>
    </>
  );
}
