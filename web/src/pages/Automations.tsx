import { useEffect, useState } from "react";
import { api, Automation } from "../lib/api";

export default function Automations() {
  const [rows, setRows] = useState<Automation[]>([]);
  useEffect(() => { api<Automation[]>("/api/v1/automations").then(setRows); }, []);
  async function toggle(a: Automation) {
    await api("/api/v1/automations", { method: "PATCH", body: JSON.stringify({ id: a.id, enabled: !a.enabled }) });
    setRows(await api<Automation[]>("/api/v1/automations"));
  }
  return (
    <>
      <div className="topbar"><div><h1>Automations</h1><p className="lede">Notifications and approved actions from events.</p></div></div>
      <div className="card table-wrap">
        <table>
          <thead><tr><th>Name</th><th>Trigger</th><th>Rule</th><th>Action</th><th></th></tr></thead>
          <tbody>
            {rows.map((a) => (
              <tr key={a.id}>
                <td>{a.name}</td>
                <td>{a.trigger_kind}</td>
                <td>{a.capability} {a.operator} {a.threshold}</td>
                <td>{a.action}</td>
                <td><button className="btn small ghost" onClick={() => toggle(a)}>{a.enabled ? "Enabled" : "Paused"}</button></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
