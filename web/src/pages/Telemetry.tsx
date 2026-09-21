import { useEffect, useMemo, useState } from "react";
import { api, Observation, Telemetry } from "../lib/api";
import { fmt } from "../components/Shell";
import { useYardStream } from "../lib/stream";

const RANGE_PRESETS: { label: string; hours: number }[] = [
  { label: "1h", hours: 1 },
  { label: "24h", hours: 24 },
  { label: "7d", hours: 24 * 7 },
];

function showValue(p: { value: number; unit: string; value_kind?: string; value_text?: string }) {
  if (p.value_kind === "text" || p.value_kind === "bool") return p.value_text || "";
  return `${p.value.toFixed(2)} ${p.unit}`.trim();
}

export default function TelemetryPage() {
  const [rows, setRows] = useState<Telemetry[]>([]);

  async function load() {
    setRows(await api<Telemetry[]>("/api/v1/telemetry"));
  }
  useEffect(() => { load(); }, []);
  useYardStream((ev) => {
    if (ev.kind === "observation" || ev.kind === "asset.health") load();
  });

  const grouped = useMemo(() => {
    const m = new Map<string, Telemetry[]>();
    for (const r of rows) {
      const arr = m.get(r.asset_name) || [];
      arr.push(r);
      m.set(r.asset_name, arr);
    }
    return [...m.entries()];
  }, [rows]);

  const assets = useMemo(() => {
    const m = new Map<string, string>();
    for (const r of rows) m.set(r.asset_id, r.asset_name);
    return [...m.entries()].map(([id, name]) => ({ id, name }));
  }, [rows]);

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Telemetry</h1>
          <p className="lede">Measurements, freshness, and quality. Stale points never look healthy.</p>
        </div>
      </div>
      {grouped.map(([name, pts]) => (
        <div className="card" key={name} style={{ marginBottom: 12 }}>
          <h2>{name}</h2>
          <div className="signal-grid">
            {pts.map((p, i) => (
              <div className="signal-chip" key={i} title={`${p.capability} · ${fmt(p.observed_at)}`}>
                <span className="signal-name">{p.capability}</span>
                <span className="signal-value">{showValue(p)}</span>
              </div>
            ))}
          </div>
        </div>
      ))}
      <HistoryDashboard assets={assets} rows={rows} />
      <div className="card table-wrap">
        <table>
          <thead><tr><th>Asset</th><th>Signal</th><th>Value</th><th>Quality</th><th>Fresh</th><th>Observed</th><th>Source</th></tr></thead>
          <tbody>
            {rows.map((r, i) => (
              <tr key={i}>
                <td>{r.asset_name}</td>
                <td>{r.capability}</td>
                <td>{showValue(r)}</td>
                <td>{r.quality}</td>
                <td><span className={`pill ${r.fresh ? "ok" : "stale"}`}>{r.fresh ? "fresh" : "stale"}</span></td>
                <td>{fmt(r.observed_at)}</td>
                <td>{r.source}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {!rows.length && <p className="empty">No observations yet. Run the simulator.</p>}
      </div>
    </>
  );
}

function HistoryDashboard({ assets, rows }: { assets: { id: string; name: string }[]; rows: Telemetry[] }) {
  const [assetId, setAssetId] = useState("");
  const [capability, setCapability] = useState("");
  const [hours, setHours] = useState(24);
  const [points, setPoints] = useState<Observation[]>([]);

  const capabilities = useMemo(
    () => [...new Set(rows.filter((r) => r.asset_id === assetId).map((r) => r.capability))],
    [rows, assetId],
  );

  useEffect(() => {
    if (!assetId && assets.length) setAssetId(assets[0].id);
  }, [assets, assetId]);
  useEffect(() => {
    if (capabilities.length && !capabilities.includes(capability)) setCapability(capabilities[0]);
  }, [capabilities, capability]);

  useEffect(() => {
    if (!assetId || !capability) { setPoints([]); return; }
    const to = new Date();
    const from = new Date(to.getTime() - hours * 3600_000);
    api<Observation[]>(
      `/api/v1/assets/${assetId}/observations?capability=${encodeURIComponent(capability)}&from=${from.toISOString()}&to=${to.toISOString()}`,
    ).then(setPoints);
  }, [assetId, capability, hours]);

  const ordered = useMemo(
    () => [...points].filter((p) => !p.value_kind || p.value_kind === "number").reverse(),
    [points],
  );
  const max = Math.max(...ordered.map((p) => Math.abs(p.value)), 1);

  return (
    <div className="card" style={{ marginBottom: 12 }}>
      <div className="topbar" style={{ marginBottom: 8 }}>
        <h2>Signal history</h2>
        <div className="row-actions">
          <select value={assetId} onChange={(e) => setAssetId(e.target.value)}>
            {assets.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
          </select>
          <select value={capability} onChange={(e) => setCapability(e.target.value)}>
            {capabilities.map((c) => <option key={c} value={c}>{c}</option>)}
          </select>
          {RANGE_PRESETS.map((p) => (
            <button
              key={p.label}
              type="button"
              className={`btn small ${hours === p.hours ? "accent" : "ghost"}`}
              onClick={() => setHours(p.hours)}
            >
              {p.label}
            </button>
          ))}
        </div>
      </div>
      {ordered.length ? (
        <div className="spark" style={{ margin: "8px 0" }}>
          {ordered.map((p, i) => (
            <i key={i} title={`${p.value} ${p.unit} · ${fmt(p.observed_at)}`} style={{ height: `${Math.max(8, (Math.abs(p.value) / max) * 36)}px` }} />
          ))}
        </div>
      ) : points.length ? (
        <p className="empty">{showValue(points[0])}</p>
      ) : (
        <p className="empty">No observations for this asset and signal in the selected range.</p>
      )}
    </div>
  );
}
