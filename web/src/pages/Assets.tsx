import { FormEvent, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { api, Asset, EventItem, Site, WorkOrder } from "../lib/api";
import { fmt, Health } from "../components/Shell";

type Detail = {
  asset: Asset;
  capabilities: { name: string; unit: string }[];
  observations: { capability: string; value: number; unit: string; observed_at: string; quality: string; source: string }[];
  events?: EventItem[];
  work_orders?: WorkOrder[];
};

const KINDS = ["device", "sensor", "machine", "vehicle", "equipment"];

export default function Assets() {
  const [rows, setRows] = useState<Asset[]>([]);
  const [sites, setSites] = useState<Site[]>([]);
  const [q, setQ] = useState("");
  const [kind, setKind] = useState("");
  const [sel, setSel] = useState<Detail | null>(null);
  const [tab, setTab] = useState("Overview");
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState(false);
  const [err, setErr] = useState("");
  const [form, setForm] = useState({
    name: "", kind: "equipment", external_ref: "", manufacturer: "", model: "", serial: "",
    site_id: "", latitude: "", longitude: "", stale_after_sec: "90",
  });
  const [params] = useSearchParams();

  async function load() {
    const qs = new URLSearchParams();
    if (q) qs.set("q", q);
    if (kind) qs.set("kind", kind);
    const [a, s] = await Promise.all([
      api<Asset[]>(`/api/v1/assets?${qs}`),
      api<Site[]>("/api/v1/sites"),
    ]);
    setRows(a);
    setSites(s);
  }
  useEffect(() => { load(); }, [kind]);

  async function open(id: string) {
    setSel(await api<Detail>(`/api/v1/assets/${id}`));
    setTab("Overview");
    setEditing(false);
  }

  useEffect(() => {
    const focus = params.get("focus");
    if (focus) void open(focus);
  }, [params]);

  const kinds = useMemo(() => Array.from(new Set([...KINDS, ...rows.map((r) => r.kind)])), [rows]);

  function startCreate() {
    setForm({
      name: "", kind: "equipment", external_ref: "", manufacturer: "", model: "", serial: "",
      site_id: "", latitude: "", longitude: "", stale_after_sec: "90",
    });
    setFormOpen(true);
    setEditing(false);
    setErr("");
  }

  function startEdit() {
    if (!sel) return;
    const a = sel.asset;
    setForm({
      name: a.name,
      kind: a.kind,
      external_ref: a.external_ref || "",
      manufacturer: a.manufacturer || "",
      model: a.model || "",
      serial: a.serial || "",
      site_id: a.site_id || "",
      latitude: a.latitude != null ? String(a.latitude) : "",
      longitude: a.longitude != null ? String(a.longitude) : "",
      stale_after_sec: String(a.stale_after_sec || 90),
    });
    setEditing(true);
    setFormOpen(true);
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
      external_ref: form.external_ref.trim(),
      manufacturer: form.manufacturer.trim(),
      model: form.model.trim(),
      serial: form.serial.trim(),
      stale_after_sec: Number(form.stale_after_sec) || 90,
    };
    if (form.site_id) body.site_id = form.site_id;
    if (form.latitude) body.latitude = Number(form.latitude);
    if (form.longitude) body.longitude = Number(form.longitude);
    try {
      if (editing && sel) {
        await api(`/api/v1/assets/${sel.asset.id}`, { method: "PATCH", body: JSON.stringify(body) });
        await open(sel.asset.id);
      } else {
        const created = await api<Asset>("/api/v1/assets", { method: "POST", body: JSON.stringify(body) });
        await open(created.id);
      }
      setFormOpen(false);
      await load();
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : "save failed");
    }
  }

  async function remove() {
    if (!sel || !confirm(`Delete ${sel.asset.name}?`)) return;
    await api(`/api/v1/assets/${sel.asset.id}`, { method: "DELETE" });
    setSel(null);
    await load();
  }

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Assets</h1>
          <p className="lede">Devices, vehicles, machines, sensors, and other equipment.</p>
        </div>
        <div className="row-actions">
          <input className="search" placeholder="Search name, ref, serial" value={q} onChange={(e) => setQ(e.target.value)} onKeyDown={(e) => e.key === "Enter" && load()} />
          <select className="search" value={kind} onChange={(e) => setKind(e.target.value)}>
            <option value="">All kinds</option>
            {kinds.map((k) => <option key={k}>{k}</option>)}
          </select>
          <button type="button" className="btn accent" onClick={startCreate}>Add asset</button>
        </div>
      </div>
      {formOpen && (
        <form className="card form-card" onSubmit={save} style={{ marginBottom: 16 }}>
          <h2>{editing ? "Edit asset" : "New asset"}</h2>
          <div className="form-grid">
            <label>Name<input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></label>
            <label>Kind
              <select value={form.kind} onChange={(e) => setForm({ ...form, kind: e.target.value })}>
                {kinds.map((k) => <option key={k}>{k}</option>)}
              </select>
            </label>
            <label>External ref<input value={form.external_ref} onChange={(e) => setForm({ ...form, external_ref: e.target.value })} /></label>
            <label>Site
              <select value={form.site_id} onChange={(e) => setForm({ ...form, site_id: e.target.value })}>
                <option value="">None</option>
                {sites.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
              </select>
            </label>
            <label>Manufacturer<input value={form.manufacturer} onChange={(e) => setForm({ ...form, manufacturer: e.target.value })} /></label>
            <label>Model<input value={form.model} onChange={(e) => setForm({ ...form, model: e.target.value })} /></label>
            <label>Serial<input value={form.serial} onChange={(e) => setForm({ ...form, serial: e.target.value })} /></label>
            <label>Stale after (sec)<input type="number" value={form.stale_after_sec} onChange={(e) => setForm({ ...form, stale_after_sec: e.target.value })} /></label>
            <label>Latitude<input type="number" step="any" value={form.latitude} onChange={(e) => setForm({ ...form, latitude: e.target.value })} /></label>
            <label>Longitude<input type="number" step="any" value={form.longitude} onChange={(e) => setForm({ ...form, longitude: e.target.value })} /></label>
          </div>
          {err && <p className="form-error">{err}</p>}
          <div className="row-actions" style={{ marginTop: 12 }}>
            <button className="btn accent" type="submit">{editing ? "Save" : "Create"}</button>
            <button className="btn ghost" type="button" onClick={() => setFormOpen(false)}>Cancel</button>
          </div>
        </form>
      )}
      <div className="split">
        <div className="card table-wrap">
          <table>
            <thead><tr><th>Name</th><th>Kind</th><th>Health</th><th>Ref</th><th>Last seen</th></tr></thead>
            <tbody>
              {rows.map((a) => (
                <tr key={a.id} className={sel?.asset.id === a.id ? "selected" : ""} onClick={() => open(a.id)} style={{ cursor: "pointer" }}>
                  <td>{a.name}</td><td>{a.kind}</td><td><Health value={a.health} /></td><td>{a.external_ref}</td><td>{fmt(a.last_seen_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <aside className="panel">
          {!sel && <p className="empty">Select an asset. The list stays in place.</p>}
          {sel && (
            <>
              <p className="kicker">{sel.asset.kind}</p>
              <h1 style={{ fontSize: 22 }}>{sel.asset.name}</h1>
              <p className="lede">{sel.asset.manufacturer} {sel.asset.model} · {sel.asset.serial}</p>
              <div className="row-actions" style={{ margin: "8px 0" }}>
                <button type="button" className="btn small ghost" onClick={startEdit}>Edit</button>
                <button type="button" className="btn small ghost" onClick={remove}>Delete</button>
              </div>
              <div className="tabs">
                {["Overview", "Telemetry", "Activity", "Work", "Integrations"].map((t) => (
                  <button key={t} className={tab === t ? "active" : ""} onClick={() => setTab(t)}>{t}</button>
                ))}
              </div>
              {tab === "Overview" && (
                <div>
                  <p><Health value={sel.asset.health} /> · stale after {sel.asset.stale_after_sec}s</p>
                  <p className="lede">Capabilities</p>
                  {sel.capabilities.map((c) => <div key={c.name} className="pill" style={{ margin: 4 }}>{c.name} {c.unit}</div>)}
                  {(sel.asset.latitude != null && sel.asset.longitude != null) && (
                    <p className="lede" style={{ marginTop: 8 }}>{sel.asset.latitude.toFixed(5)}, {sel.asset.longitude.toFixed(5)}</p>
                  )}
                </div>
              )}
              {tab === "Telemetry" && (
                <>
                  <Spark obs={sel.observations} />
                  <table>
                    <tbody>
                      {sel.observations.slice(0, 12).map((o, i) => (
                        <tr key={i}><td>{o.capability}</td><td>{o.value.toFixed(2)} {o.unit}</td><td>{o.quality}</td></tr>
                      ))}
                    </tbody>
                  </table>
                </>
              )}
              {tab === "Activity" && (
                <div>
                  {(sel.events || []).length ? (sel.events || []).map((e) => (
                    <p key={e.id} style={{ margin: "8px 0", fontSize: 13 }}>
                      <strong>{e.title}</strong><br />
                      <span className="lede">{e.kind} · {e.severity} · {fmt(e.created_at)}</span>
                    </p>
                  )) : <p className="lede">No events for this asset yet.</p>}
                </div>
              )}
              {tab === "Work" && (
                <div>
                  {(sel.work_orders || []).length ? (
                    <table>
                      <tbody>
                        {(sel.work_orders || []).map((w) => (
                          <tr key={w.id}><td>{w.title}</td><td>{w.status}</td><td>{w.priority}</td></tr>
                        ))}
                      </tbody>
                    </table>
                  ) : <p className="lede">No work orders. Create one from Work orders or Incidents.</p>}
                </div>
              )}
              {tab === "Integrations" && (
                <p className="lede">Source of truth is the connector that last published inventory for ref {sel.asset.external_ref || "—"}.</p>
              )}
            </>
          )}
        </aside>
      </div>
    </>
  );
}

function Spark({ obs }: { obs: { capability: string; value: number }[] }) {
  const byCap = new Map<string, number[]>();
  for (const o of [...obs].reverse()) {
    const arr = byCap.get(o.capability) || [];
    if (arr.length < 24) arr.push(o.value);
    byCap.set(o.capability, arr);
  }
  const first = [...byCap.entries()][0];
  if (!first) return null;
  const [, vals] = first;
  const max = Math.max(...vals, 1);
  return (
    <div style={{ marginBottom: 12 }}>
      <p className="lede" style={{ marginBottom: 6 }}>{first[0]} history</p>
      <div className="spark">
        {vals.map((v, i) => <i key={i} style={{ height: `${Math.max(8, (v / max) * 36)}px` }} />)}
      </div>
    </div>
  );
}
