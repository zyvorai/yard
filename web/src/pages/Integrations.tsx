import { useEffect, useState } from "react";
import { api, Connector } from "../lib/api";
import { fmt } from "../components/Shell";

export default function Integrations() {
  const [rows, setRows] = useState<Connector[]>([]);
  useEffect(() => { api<Connector[]>("/api/v1/connectors").then(setRows); }, []);
  return (
    <>
      <div className="topbar"><div><h1>Integrations</h1><p className="lede">Agents, HTTP ingestion, MQTT-ready contracts, and optional Zyvor connectors.</p></div></div>
      <div className="grid" style={{ gridTemplateColumns: "repeat(auto-fill,minmax(280px,1fr))" }}>
        {rows.map((c) => (
          <div className="card" key={c.id}>
            <h2>{c.kind}</h2>
            <div style={{ fontSize: 18, fontWeight: 650 }}>{c.name}</div>
            <p className="lede">{c.endpoint || "Not configured"}</p>
            <p><span className={`pill ${c.status === "connected" ? "ok" : c.status === "available" ? "info" : "stale"}`}>{c.status}</span></p>
            <p className="lede">Actions: {c.actions}</p>
            {c.token_hint && <p className="lede">Token {c.token_hint}</p>}
            <p className="lede">Last sync {fmt(c.last_sync_at)}</p>
          </div>
        ))}
      </div>
    </>
  );
}
