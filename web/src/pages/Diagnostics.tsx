import { useEffect, useMemo, useState } from "react";
import { api, EventItem } from "../lib/api";
import { fmt } from "../components/Shell";

function sevClass(sev: string) {
  return sev === "critical" ? "diag-sev--critical" : sev === "warning" ? "diag-sev--warning" : "diag-sev--info";
}

function DiagLine({ e }: { e: EventItem }) {
  return (
    <div className="diag-line">
      <span className="diag-ts">{fmt(e.created_at)}</span>
      <span className={`diag-sev ${sevClass(e.severity)}`}>{e.severity}</span>
      <span className="diag-kind">{e.kind}</span>
      <span className="diag-title">{e.title}</span>
      {e.body && <span className="diag-body">{e.body}</span>}
    </div>
  );
}

export default function Diagnostics() {
  const [rows, setRows] = useState<EventItem[]>([]);
  const [q, setQ] = useState("");
  useEffect(() => { api<EventItem[]>("/api/v1/events").then(setRows); }, []);
  const filtered = useMemo(() => rows.filter((r) => `${r.title} ${r.body} ${r.kind}`.toLowerCase().includes(q.toLowerCase())), [rows, q]);
  return (
    <>
      <div className="topbar"><div><h1>Diagnostics</h1><p className="lede">Searchable operational log. Dark surface, readable type.</p></div></div>
      <div className="diag">
        <div className="diag-bar">
          <span className="diag-dot diag-dot--red" />
          <span className="diag-dot diag-dot--yellow" />
          <span className="diag-dot diag-dot--green" />
          <span className="diag-bar-title">events — yard</span>
        </div>
        <div className="diag-body-wrap">
          <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Filter logs" />
          <div className="diag-log">
            {filtered.length ? filtered.map((e) => <DiagLine key={e.id} e={e} />) : <p className="diag-empty">No matching events.</p>}
          </div>
        </div>
      </div>
    </>
  );
}
