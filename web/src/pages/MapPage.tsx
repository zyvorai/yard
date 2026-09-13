import { useEffect, useMemo, useState } from "react";
import { api, Asset, Site } from "../lib/api";

type Pin = { id: string; name: string; kind: string; health?: string; latitude: number; longitude: number };

export default function MapPage() {
  const [assets, setAssets] = useState<Asset[]>([]);
  const [sites, setSites] = useState<Site[]>([]);
  const [filter, setFilter] = useState<"all" | "assets" | "sites">("all");

  useEffect(() => {
    Promise.all([
      api<Asset[]>("/api/v1/assets"),
      api<Site[]>("/api/v1/sites"),
    ]).then(([a, s]) => {
      setAssets(a);
      setSites(s);
    });
  }, []);

  const pins: Pin[] = useMemo(() => {
    const out: Pin[] = [];
    if (filter !== "sites") {
      for (const a of assets) {
        if (a.latitude != null && a.longitude != null) {
          out.push({ id: a.id, name: a.name, kind: a.kind, health: a.health, latitude: a.latitude, longitude: a.longitude });
        }
      }
    }
    if (filter !== "assets") {
      for (const s of sites) {
        if (s.latitude != null && s.longitude != null) {
          out.push({ id: s.id, name: s.name, kind: s.kind, latitude: s.latitude, longitude: s.longitude });
        }
      }
    }
    return out;
  }, [assets, sites, filter]);

  const bounds = useMemo(() => {
    if (!pins.length) return null;
    return {
      minLat: Math.min(...pins.map((p) => p.latitude)),
      maxLat: Math.max(...pins.map((p) => p.latitude)),
      minLng: Math.min(...pins.map((p) => p.longitude)),
      maxLng: Math.max(...pins.map((p) => p.longitude)),
    };
  }, [pins]);

  function pos(p: Pin) {
    if (!bounds) return { left: "50%", top: "50%" };
    const x = ((p.longitude - bounds.minLng) / (bounds.maxLng - bounds.minLng || 1)) * 80 + 10;
    const y = (1 - (p.latitude - bounds.minLat) / (bounds.maxLat - bounds.minLat || 1)) * 70 + 12;
    return { left: `${x}%`, top: `${y}%` };
  }

  const stale = assets.filter((a) => a.health === "stale" || a.health === "critical").length;

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Map</h1>
          <p className="lede">Assets and sites with coordinates. Offline assets stay visibly stale.</p>
        </div>
        <div className="row-actions">
          <select className="search" value={filter} onChange={(e) => setFilter(e.target.value as typeof filter)}>
            <option value="all">Assets + sites</option>
            <option value="assets">Assets only</option>
            <option value="sites">Sites only</option>
          </select>
        </div>
      </div>
      <div className="map-legend">
        <span className="pill ok">{pins.length} plotted</span>
        <span className="pill stale">{assets.length - assets.filter((a) => a.latitude && a.longitude).length} assets without coords</span>
        {stale > 0 && <span className="pill bad">{stale} attention</span>}
      </div>
      {!pins.length ? (
        <div className="card"><p className="empty">No coordinates yet. Add latitude/longitude on a site or asset.</p></div>
      ) : (
        <div className="map">
          {pins.map((p) => (
            <div className="pin" key={p.id} style={pos(p)} title={`${p.name} (${p.kind})`}>
              <span className="lbl">{p.name}</span>
              <span className={`dot ${p.health === "critical" ? "critical" : p.health === "stale" ? "stale" : p.health === "degraded" ? "warn" : ""}`} />
            </div>
          ))}
        </div>
      )}
      {pins.length > 0 && (
        <div className="card table-wrap" style={{ marginTop: 16 }}>
          <table>
            <thead><tr><th>Name</th><th>Kind</th><th>Health</th><th>Lat</th><th>Lng</th></tr></thead>
            <tbody>
              {pins.map((p) => (
                <tr key={p.id}>
                  <td>{p.name}</td>
                  <td>{p.kind}</td>
                  <td>{p.health || "—"}</td>
                  <td>{p.latitude.toFixed(4)}</td>
                  <td>{p.longitude.toFixed(4)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
