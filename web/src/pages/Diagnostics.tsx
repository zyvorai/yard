import { useEffect, useMemo, useState } from "react";
import { api, EventItem } from "../lib/api";
import { fmt } from "../components/Shell";

export default function Diagnostics() {
  const [rows, setRows] = useState<EventItem[]>([]);
  const [q, setQ] = useState("");
  useEffect(() => { api<EventItem[]>("/api/v1/events").then(setRows); }, []);
  const filtered = useMemo(() => rows.filter((r) => `${r.title} ${r.body} ${r.kind}`.toLowerCase().includes(q.toLowerCase())), [rows, q]);
  return (
    <>
      <div className="topbar"><div><h1>Diagnostics</h1><p className="lede">Searchable operational log. Dark surface, readable type.</p></div></div>
      <div className="diag">
        <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Filter logs" />
        <pre>
          {filtered.map((e) => `${fmt(e.created_at)}  ${e.severity.padEnd(8)}  ${e.kind.padEnd(20)}  ${e.title}  ${e.body}`).join("\n") || "No matching events."}
        </pre>
      </div>
    </>
  );
}
