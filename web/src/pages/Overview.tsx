import { useCallback, useEffect, useState } from "react";
import { api, Overview as Ov } from "../lib/api";
import { fmt } from "../components/Shell";
import { GroupedList, GroupedRow } from "../components/GroupedList";
import { notifyCritical, useYardStream } from "../lib/stream";

function EventIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M12 3l9 16H3l9-16ZM12 10v4M12 17.5v.01"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function ActivityIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M4 12h4l2 7 4-14 2 7h4"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function severityTone(sev?: string): "ok" | "warn" | "bad" | "info" | "stale" {
  switch (sev) {
    case "critical":
      return "bad";
    case "warning":
    case "degraded":
      return "warn";
    case "info":
      return "info";
    case "ok":
    case "healthy":
      return "ok";
    default:
      return "stale";
  }
}

export default function Overview() {
  const [data, setData] = useState<Ov | null>(null);
  const [err, setErr] = useState("");
  const [live, setLive] = useState(false);
  const [pulse, setPulse] = useState("");
  const [widgets, setWidgets] = useState<string[]>(["health", "incidents", "work", "activity"]);

  const refresh = useCallback(() => {
    api<Ov>("/api/v1/overview").then((d) => {
      setData(d);
      setErr("");
    }).catch((e) => setErr(String(e)));
  }, []);

  useEffect(() => { refresh(); }, [refresh]);
  useEffect(() => {
    api<{ widgets?: string[] }>("/api/v1/me/prefs").then((p) => {
      if (Array.isArray(p.widgets) && p.widgets.length) setWidgets(p.widgets);
    }).catch(() => {});
  }, []);

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
  const show = (name: string) => widgets.includes(name) || widgets.includes("health");
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
        {(widgets.includes("health") || widgets.length === 0) && <div className="card"><h2>Assets</h2><div className="metric">{data.assets_total}</div></div>}
        {widgets.includes("health") && <div className="card"><h2>Healthy</h2><div className="metric">{data.assets_healthy}<small>{data.assets_stale} stale</small></div></div>}
        {widgets.includes("incidents") && <div className="card"><h2>Open incidents</h2><div className="metric">{data.open_incidents}<small>{data.assets_critical} critical</small></div></div>}
        {widgets.includes("work") && <div className="card"><h2>Work orders</h2><div className="metric">{data.open_work_orders}<small>{data.active_connectors} connectors</small></div></div>}
      </div>
      <div className="grid split" style={{ marginTop: 16 }}>
        <div>
          {show("activity") && (
          <GroupedList title="Recent events">
            {data.recent_events?.length ? (
              data.recent_events.map((e) => (
                <GroupedRow
                  key={e.id}
                  icon={<EventIcon />}
                  tone={severityTone(e.severity)}
                  label={e.title}
                  description={fmt(e.created_at)}
                  trailing={<span className={`pill ${severityTone(e.severity)}`}>{e.severity}</span>}
                />
              ))
            ) : (
              <div className="settings-row">
                <span className="row-body">
                  <span className="row-description">
                    No events yet. Start the <a href="/onboarding">Get started</a> path or run the simulator.
                  </span>
                </span>
              </div>
            )}
          </GroupedList>
          )}
        </div>
        <div>
          {widgets.includes("activity") && (
          <GroupedList title="Activity">
            {data.recent_activity?.length ? (
              data.recent_activity.map((a) => (
                <GroupedRow
                  key={a.id}
                  icon={<ActivityIcon />}
                  tone="stale"
                  label={<>{a.action} <span className="row-description" style={{ display: "inline", fontWeight: 500 }}>· {a.actor}</span></>}
                  description={a.detail || a.object}
                />
              ))
            ) : (
              <div className="settings-row"><span className="row-body"><span className="row-description">No activity yet.</span></span></div>
            )}
          </GroupedList>
          )}
        </div>
      </div>
    </>
  );
}
