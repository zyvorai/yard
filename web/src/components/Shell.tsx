import type { ReactNode } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { clearToken } from "../lib/api";
import CommandPalette from "./CommandPalette";

function navIcon(d: string) {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d={d} stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

const NAV_ICONS: Record<string, ReactNode> = {
  "/": navIcon("M4 13h6V4H4v9ZM14 20h6v-9h-6v9ZM14 4v4h6V4h-6ZM4 20h6v-4H4v4Z"),
  "/assets": navIcon("M3 8l9-5 9 5-9 5-9-5ZM3 8v8l9 5 9-5V8M12 13v8"),
  "/sites": navIcon("M5 21V7l7-4 7 4v14M9 21v-6h6v6M9 11h.01M15 11h.01"),
  "/map": navIcon("M9 20l-6-2V4l6 2 6-2 6 2v14l-6-2-6 2ZM9 6v14M15 4v14"),
  "/telemetry": navIcon("M3 12h4l2 7 4-14 2 7h4"),
  "/work": navIcon("M9 4h6a1 1 0 0 1 1 1v2H8V5a1 1 0 0 1 1-1ZM5 7h14v13H5V7Z"),
  "/incidents": navIcon("M12 3l9 16H3l9-16ZM12 10v4M12 17.5v.01"),
  "/automations": navIcon("M13 2 4 14h6l-1 8 9-12h-6l1-8Z"),
  "/integrations": navIcon("M9 3v4M15 3v4M7 7h10a2 2 0 0 1 2 2v2a5 5 0 0 1-5 5h-4a5 5 0 0 1-5-5V9a2 2 0 0 1 2-2ZM12 16v5"),
  "/admin": navIcon("M12 3l7 3v5c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V6l7-3Z"),
  "/diagnostics": navIcon("M4 5h16v12H4V5ZM8 21h8M12 17v4M8 9l2 2-2 2M13 13h3"),
  "/settings": navIcon("M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6ZM19 12a7 7 0 0 0-.1-1.2l2-1.6-2-3.4-2.4 1a7 7 0 0 0-2-1.2L14 3h-4l-.5 2.6a7 7 0 0 0-2 1.2l-2.4-1-2 3.4 2 1.6a7 7 0 0 0 0 2.4l-2 1.6 2 3.4 2.4-1a7 7 0 0 0 2 1.2L10 21h4l.5-2.6a7 7 0 0 0 2-1.2l2.4 1 2-3.4-2-1.6c.07-.4.1-.8.1-1.2Z"),
  "/onboarding": navIcon("M5 21V4h11l3 4-3 4H5"),
};

const items: [string, string][] = [
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
  ["/onboarding", "Get started"],
];

const mobileItems = [
  ["/", "Home"],
  ["/assets", "Assets"],
  ["/map", "Map"],
  ["/work", "Work"],
  ["/settings", "More"],
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
              {NAV_ICONS[to]}
              {label}
            </NavLink>
          ))}
          <div className="sec">Platform</div>
          {items.slice(7).map(([to, label]) => (
            <NavLink key={to} to={to} className={({ isActive }) => (isActive ? "active" : "")}>
              {NAV_ICONS[to]}
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="foot">
          <div>Northwind Operations</div>
          <p className="lede" style={{ marginTop: 6, fontSize: 11 }}>⌘K command palette</p>
        </div>
      </aside>
      <main className={mapMode ? "main main--map" : "main"}>{children}</main>
      <nav className="mobile-nav" aria-label="Primary">
        {mobileItems.map(([to, label]) => (
          <NavLink key={to} to={to} end={to === "/"} className={({ isActive }) => (isActive ? "active" : "")}>
            {label}
          </NavLink>
        ))}
      </nav>
      <CommandPalette />
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
