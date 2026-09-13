import { useEffect, useState } from "react";
import { api, Asset } from "../lib/api";

export default function MapPage() {
  const [rows, setRows] = useState<Asset[]>([]);
  useEffect(() => { api<Asset[]>("/api/v1/assets").then(setRows); }, []);
  const located = rows.filter((a) => a.latitude && a.longitude);
  const minLat = Math.min(...located.map((a) => a.latitude!));
  const maxLat = Math.max(...located.map((a) => a.latitude!));
  const minLng = Math.min(...located.map((a) => a.longitude!));
  const maxLng = Math.max(...located.map((a) => a.longitude!));
  function pos(a: Asset) {
    const x = ((a.longitude! - minLng) / (maxLng - minLng || 1)) * 80 + 10;
    const y = (1 - (a.latitude! - minLat) / (maxLat - minLat || 1)) * 70 + 12;
    return { left: `${x}%`, top: `${y}%` };
  }
  return (
    <>
      <div className="topbar"><div><h1>Map</h1><p className="lede">Assets with known locations. Offline assets stay visibly stale.</p></div></div>
      <div className="map">
        {located.map((a) => (
          <div className="pin" key={a.id} style={pos(a)} title={a.name}>
            <span className="lbl">{a.name}</span>
            <span className={`dot ${a.health === "critical" ? "critical" : a.health === "stale" ? "stale" : ""}`} />
          </div>
        ))}
      </div>
    </>
  );
}
