import { FormEvent, useEffect, useState, type ReactNode } from "react";
import { api, getToken } from "../lib/api";

type Place = { id: string; name: string; kind: string; parent_id?: string; floorplan?: string };
type Template = { id: string; name: string; kind: string };
type LinkRow = { id: string; from_asset_id: string; to_asset_id: string; relation: string };
type Me = { role: string };

const KINDS = ["region", "campus", "building", "floor", "zone"];

export default function Locations() {
  const [rows, setRows] = useState<Place[]>([]);
  const [templates, setTemplates] = useState<Template[]>([]);
  const [links, setLinks] = useState<LinkRow[]>([]);
  const [canWrite, setCanWrite] = useState(false);
  const [name, setName] = useState("");
  const [kind, setKind] = useState("building");
  const [parent, setParent] = useState("");
  const [tpl, setTpl] = useState("");
  const [msg, setMsg] = useState("");
  const [place, setPlace] = useState("");
  const [pins, setPins] = useState<{ id: string; name: string; floor_x?: number; floor_y?: number }[]>([]);
  const [planURL, setPlanURL] = useState("");

  async function load() {
    const [locs, tpls, lns, me] = await Promise.all([
      api<Place[]>("/api/v1/locations"),
      api<Template[]>("/api/v1/asset-templates"),
      api<LinkRow[]>("/api/v1/asset-links"),
      api<Me>("/api/v1/auth/me"),
    ]);
    setRows(locs);
    setTemplates(tpls);
    setLinks(lns);
    setCanWrite(me.role === "admin" || me.role === "operator");
  }
  useEffect(() => { load(); }, []);

  async function createPlace(e: FormEvent) {
    e.preventDefault();
    setMsg("");
    try {
      await api("/api/v1/locations", {
        method: "POST",
        body: JSON.stringify({ name: name.trim(), kind, parent_id: parent || undefined }),
      });
      setName("");
      await load();
    } catch (ex) {
      setMsg(ex instanceof Error ? ex.message : "create failed");
    }
  }

  async function createTemplate(e: FormEvent) {
    e.preventDefault();
    setMsg("");
    try {
      await api("/api/v1/asset-templates", { method: "POST", body: JSON.stringify({ name: tpl.trim(), kind: "equipment" }) });
      setTpl("");
      await load();
    } catch (ex) {
      setMsg(ex instanceof Error ? ex.message : "template failed");
    }
  }

  async function show(id: string) {
    setPlace(id);
    try {
      setPins(await api(`/api/v1/locations/${id}/pins`));
    } catch {
      setPins([]);
    }
    const row = rows.find((item) => item.id === id);
    if (!row?.floorplan) {
      setPlanURL("");
      return;
    }
    const res = await fetch(`/api/v1/locations/${id}/floorplan`, { headers: { Authorization: `Bearer ${getToken()}` } });
    if (!res.ok) {
      setPlanURL("");
      return;
    }
    setPlanURL(URL.createObjectURL(await res.blob()));
  }

  const byParent = (id?: string) => rows.filter((r) => (r.parent_id || "") === (id || ""));

  function tree(parentID: string | undefined, depth: number): ReactNode[] {
    return byParent(parentID).flatMap((row) => [
      <div key={row.id} style={{ paddingLeft: depth * 16, marginBottom: 6 }}>
        <button type="button" className="btn small ghost" onClick={() => void show(row.id)}>{row.name}</button> <span className="pill info">{row.kind}</span>
        {row.floorplan ? <span className="lede"> · plan {row.floorplan}</span> : null}
      </div>,
      ...tree(row.id, depth + 1),
    ]);
  }

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Locations</h1>
          <p className="lede">Region, campus, building, floor, and zone. Templates name a kind of asset. Links say how assets relate.</p>
        </div>
      </div>
      {msg && <p className="lede" style={{ marginBottom: 12 }}>{msg}</p>}
      <div className="grid" style={{ gridTemplateColumns: "1.2fr .8fr" }}>
        <div className="card">
          <h2>Places</h2>
          {rows.length ? tree(undefined, 0) : <p className="empty">No places yet.</p>}
          {canWrite && (
            <form onSubmit={createPlace} style={{ marginTop: 16 }}>
              <label>Name<input value={name} onChange={(e) => setName(e.target.value)} required /></label>
              <label>Kind
                <select value={kind} onChange={(e) => setKind(e.target.value)}>{KINDS.map((k) => <option key={k}>{k}</option>)}</select>
              </label>
              <label>Parent
                <select value={parent} onChange={(e) => setParent(e.target.value)}>
                  <option value="">None</option>
                  {rows.map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
                </select>
              </label>
              <button className="btn small accent" type="submit" style={{ marginTop: 8 }}>Add place</button>
            </form>
          )}
        </div>
        <div className="card">
          <h2>Templates</h2>
          {templates.map((t) => <p key={t.id}>{t.name} <span className="pill stale">{t.kind}</span></p>)}
          {!templates.length && <p className="empty">No templates yet.</p>}
          {canWrite && (
            <form onSubmit={createTemplate} style={{ marginTop: 12 }}>
              <label>Name<input value={tpl} onChange={(e) => setTpl(e.target.value)} required /></label>
              <button className="btn small ghost" type="submit" style={{ marginTop: 8 }}>Add template</button>
            </form>
          )}
          <h2 style={{ marginTop: 20 }}>Floor plan</h2>
          {!place && <p className="empty">Choose a place to see its pins.</p>}
          {planURL && <img src={planURL} alt="Floor plan" style={{ maxWidth: "100%", marginTop: 8 }} />}
          {place && pins.map((pin) => <p key={pin.id} className="lede">{pin.name} at {pin.floor_x ?? "—"}, {pin.floor_y ?? "—"}</p>)}
          {place && !pins.length && <p className="empty">No floor pins on this place.</p>}
          <h2 style={{ marginTop: 20 }}>Asset links</h2>
          {links.map((l) => <p key={l.id} className="lede">{l.relation}: {l.from_asset_id} → {l.to_asset_id}</p>)}
          {!links.length && <p className="empty">No links yet. Create them from the API with relation installed-on or depends-on.</p>}
        </div>
      </div>
    </>
  );
}
