import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { api, Asset, downloadAuth, EventItem, getToken, Site, WorkOrder } from "../lib/api";
import { fmt, Health } from "../components/Shell";
import { GroupedList, GroupedRow } from "../components/GroupedList";
import LocationPicker from "../components/LocationPicker";

type Cap = { id?: string; name: string; kind?: string; unit: string; min?: number; max?: number; writable?: boolean };

type Detail = {
  asset: Asset;
  capabilities: Cap[];
  observations: { capability: string; value: number; unit: string; observed_at: string; quality: string; source: string }[];
  events?: EventItem[];
  work_orders?: WorkOrder[];
};

type FileRow = { id: string; name: string; size_bytes: number };
const KINDS = ["device", "sensor", "machine", "vehicle", "equipment"];
const CUSTOM = "__custom__";

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
    name: "", kind: "equipment", custom_kind: "", external_ref: "", manufacturer: "", model: "", serial: "",
    site_id: "", latitude: "", longitude: "", stale_after_sec: "90",
  });
  const [pickOpen, setPickOpen] = useState(false);
  const [capEdit, setCapEdit] = useState(false);
  const [capRows, setCapRows] = useState<Cap[]>([]);
  const [files, setFiles] = useState<FileRow[]>([]);
  const [labelURL, setLabelURL] = useState("");
  const [code, setCode] = useState("");
  const [params] = useSearchParams();
  const [ioMsg, setIoMsg] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);

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
    if (labelURL) URL.revokeObjectURL(labelURL);
    setLabelURL("");
    setSel(await api<Detail>(`/api/v1/assets/${id}`));
    setFiles(await api<FileRow[]>(`/api/v1/assets/${id}/attachments`));
    setTab("Overview");
    setEditing(false);
  }

  async function showLabel() {
    if (!sel) return;
    const res = await fetch(`/api/v1/assets/${sel.asset.id}/label`, {
      headers: { Authorization: `Bearer ${getToken()}` },
    });
    if (!res.ok) {
      setErr("Could not load the label");
      return;
    }
    const blob = await res.blob();
    if (labelURL) URL.revokeObjectURL(labelURL);
    setLabelURL(URL.createObjectURL(blob));
  }

  async function uploadFile(file: File) {
    if (!sel) return;
    setErr("");
    const body = new FormData();
    body.append("file", file);
    const res = await fetch(`/api/v1/assets/${sel.asset.id}/attachments`, {
      method: "POST",
      headers: { Authorization: `Bearer ${getToken()}` },
      body,
    });
    if (!res.ok) {
      setErr(await res.text());
      return;
    }
    setFiles(await api<FileRow[]>(`/api/v1/assets/${sel.asset.id}/attachments`));
  }

  async function downloadFile(file: FileRow) {
    if (!sel) return;
    const res = await fetch(`/api/v1/assets/${sel.asset.id}/attachments/${file.id}`, {
      headers: { Authorization: `Bearer ${getToken()}` },
    });
    if (!res.ok) {
      setErr("Could not download the file");
      return;
    }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = file.name;
    a.click();
    URL.revokeObjectURL(url);
  }

  async function findCode(e: FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      const asset = await api<Asset>(`/api/v1/assets/lookup?q=${encodeURIComponent(code.trim())}`);
      setCode("");
      await open(asset.id);
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : "No asset for that code");
    }
  }

  useEffect(() => {
    const focus = params.get("focus");
    if (focus) void open(focus);
  }, [params]);

  const kinds = useMemo(() => Array.from(new Set([...KINDS, ...rows.map((r) => r.kind)])), [rows]);

  function startCreate() {
    setForm({
      name: "", kind: "equipment", custom_kind: "", external_ref: "", manufacturer: "", model: "", serial: "",
      site_id: "", latitude: "", longitude: "", stale_after_sec: "90",
    });
    setFormOpen(true);
    setEditing(false);
    setPickOpen(false);
    setErr("");
  }

  function startEdit() {
    if (!sel) return;
    const a = sel.asset;
    const known = KINDS.includes(a.kind);
    setForm({
      name: a.name,
      kind: known ? a.kind : CUSTOM,
      custom_kind: known ? "" : a.kind,
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
    setPickOpen(false);
    setErr("");
  }

  function resolvedKind() {
    return form.kind === CUSTOM ? form.custom_kind.trim() || "equipment" : form.kind;
  }

  function locateMe() {
    if (!navigator.geolocation) {
      setErr("Geolocation is not supported by this browser.");
      return;
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setForm((f) => ({ ...f, latitude: String(pos.coords.latitude), longitude: String(pos.coords.longitude) }));
        setErr("");
      },
      (geoErr) => setErr(`Location unavailable: ${geoErr.message}`),
      { enableHighAccuracy: true, timeout: 10000 }
    );
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
      kind: resolvedKind(),
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

  function startCapEdit() {
    if (!sel) return;
    setCapRows(sel.capabilities.map((c) => ({ ...c })));
    setCapEdit(true);
  }

  async function saveCaps(e: FormEvent) {
    e.preventDefault();
    if (!sel) return;
    const body = capRows
      .filter((c) => c.name.trim())
      .map((c) => ({
        name: c.name.trim(),
        kind: c.kind || "measurement",
        unit: c.unit || "",
        min: c.min,
        max: c.max,
        writable: !!c.writable,
      }));
    await api(`/api/v1/assets/${sel.asset.id}/capabilities`, { method: "PUT", body: JSON.stringify(body) });
    setCapEdit(false);
    await open(sel.asset.id);
  }

  async function remove() {
    if (!sel || !confirm(`Delete ${sel.asset.name}?`)) return;
    await api(`/api/v1/assets/${sel.asset.id}`, { method: "DELETE" });
    setSel(null);
    await load();
  }

  async function exportAssets(format: "json" | "csv") {
    setIoMsg("");
    try {
      const qs = new URLSearchParams({ format });
      if (q) qs.set("q", q);
      if (kind) qs.set("kind", kind);
      await downloadAuth(`/api/v1/assets/export?${qs}`, `assets.${format}`);
      setIoMsg(`Exported ${format.toUpperCase()}.`);
    } catch (ex) {
      setIoMsg(ex instanceof Error ? ex.message : "export failed");
    }
  }

  async function importFile(file: File) {
    setIoMsg("");
    const isCSV = file.name.toLowerCase().endsWith(".csv") || file.type.includes("csv");
    try {
      const text = await file.text();
      const res = await api<{ created: number; updated: number }>(`/api/v1/assets/import?format=${isCSV ? "csv" : "json"}`, {
        method: "POST",
        headers: { "Content-Type": isCSV ? "text/csv" : "application/json" },
        body: text,
      });
      setIoMsg(`Import: ${res.created} created, ${res.updated} updated.`);
      await load();
    } catch (ex) {
      setIoMsg(ex instanceof Error ? ex.message : "import failed");
    }
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
          <button type="button" className="btn ghost" onClick={() => exportAssets("csv")}>Export CSV</button>
          <button type="button" className="btn ghost" onClick={() => exportAssets("json")}>Export JSON</button>
          <button type="button" className="btn ghost" onClick={() => fileRef.current?.click()}>Import</button>
          <input
            ref={fileRef}
            type="file"
            accept=".csv,.json,application/json,text/csv"
            hidden
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) void importFile(f);
              e.target.value = "";
            }}
          />
          <button type="button" className="btn accent" onClick={startCreate}>Add asset</button>
        </div>
      </div>
      {ioMsg && <p className="lede" style={{ marginBottom: 12 }}>{ioMsg}</p>}
      <form className="row-actions" onSubmit={findCode} style={{ marginBottom: 12 }}>
        <input className="search" placeholder="Label code or yard:asset:…" value={code} onChange={(e) => setCode(e.target.value)} />
        <button className="btn small ghost" type="submit">Find label</button>
      </form>
      {formOpen && (
        <form className="card form-card" onSubmit={save} style={{ marginBottom: 16 }}>
          <h2>{editing ? "Edit asset" : "New asset"}</h2>
          <div className="form-grid">
            <label>Name<input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></label>
            <label>Kind
              <select value={form.kind} onChange={(e) => setForm({ ...form, kind: e.target.value })}>
                {kinds.map((k) => <option key={k} value={k}>{k}</option>)}
                <option value={CUSTOM}>Custom…</option>
              </select>
            </label>
            {form.kind === CUSTOM && (
              <label>Custom kind<input value={form.custom_kind} onChange={(e) => setForm({ ...form, custom_kind: e.target.value })} placeholder="e.g. pump-skid" required /></label>
            )}
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
            <div className="location-toggle-row" style={{ gridColumn: "1 / -1" }}>
              <button type="button" className="btn small ghost" onClick={locateMe}>Use my location</button>
              <button type="button" className="btn small ghost" onClick={() => setPickOpen((v) => !v)}>
                {pickOpen ? "Hide map" : "Pick on map"}
              </button>
            </div>
            {pickOpen && (
              <div style={{ gridColumn: "1 / -1" }}>
                <LocationPicker
                  latitude={form.latitude ? Number(form.latitude) : undefined}
                  longitude={form.longitude ? Number(form.longitude) : undefined}
                  onChange={(lat, lng) => setForm((f) => ({ ...f, latitude: String(lat), longitude: String(lng) }))}
                />
              </div>
            )}
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
          {!rows.length && (
            <p className="empty">
              No assets yet.{" "}
              <button type="button" className="btn small accent" onClick={startCreate}>Add asset</button>
              {" "}or finish <a href="/onboarding">Get started</a>.
            </p>
          )}
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
                <button type="button" className="btn small ghost" onClick={showLabel}>Label</button>
                <button type="button" className="btn small ghost" onClick={remove}>Delete</button>
              </div>
              {labelURL && (
                <div className="card" style={{ marginBottom: 12 }}>
                  <img src={labelURL} alt={`QR label for ${sel.asset.name}`} width={180} height={180} />
                  <p className="lede">{sel.asset.name}</p>
                  <p className="lede">{sel.asset.serial || sel.asset.external_ref || sel.asset.id}</p>
                </div>
              )}
              <div style={{ margin: "8px 0 12px" }}>
                <p className="kicker">Files</p>
                {files.map((f) => (
                  <button key={f.id} type="button" className="btn small ghost" style={{ marginRight: 8, marginTop: 6 }} onClick={() => downloadFile(f)}>
                    {f.name}
                  </button>
                ))}
                {!files.length && <p className="lede">No manuals or photos yet.</p>}
                <label className="btn small ghost" style={{ marginTop: 8, display: "inline-block" }}>
                  Add file
                  <input type="file" hidden onChange={(e) => { const f = e.target.files?.[0]; if (f) void uploadFile(f); e.target.value = ""; }} />
                </label>
              </div>
              <div className="tabs">
                {["Overview", "Telemetry", "Activity", "Work", "Integrations"].map((t) => (
                  <button key={t} className={tab === t ? "active" : ""} onClick={() => setTab(t)}>{t}</button>
                ))}
              </div>
              {tab === "Overview" && (
                <div>
                  <p><Health value={sel.asset.health} /> · stale after {sel.asset.stale_after_sec}s</p>
                  <ScoreLine id={sel.asset.id} />
                  {sel.asset.desired_state && sel.asset.desired_state !== "{}" && (
                    <p className="lede">Desired {sel.asset.desired_state}</p>
                  )}
                  <div className="row-actions" style={{ margin: "8px 0", justifyContent: "space-between" }}>
                    <p className="lede" style={{ margin: 0 }}>Capabilities</p>
                    <button type="button" className="btn small ghost" onClick={startCapEdit}>Edit</button>
                  </div>
                  {capEdit ? (
                    <form onSubmit={saveCaps}>
                      {capRows.map((c, i) => (
                        <div key={i} className="form-grid" style={{ marginBottom: 8, gridTemplateColumns: "1fr 1fr" }}>
                          <input placeholder="name" value={c.name} onChange={(e) => {
                            const next = [...capRows]; next[i] = { ...c, name: e.target.value }; setCapRows(next);
                          }} />
                          <input placeholder="unit" value={c.unit} onChange={(e) => {
                            const next = [...capRows]; next[i] = { ...c, unit: e.target.value }; setCapRows(next);
                          }} />
                          <input type="number" step="any" placeholder="min" value={c.min ?? ""} onChange={(e) => {
                            const next = [...capRows]; next[i] = { ...c, min: e.target.value === "" ? undefined : Number(e.target.value) }; setCapRows(next);
                          }} />
                          <input type="number" step="any" placeholder="max" value={c.max ?? ""} onChange={(e) => {
                            const next = [...capRows]; next[i] = { ...c, max: e.target.value === "" ? undefined : Number(e.target.value) }; setCapRows(next);
                          }} />
                        </div>
                      ))}
                      <div className="row-actions">
                        <button type="button" className="btn small ghost" onClick={() => setCapRows([...capRows, { name: "", unit: "" }])}>Add signal</button>
                        <button type="submit" className="btn small accent">Save</button>
                        <button type="button" className="btn small ghost" onClick={() => setCapEdit(false)}>Cancel</button>
                      </div>
                    </form>
                  ) : sel.capabilities.length ? (
                    <GroupedList>
                      {sel.capabilities.map((c) => (
                        <GroupedRow
                          key={c.name}
                          tone="info"
                          icon={c.writable ? "✎" : "◔"}
                          label={c.name}
                          description={
                            c.min != null && c.max != null
                              ? `${c.unit || "—"} · ${c.min}–${c.max}`
                              : c.min != null
                              ? `${c.unit || "—"} · min ${c.min}`
                              : c.max != null
                              ? `${c.unit || "—"} · max ${c.max}`
                              : c.unit || undefined
                          }
                        />
                      ))}
                    </GroupedList>
                  ) : (
                    <p className="lede">No capabilities yet. Edit to add signals.</p>
                  )}
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
                (sel.events || []).length ? (
                  <GroupedList>
                    {(sel.events || []).map((e) => (
                      <GroupedRow
                        key={e.id}
                        tone={e.severity === "critical" ? "bad" : e.severity === "warning" ? "warn" : "info"}
                        icon="●"
                        label={e.title}
                        description={`${e.kind} · ${e.severity} · ${fmt(e.created_at)}`}
                      />
                    ))}
                  </GroupedList>
                ) : <p className="lede">No events for this asset yet.</p>
              )}
              {tab === "Work" && (
                (sel.work_orders || []).length ? (
                  <GroupedList>
                    {(sel.work_orders || []).map((w) => (
                      <GroupedRow
                        key={w.id}
                        tone={w.status === "done" ? "ok" : "accent"}
                        icon={w.status === "done" ? "✓" : "▢"}
                        label={w.title}
                        description={`${w.status} · ${w.priority} priority`}
                      />
                    ))}
                  </GroupedList>
                ) : <p className="lede">No work orders. Create one from Work orders or Incidents.</p>
              )}
              {tab === "Integrations" && (
                <IntegrationsPanel assetId={sel.asset.id} externalRef={sel.asset.external_ref || "—"} />
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

function IntegrationsPanel({ assetId, externalRef }: { assetId: string; externalRef: string }) {
  const [data, setData] = useState<Record<string, unknown> | null>(null);
  const [err, setErr] = useState("");
  useEffect(() => {
    api<Record<string, unknown>>(`/api/v1/assets/${assetId}/integrations`)
      .then(setData)
      .catch((ex) => setErr(ex instanceof Error ? ex.message : "failed"));
  }, [assetId]);
  if (err) return <p className="lede">{err}</p>;
  if (!data) return <p className="lede">Loading connector state…</p>;
  return (
    <div>
      <p className="lede">External ref {externalRef}. Sync Nodra / Fleet / OTA from Integrations.</p>
      {(["nodra", "fleet", "ota"] as const).map((k) => (
        <div key={k} style={{ marginTop: 12 }}>
          <h3 style={{ textTransform: "uppercase", fontSize: 12, letterSpacing: "0.06em" }}>{k}</h3>
          {data[k] ? (
            <pre style={{ whiteSpace: "pre-wrap", fontSize: 12, maxHeight: 180, overflow: "auto" }}>
              {JSON.stringify(data[k], null, 2)}
            </pre>
          ) : (
            <p className="lede">No {k} data yet.</p>
          )}
        </div>
      ))}
    </div>
  );
}

function ScoreLine({ id }: { id: string }) {
  const [score, setScore] = useState<number | null>(null);
  useEffect(() => {
    let stop = false;
    api<{ score: number }>(`/api/v1/assets/${id}/score`)
      .then((row) => { if (!stop) setScore(row.score); })
      .catch(() => { if (!stop) setScore(null); });
    return () => { stop = true; };
  }, [id]);
  if (score == null) return null;
  return <p className="lede">Health score {score}</p>;
}
