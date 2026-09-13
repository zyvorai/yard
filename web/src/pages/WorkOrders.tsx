import { useEffect, useState } from "react";
import { api, WorkOrder } from "../lib/api";
import { fmt } from "../components/Shell";

export default function WorkOrders() {
  const [rows, setRows] = useState<WorkOrder[]>([]);
  useEffect(() => { api<WorkOrder[]>("/api/v1/work-orders").then(setRows); }, []);
  async function done(id: string) {
    await api(`/api/v1/work-orders/${id}`, { method: "PATCH", body: JSON.stringify({ status: "done", notes: "Completed in the field." }) });
    setRows(await api<WorkOrder[]>("/api/v1/work-orders"));
  }
  return (
    <>
      <div className="topbar"><div><h1>Work orders</h1><p className="lede">Inspections, repairs, installations, and maintenance.</p></div></div>
      <div className="card table-wrap">
        <table>
          <thead><tr><th>Title</th><th>Kind</th><th>Priority</th><th>Status</th><th>Assignee</th><th>Opened</th><th></th></tr></thead>
          <tbody>
            {rows.map((w) => (
              <tr key={w.id}>
                <td>{w.title}</td><td>{w.kind}</td><td>{w.priority}</td><td>{w.status}</td><td>{w.assignee}</td><td>{fmt(w.created_at)}</td>
                <td>{w.status !== "done" && <button className="btn small" onClick={() => done(w.id)}>Complete</button>}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {!rows.length && <p className="empty">No work orders yet.</p>}
      </div>
    </>
  );
}
