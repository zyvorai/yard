import type { ReactNode } from "react";
import { NavLink, useLocation } from "react-router-dom";
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
  ["/settings", "Settings"],
];

function SignOutIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M10 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h4"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M16 8l4 4-4 4M9 12h11"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export default function Shell({ children }: { children: ReactNode }) {
  const loc = useLocation();
  const mapMode = loc.pathname === "/map";

  function signOut() {
    clearToken();
    window.location.href = "/login";
  }

  return (
    <div className="app">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">
            <img src="/logo.svg" width={32} height={32} alt="Zyvor" />
            <div>
              <div className="name">Yard</div>
              <div className="sub">zyvor.dev</div>
            </div>
          </div>
          <button
            type="button"
            className="icon-btn"
            aria-label="Sign out"
            title="Sign out"
            onClick={signOut}
          >
            <SignOutIcon />
          </button>
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
