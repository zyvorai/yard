import { useCallback, useEffect, useState } from "react";
import { api, Overview as Ov } from "../lib/api";
import { fmt } from "../components/Shell";
import { notifyCritical, useYardStream } from "../lib/stream";

export default function Overview() {
  const [data, setData] = useState<Ov | null>(null);
  const [err, setErr] = useState("");
  const [live, setLive] = useState(false);
  const [pulse, setPulse] = useState("");

  const refresh = useCallback(() => {
    api<Ov>("/api/v1/overview").then((d) => {
      setData(d);
      setErr("");
    }).catch((e) => setErr(String(e)));
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  useYardStream((ev) => {
    setLive(true);
    setPulse(ev.kind);
    if (ev.kind === "incident.opened") {
      const d = ev.data as { title?: string; severity?: string };
      if (d?.severity === "critical") {
        notifyCritical("Critical incident", d.title || "New critical incident");
      }
      refresh();
    }
    if (ev.kind === "assets.stale" || ev.kind === "asset.health" || ev.kind === "observation" || ev.kind === "workorder.created" || ev.kind === "automation.notify") {
      refresh();
    }
  });

  if (err) return <p>{err}</p>;
  if (!data) return <p className="lede">Loading workspace…</p>;
  return (
    <>
      <div className="topbar">
        <div>
          <h1>Overview</h1>
          <p className="lede">Asset health, active work, incidents, and recent activity.</p>
        </div>
        <div className="row-actions">
          {live && <span className="pill ok">Live{pulse ? ` · ${pulse}` : ""}</span>}
          <button type="button" className="btn ghost small" onClick={refresh}>Refresh</button>
        </div>
      </div>
      <div className="grid stats">
        <div className="card"><h2>Assets</h2><div className="metric">{data.assets_total}</div></div>
        <div className="card"><h2>Healthy</h2><div className="metric">{data.assets_healthy}<small>{data.assets_stale} stale</small></div></div>
        <div className="card"><h2>Open incidents</h2><div className="metric">{data.open_incidents}<small>{data.assets_critical} critical</small></div></div>
        <div className="card"><h2>Work orders</h2><div className="metric">{data.open_work_orders}<small>{data.active_connectors} connectors</small></div></div>
      </div>
      <div className="grid split" style={{ marginTop: 16 }}>
        <div className="card">
          <h2>Recent events</h2>
          {data.recent_events?.length ? (
            <table><tbody>
              {data.recent_events.map((e) => (
                <tr key={e.id}><td>{e.title}</td><td className="pill">{e.severity}</td><td>{fmt(e.created_at)}</td></tr>
              ))}
            </tbody></table>
          ) : <p className="empty">No events yet. Start the simulator.</p>}
        </div>
        <div className="card">
          <h2>Activity</h2>
          {data.recent_activity?.map((a) => (
            <p key={a.id} style={{ margin: "10px 0", fontSize: 13 }}>
              <strong>{a.action}</strong> · {a.actor}<br />
              <span className="lede">{a.detail || a.object}</span>
            </p>
          ))}
        </div>
      </div>
    </>
  );
}
