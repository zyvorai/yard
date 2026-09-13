import { lazy, Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { api, Asset, Site } from "../lib/api";
import { Health, fmt } from "../components/Shell";
import type { MapPin } from "../components/YardMap";
import { notifyCritical, useYardStream } from "../lib/stream";

const YardMap = lazy(() => import("../components/YardMap"));

type KindFilter = "all" | "assets" | "sites";
type HealthFilter = "all" | "attention";

export default function MapPage() {
  const [assets, setAssets] = useState<Asset[]>([]);
  const [sites, setSites] = useState<Site[]>([]);
  const [kind, setKind] = useState<KindFilter>("all");
  const [health, setHealth] = useState<HealthFilter>("all");
  const [q, setQ] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const reload = useCallback(() => {
    Promise.all([
      api<Asset[]>("/api/v1/assets"),
      api<Site[]>("/api/v1/sites"),
    ]).then(([a, s]) => {
      setAssets(a);
      setSites(s);
    });
  }, []);

  useEffect(() => { reload(); }, [reload]);

  useYardStream((ev) => {
    if (ev.kind === "asset.health" || ev.kind === "assets.stale" || ev.kind === "observation" || ev.kind === "inventory" || ev.kind === "site.created" || ev.kind === "site.updated") {
      reload();
    }
    if (ev.kind === "incident.opened") {
      const d = ev.data as { title?: string; severity?: string };
      if (d?.severity === "critical") notifyCritical("Critical incident", d.title || "New incident");
      reload();
    }
  });

  const pins: MapPin[] = useMemo(() => {
    const out: MapPin[] = [];
    const query = q.trim().toLowerCase();

    if (kind !== "sites") {
      for (const a of assets) {
        if (a.latitude == null || a.longitude == null) continue;
        if (health === "attention" && a.health !== "critical" && a.health !== "stale" && a.health !== "degraded") continue;
        if (query) {
          const hay = `${a.name} ${a.kind} ${a.external_ref} ${a.serial}`.toLowerCase();
          if (!hay.includes(query)) continue;
        }
        out.push({
          id: a.id,
          name: a.name,
          kind: a.kind,
          health: a.health,
          latitude: a.latitude,
          longitude: a.longitude,
          entity: "asset",
          last_seen_at: a.last_seen_at,
          external_ref: a.external_ref,
        });
      }
    }

    if (kind !== "assets") {
      for (const s of sites) {
        if (s.latitude == null || s.longitude == null) continue;
        if (health === "attention") continue;
        if (query) {
          const hay = `${s.name} ${s.kind} ${s.address || ""}`.toLowerCase();
          if (!hay.includes(query)) continue;
        }
        out.push({
          id: s.id,
          name: s.name,
          kind: s.kind,
          latitude: s.latitude,
          longitude: s.longitude,
          entity: "site",
        });
      }
    }
    return out;
  }, [assets, sites, kind, health, q]);

  const selected = pins.find((p) => p.id === selectedId) || null;
  const missingCoords = assets.filter((a) => a.latitude == null || a.longitude == null).length;
  const attention = assets.filter((a) => a.health === "critical" || a.health === "stale" || a.health === "degraded").length;

  useEffect(() => {
    if (selectedId && !pins.some((p) => p.id === selectedId)) {
      setSelectedId(null);
    }
  }, [pins, selectedId]);

  return (
    <div className="map-stage">
      <Suspense fallback={<div className="map-canvas" style={{ background: "var(--bg)" }} />}>
        <YardMap
          pins={pins}
          selectedId={selectedId}
          onSelect={(pin) => setSelectedId(pin.id)}
        />
      </Suspense>

      <aside className="map-panel map-glass">
        <h1>Map</h1>
        <p className="lede">{pins.length} locations · {attention} need attention</p>
        <input
          className="map-search"
          placeholder="Search name, kind, ref…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          aria-label="Search map"
        />
        <div className="map-segments" role="group" aria-label="Entity filter">
          {([
            ["all", "All"],
            ["assets", "Assets"],
            ["sites", "Sites"],
          ] as const).map(([id, label]) => (
            <button key={id} type="button" className={kind === id ? "active" : ""} onClick={() => setKind(id)}>
              {label}
            </button>
          ))}
        </div>
        <div className="map-chips" role="group" aria-label="Health filter">
          <button type="button" className={health === "all" ? "active" : ""} onClick={() => setHealth("all")}>
            All health
          </button>
          <button type="button" className={health === "attention" ? "active" : ""} onClick={() => setHealth("attention")}>
            Attention ({attention})
          </button>
          {missingCoords > 0 && <span className="pill stale">{missingCoords} without coords</span>}
        </div>
        <div className="map-list">
          {pins.map((p) => (
            <button
              key={p.id}
              type="button"
              className={`map-list-item${p.id === selectedId ? " selected" : ""}`}
              onClick={() => setSelectedId(p.id)}
            >
              <span className={`map-marker ${p.entity === "site" ? "site" : (p.health || "unknown")}`} style={{ animation: "none", opacity: 1, transform: "scale(0.85)", flexShrink: 0 }} />
              <span className="meta">
                <strong>{p.name}</strong>
                <span>{p.entity} · {p.kind}{p.health ? ` · ${p.health}` : ""}</span>
              </span>
            </button>
          ))}
          {!pins.length && <p className="empty" style={{ padding: 16 }}>No matches on the map.</p>}
        </div>
      </aside>

      {selected && (
        <div className="map-detail map-glass">
          <p className="kicker">{selected.entity}</p>
          <h2>{selected.name}</h2>
          <p className="lede" style={{ marginTop: 4 }}>{selected.kind}</p>
          {selected.health && (
            <p style={{ marginTop: 10 }}><Health value={selected.health} /></p>
          )}
          <p className="coords">
            {selected.latitude.toFixed(5)}, {selected.longitude.toFixed(5)}
          </p>
          {selected.entity === "asset" && (
            <>
              {selected.external_ref && <p className="lede">Ref {selected.external_ref}</p>}
              <p className="lede">Last seen {fmt(selected.last_seen_at)}</p>
              <div className="row-actions" style={{ marginTop: 14 }}>
                <Link className="btn accent small" to={`/assets?focus=${encodeURIComponent(selected.id)}`}>
                  Open asset
                </Link>
                <button type="button" className="btn ghost small" onClick={() => setSelectedId(null)}>
                  Close
                </button>
              </div>
            </>
          )}
          {selected.entity === "site" && (
            <div className="row-actions" style={{ marginTop: 14 }}>
              <Link className="btn accent small" to="/sites">
                Sites
              </Link>
              <button type="button" className="btn ghost small" onClick={() => setSelectedId(null)}>
                Close
              </button>
            </div>
          )}
        </div>
      )}

      {!pins.length && !q && kind === "all" && health === "all" && (
        <div className="map-empty">
          <div className="map-glass">
            <h2 style={{ margin: "0 0 8px", fontSize: 20 }}>No coordinates yet</h2>
            <p className="lede" style={{ margin: 0 }}>
              Add latitude and longitude on a site or asset to plot it here.
            </p>
          </div>
        </div>
      )}
    </div>
  );
}
