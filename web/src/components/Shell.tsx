import type { ReactNode } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { clearToken } from "../lib/api";
import { useTheme } from "../lib/theme";

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
  const loc = useLocation();
  const { theme, setTheme } = useTheme();
  const mapMode = loc.pathname === "/map";

  return (
    <div className="app">
      <aside className="sidebar">
        <div className="brand">
          <img src="/logo.svg" width={32} height={32} alt="Zyvor" />
          <div>
            <div className="name">Yard</div>
            <div className="sub">zyvor.dev</div>
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
          <div className="theme-toggle" role="group" aria-label="Theme">
            <button type="button" className={theme === "light" ? "active" : ""} onClick={() => setTheme("light")}>
              Light
            </button>
            <button type="button" className={theme === "dark" ? "active" : ""} onClick={() => setTheme("dark")}>
              Dark
            </button>
          </div>
          <button className="btn ghost small" style={{ marginTop: 4, width: "100%" }} onClick={() => { clearToken(); window.location.href = "/login"; }}>
            Sign out
          </button>
        </div>
      </aside>
      <main className={mapMode ? "main main--map" : "main"}>{children}</main>
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
