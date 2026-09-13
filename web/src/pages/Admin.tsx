import { useEffect, useState } from "react";
import { api } from "../lib/api";
import { fmt } from "../components/Shell";

type Audit = { id: string; actor: string; action: string; object: string; detail: string; created_at: string };

export default function Admin() {
  const [rows, setRows] = useState<Audit[]>([]);
  useEffect(() => { api<Audit[]>("/api/v1/audit").then(setRows); }, []);
  return (
    <>
      <div className="topbar"><div><h1>Administration</h1><p className="lede">Workspace, permissions, connector credentials, and audit history.</p></div></div>
      <div className="card table-wrap">
        <table>
          <thead><tr><th>When</th><th>Actor</th><th>Action</th><th>Object</th><th>Detail</th></tr></thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.id}><td>{fmt(r.created_at)}</td><td>{r.actor}</td><td>{r.action}</td><td>{r.object}</td><td>{r.detail}</td></tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
