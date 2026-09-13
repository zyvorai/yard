import { useEffect, useMemo, useState } from "react";
import { api, Asset } from "../lib/api";
import { fmt, Health } from "../components/Shell";

type Detail = { asset: Asset; capabilities: { name: string; unit: string }[]; observations: { capability: string; value: number; unit: string; observed_at: string; quality: string; source: string }[] };

export default function Assets() {
  const [rows, setRows] = useState<Asset[]>([]);
  const [q, setQ] = useState("");
  const [kind, setKind] = useState("");
  const [sel, setSel] = useState<Detail | null>(null);
  const [tab, setTab] = useState("Overview");

  async function load() {
    const qs = new URLSearchParams();
    if (q) qs.set("q", q);
    if (kind) qs.set("kind", kind);
    setRows(await api<Asset[]>(`/api/v1/assets?${qs}`));
  }
  useEffect(() => { load(); }, [kind]);

  async function open(id: string) {
    setSel(await api<Detail>(`/api/v1/assets/${id}`));
    setTab("Overview");
  }

  const kinds = useMemo(() => Array.from(new Set(rows.map((r) => r.kind))), [rows]);

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
        </div>
      </div>
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
                </div>
              )}
              {tab === "Telemetry" && (
                <table>
                  <tbody>
                    {sel.observations.slice(0, 12).map((o, i) => (
                      <tr key={i}><td>{o.capability}</td><td>{o.value.toFixed(2)} {o.unit}</td><td>{o.quality}</td></tr>
                    ))}
                  </tbody>
                </table>
              )}
              {tab === "Activity" && <p className="lede">Events and audit entries for this asset appear on Incidents and Administration.</p>}
              {tab === "Work" && <p className="lede">Open a work order from Incidents when a repair or inspection is required.</p>}
              {tab === "Integrations" && <p className="lede">Source of truth for this asset is whatever connector last published inventory.</p>}
            </>
          )}
        </aside>
      </div>
    </>
  );
}
