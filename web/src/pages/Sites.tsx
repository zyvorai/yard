import { useEffect, useState } from "react";
import { api, Site } from "../lib/api";

export default function Sites() {
  const [rows, setRows] = useState<Site[]>([]);
  useEffect(() => { api<Site[]>("/api/v1/sites").then(setRows); }, []);
  return (
    <>
      <div className="topbar"><div><h1>Sites</h1><p className="lede">Factories, warehouses, offices, and customer locations.</p></div></div>
      <div className="grid" style={{ gridTemplateColumns: "repeat(auto-fill,minmax(260px,1fr))" }}>
        {rows.map((s) => (
          <div className="card" key={s.id}>
            <h2>{s.kind}</h2>
            <div style={{ fontSize: 20, fontWeight: 650 }}>{s.name}</div>
            <p className="lede">{s.address}</p>
          </div>
        ))}
      </div>
    </>
  );
}
