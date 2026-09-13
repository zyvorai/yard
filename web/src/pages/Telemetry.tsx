import { useEffect, useState } from "react";
import { api, Telemetry } from "../lib/api";
import { fmt } from "../components/Shell";

export default function TelemetryPage() {
  const [rows, setRows] = useState<Telemetry[]>([]);
  useEffect(() => { api<Telemetry[]>("/api/v1/telemetry").then(setRows); }, []);
  return (
    <>
      <div className="topbar"><div><h1>Telemetry</h1><p className="lede">Measurements, freshness, and quality. Stale points never look healthy.</p></div></div>
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
