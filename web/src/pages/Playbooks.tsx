import { FormEvent, useEffect, useState } from "react";
import { api } from "../lib/api";

type Playbook = { id: string; name: string; body: string };
type Run = { id: string; status: string; step_index: number; error?: string };
type Asset = { id: string; name: string };
type Conn = { id: string; name: string };

type Step = { action: string; payload: string };

function yamlFor(name: string, steps: Step[]) {
  const lines = [`name: ${name || "Playbook"}`, "steps:"];
  for (const step of steps) {
    lines.push(`  - action: ${step.action || "diagnostics.read"}`);
    if (step.payload && step.payload !== "{}") lines.push(`    payload: '${step.payload.split("'").join("''")}'`);
  }
  return lines.join("\n") + "\n";
}

export default function Playbooks() {
  const [rows, setRows] = useState<Playbook[]>([]);
  const [assets, setAssets] = useState<Asset[]>([]);
  const [conns, setConns] = useState<Conn[]>([]);
  const [name, setName] = useState("Restart pump");
  const [steps, setSteps] = useState<Step[]>([
    { action: "diagnostics.read", payload: "{}" },
    { action: "reboot", payload: "{}" },
  ]);
  const body = yamlFor(name, steps);
  const [assetId, setAssetId] = useState("");
  const [connId, setConnId] = useState("");
  const [msg, setMsg] = useState("");
  const [lastRun, setLastRun] = useState("");

  async function load() {
    const [list, found, cs] = await Promise.all([
      api<Playbook[]>("/api/v1/playbooks"),
      api<Asset[]>("/api/v1/assets"),
      api<Conn[]>("/api/v1/connectors"),
    ]);
    setRows(list);
    setAssets(found);
    setConns(cs);
    setAssetId((id) => id || found[0]?.id || "");
    setConnId((id) => id || cs[0]?.id || "");
  }
  useEffect(() => { load(); }, []);

  async function create(e: FormEvent) {
    e.preventDefault();
    setMsg("");
    try {
      await api("/api/v1/playbooks", { method: "POST", body: JSON.stringify({ body }) });
      await load();
    } catch (err) {
      setMsg(err instanceof Error ? err.message : "Could not save");
    }
  }

  async function run(id: string, dry: boolean) {
    setMsg("");
    try {
      const out = await api<{ run: Run }>(`/api/v1/playbooks/${id}/run`, {
        method: "POST",
        body: JSON.stringify({ asset_id: assetId, connector_id: connId, dry_run: dry }),
      });
      setLastRun(out.run.id);
      setMsg(`${dry ? "Dry run" : "Run"} ${out.run.status}`);
    } catch (err) {
      setMsg(err instanceof Error ? err.message : "Run failed");
    }
  }

  async function approve() {
    setMsg("");
    if (!lastRun) {
      setMsg("Run a playbook first");
      return;
    }
    try {
      await api(`/api/v1/playbook-runs/${lastRun}/approve`, { method: "POST" });
      setMsg("Approved");
    } catch (err) {
      setMsg(err instanceof Error ? err.message : "Approve failed");
    }
  }

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Playbooks</h1>
          <p className="lede">Ordered steps. A dry run does not enqueue work. Reboot and similar steps wait for a second operator.</p>
        </div>
      </div>
      <form className="card" onSubmit={create} style={{ marginBottom: 12 }}>
        <label>Name
          <input value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        {steps.map((step, i) => (
          <div key={i} className="form-grid" style={{ marginTop: 8 }}>
            <input aria-label={`Action ${i + 1}`} value={step.action} onChange={(e) => {
              const next = [...steps];
              next[i] = { ...step, action: e.target.value };
              setSteps(next);
            }} />
            <input aria-label={`Payload ${i + 1}`} value={step.payload} onChange={(e) => {
              const next = [...steps];
              next[i] = { ...step, payload: e.target.value };
              setSteps(next);
            }} />
            <div className="row-actions">
              <button className="btn small ghost" type="button" onClick={() => setSteps(steps.filter((_, j) => j !== i))} disabled={steps.length === 1}>Remove</button>
              <button className="btn small ghost" type="button" disabled={i === 0} onClick={() => {
                const next = [...steps];
                [next[i - 1], next[i]] = [next[i], next[i - 1]];
                setSteps(next);
              }}>Up</button>
            </div>
          </div>
        ))}
        <button className="btn small ghost" type="button" style={{ marginTop: 8 }} onClick={() => setSteps([...steps, { action: "", payload: "{}" }])}>Add step</button>
        <pre className="lede" style={{ whiteSpace: "pre-wrap" }}>{body}</pre>
        <div className="row-actions" style={{ marginTop: 8 }}>
          <select value={assetId} onChange={(e) => setAssetId(e.target.value)}>
            {assets.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
          </select>
          <select value={connId} onChange={(e) => setConnId(e.target.value)}>
            {conns.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
          </select>
          <button className="btn small accent" type="submit">Save</button>
        </div>
        {msg && <p className="lede">{msg}</p>}
      </form>
      {rows.map((p) => (
        <div className="card" key={p.id} style={{ marginBottom: 12 }}>
          <div className="topbar">
            <h2>{p.name}</h2>
            <div className="row-actions">
              <button className="btn small ghost" type="button" onClick={() => run(p.id, true)}>Dry run</button>
              <button className="btn small accent" type="button" onClick={() => run(p.id, false)}>Run</button>
              <button className="btn small ghost" type="button" onClick={approve}>Approve</button>
            </div>
          </div>
        </div>
      ))}
    </>
  );
}
