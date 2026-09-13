import type { ReactNode } from "react";
import { NavLink } from "react-router-dom";
import { clearToken } from "../lib/api";

const items = [
  ["/", "Overview"],
  ["/assets", "Assets"],
  ["/sites", "Sites"],
  ["/map", "Map"],
  ["/telemetry", "Telemetry"],
  ["/work", "Work orders"],
  ["/incidents", "Incidents"],
  ["/automations", "Automations"],
  ["/integrations", "Integrations"],
  ["/admin", "Administration"],
  ["/diagnostics", "Diagnostics"],
];

export default function Shell({ children }: { children: ReactNode }) {
  return (
    <div className="app">
      <aside className="sidebar">
        <div className="brand">
          <img src="/logo.svg" alt="Zyvor" />
          <div>
            <div className="name">Estate</div>
            <div className="sub">Zyvor operations</div>
          </div>
        </div>
        <nav className="nav">
          {items.slice(0, 7).map(([to, label]) => (
            <NavLink key={to} to={to} end={to === "/"} className={({ isActive }) => (isActive ? "active" : "")}>
              {label}
            </NavLink>
          ))}
          <div className="sec">Platform</div>
          {items.slice(7).map(([to, label]) => (
            <NavLink key={to} to={to} className={({ isActive }) => (isActive ? "active" : "")}>
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="foot">
          <div>Northwind Operations</div>
          <button className="btn ghost small" style={{ marginTop: 8 }} onClick={() => { clearToken(); window.location.href = "/login"; }}>
            Sign out
          </button>
        </div>
      </aside>
      <main className="main">{children}</main>
    </div>
  );
}

export function Health({ value }: { value: string }) {
  const cls = value === "healthy" ? "ok" : value === "degraded" || value === "warning" ? "warn" : value === "critical" ? "bad" : "stale";
  return <span className={`pill ${cls}`}>{value}</span>;
}

export function fmt(ts?: string) {
  if (!ts) return "—";
  const d = new Date(ts);
  if (Number.isNaN(d.getTime())) return ts;
  return d.toLocaleString();
}
