import { FormEvent, useEffect, useState } from "react";
import { api, Asset, Observation } from "../lib/api";

type Panel = { asset_id: string; capability: string; title?: string };
type Board = { id: string; name: string; panels: Panel[] };
type Me = { role: string };

function show(p: Observation) {
  if (p.value_kind === "text" || p.value_kind === "bool") return p.value_text || "";
  return `${p.value.toFixed(2)} ${p.unit}`.trim();
}

export default function Dashboards() {
  const [boards, setBoards] = useState<Board[]>([]);
  const [assets, setAssets] = useState<Asset[]>([]);
  const [canWrite, setCanWrite] = useState(false);
  const [name, setName] = useState("");
  const [assetId, setAssetId] = useState("");
  const [capability, setCapability] = useState("");
  const [msg, setMsg] = useState("");

  async function load() {
    const [list, found, me] = await Promise.all([
      api<Board[]>("/api/v1/dashboards"),
      api<Asset[]>("/api/v1/assets"),
      api<Me>("/api/v1/auth/me"),
    ]);
    setBoards(list);
    setAssets(found);
    setAssetId((id) => id || found[0]?.id || "");
    setCanWrite(me.role === "admin" || me.role === "operator");
  }
  useEffect(() => { load(); }, []);

  async function create(e: FormEvent) {
    e.preventDefault();
    setMsg("");
    try {
      await api("/api/v1/dashboards", {
        method: "POST",
        body: JSON.stringify({
          name,
          panels: assetId && capability ? [{ asset_id: assetId, capability, title: capability }] : [],
        }),
      });
      setName("");
      setCapability("");
      await load();
    } catch (err) {
      setMsg(err instanceof Error ? err.message : "Could not save");
    }
  }

  async function remove(id: string) {
    await api("/api/v1/dashboards/" + id, { method: "DELETE" });
    await load();
  }

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Dashboards</h1>
          <p className="lede">Saved panels for the signals you watch. Each panel is one asset and one capability.</p>
        </div>
      </div>
      {canWrite && (
        <form className="card" onSubmit={create} style={{ marginBottom: 12 }}>
          <div className="row-actions">
            <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Board name" required />
            <select value={assetId} onChange={(e) => setAssetId(e.target.value)}>
              {assets.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
            </select>
            <input value={capability} onChange={(e) => setCapability(e.target.value)} placeholder="capability" />
            <button className="btn small accent" type="submit">Save</button>
          </div>
          {msg && <p className="lede">{msg}</p>}
        </form>
      )}
      {boards.map((b) => (
        <div className="card" key={b.id} style={{ marginBottom: 12 }}>
          <div className="topbar" style={{ marginBottom: 8 }}>
            <h2>{b.name}</h2>
            {canWrite && <button className="btn small ghost" type="button" onClick={() => remove(b.id)}>Delete</button>}
          </div>
          {!b.panels.length && <p className="empty">No panels on this board.</p>}
          <div className="signal-grid">
            {b.panels.map((p, i) => <PanelCard key={i} panel={p} assetName={assets.find((a) => a.id === p.asset_id)?.name || p.asset_id} />)}
          </div>
        </div>
      ))}
      {!boards.length && <p className="empty">No dashboards yet.</p>}
    </>
  );
}

function PanelCard({ panel, assetName }: { panel: Panel; assetName: string }) {
  const [rows, setRows] = useState<Observation[]>([]);
  useEffect(() => {
    const to = new Date();
    const from = new Date(to.getTime() - 24 * 3600_000);
    api<Observation[]>(
      `/api/v1/assets/${panel.asset_id}/observations?capability=${encodeURIComponent(panel.capability)}&from=${from.toISOString()}&to=${to.toISOString()}`,
    ).then(setRows).catch(() => setRows([]));
  }, [panel.asset_id, panel.capability]);
  const latest = rows[0];
  const ordered = [...rows].filter((p) => !p.value_kind || p.value_kind === "number").reverse();
  const max = Math.max(...ordered.map((p) => Math.abs(p.value)), 1);
  return (
    <div className="signal-chip" title={panel.capability}>
      <span className="signal-name">{assetName} · {panel.title || panel.capability}</span>
      <span className="signal-value">{latest ? show(latest) : "—"}</span>
      {ordered.length > 0 && (
        <div className="spark">
          {ordered.slice(-24).map((p, i) => (
            <i key={i} style={{ height: `${Math.max(8, (Math.abs(p.value) / max) * 28)}px` }} />
          ))}
        </div>
      )}
    </div>
  );
}
