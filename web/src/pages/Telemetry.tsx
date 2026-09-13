import { useEffect, useMemo, useState } from "react";
import { api, Telemetry } from "../lib/api";
import { fmt } from "../components/Shell";
import { useYardStream } from "../lib/stream";

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
      const k = r.asset_name;
      const arr = m.get(k) || [];
      arr.push(r);
      m.set(k, arr);
    }
    return [...m.entries()];
  }, [rows]);

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Telemetry</h1>
          <p className="lede">Measurements, freshness, and quality. Stale points never look healthy.</p>
        </div>
      </div>
      {grouped.slice(0, 6).map(([name, pts]) => (
        <div className="card" key={name} style={{ marginBottom: 12 }}>
          <h2>{name}</h2>
          <div className="spark" style={{ margin: "8px 0 12px" }}>
            {pts.slice(0, 24).map((p, i) => {
              const max = Math.max(...pts.map((x) => Math.abs(x.value)), 1);
              return <i key={i} title={`${p.capability} ${p.value}`} style={{ height: `${Math.max(8, (Math.abs(p.value) / max) * 36)}px` }} />;
            })}
          </div>
        </div>
      ))}
      <div className="card table-wrap">
        <table>
          <thead><tr><th>Asset</th><th>Signal</th><th>Value</th><th>Quality</th><th>Fresh</th><th>Observed</th><th>Source</th></tr></thead>
          <tbody>
            {rows.map((r, i) => (
              <tr key={i}>
                <td>{r.asset_name}</td>
                <td>{r.capability}</td>
                <td>{r.value.toFixed(2)} {r.unit}</td>
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
