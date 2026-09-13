import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, Asset, Site } from "../lib/api";

type Item = { id: string; label: string; hint: string; to: string };

const ROUTES: Item[] = [
  { id: "r-home", label: "Overview", hint: "Dashboard", to: "/" },
  { id: "r-assets", label: "Assets", hint: "Registry", to: "/assets" },
  { id: "r-sites", label: "Sites", hint: "Locations", to: "/sites" },
  { id: "r-map", label: "Map", hint: "MapLibre", to: "/map" },
  { id: "r-telem", label: "Telemetry", hint: "Signals", to: "/telemetry" },
  { id: "r-work", label: "Work orders", hint: "Maintenance", to: "/work" },
  { id: "r-inc", label: "Incidents", hint: "Alerts", to: "/incidents" },
  { id: "r-auto", label: "Automations", hint: "Rules", to: "/automations" },
  { id: "r-int", label: "Integrations", hint: "Connectors", to: "/integrations" },
  { id: "r-admin", label: "Administration", hint: "Audit", to: "/admin" },
  { id: "r-diag", label: "Diagnostics", hint: "Logs", to: "/diagnostics" },
  { id: "r-set", label: "Settings", hint: "Theme", to: "/settings" },
  { id: "r-onb", label: "Get started", hint: "Onboarding", to: "/onboarding" },
];

export default function CommandPalette() {
  const [open, setOpen] = useState(false);
  const [q, setQ] = useState("");
  const [sel, setSel] = useState(0);
  const [assets, setAssets] = useState<Asset[]>([]);
  const [sites, setSites] = useState<Site[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);
  const nav = useNavigate();

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((v) => !v);
      }
      if (e.key === "Escape") setOpen(false);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  useEffect(() => {
    if (!open) {
      setQ("");
      setSel(0);
      return;
    }
    inputRef.current?.focus();
    Promise.all([api<Asset[]>("/api/v1/assets"), api<Site[]>("/api/v1/sites")])
      .then(([a, s]) => { setAssets(a); setSites(s); })
      .catch(() => {});
  }, [open]);

  const items = useMemo(() => {
    const extra: Item[] = [
      ...assets.slice(0, 40).map((a) => ({
        id: a.id, label: a.name, hint: `Asset · ${a.kind}`, to: `/assets?focus=${a.id}`,
      })),
      ...sites.map((s) => ({
        id: s.id, label: s.name, hint: `Site · ${s.kind}`, to: "/sites",
      })),
    ];
    const all = [...ROUTES, ...extra];
    const qq = q.trim().toLowerCase();
    if (!qq) return all.slice(0, 12);
    return all.filter((i) => i.label.toLowerCase().includes(qq) || i.hint.toLowerCase().includes(qq)).slice(0, 12);
  }, [q, assets, sites]);

  useEffect(() => { setSel(0); }, [q]);

  function go(item: Item) {
    setOpen(false);
    nav(item.to);
  }

  if (!open) return null;

  return (
    <div className="palette-backdrop" onClick={() => setOpen(false)} role="presentation">
      <div
        className="palette"
        role="dialog"
        aria-label="Command palette"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") { e.preventDefault(); setSel((i) => Math.min(i + 1, items.length - 1)); }
          if (e.key === "ArrowUp") { e.preventDefault(); setSel((i) => Math.max(i - 1, 0)); }
          if (e.key === "Enter" && items[sel]) go(items[sel]);
        }}
      >
        <input
          ref={inputRef}
          className="palette-input"
          placeholder="Go to page or asset… (⌘K)"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <ul className="palette-list">
          {items.map((item, i) => (
            <li key={item.id}>
              <button
                type="button"
                className={i === sel ? "active" : ""}
                onMouseEnter={() => setSel(i)}
                onClick={() => go(item)}
              >
                <span>{item.label}</span>
                <span className="hint">{item.hint}</span>
              </button>
            </li>
          ))}
          {!items.length && <li className="empty">No matches</li>}
        </ul>
      </div>
    </div>
  );
}
