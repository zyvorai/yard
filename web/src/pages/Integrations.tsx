import { FormEvent, useEffect, useState } from "react";
import { api, Connector } from "../lib/api";
import { fmt } from "../components/Shell";

type ActionResult = { id: string; status: string; result: string; action: string };

function parseActions(raw: string): string[] {
  try {
    const v = JSON.parse(raw || "[]");
    return Array.isArray(v) ? v.map(String) : [];
  } catch {
    return [];
  }
}

export default function Integrations() {
  const [rows, setRows] = useState<Connector[]>([]);
  const [msg, setMsg] = useState("");
  const [busy, setBusy] = useState("");

  async function load() {
    setRows(await api<Connector[]>("/api/v1/connectors"));
  }
  useEffect(() => { load(); }, []);

  async function saveEndpoint(c: Connector, endpoint: string) {
    setBusy(c.id);
    setMsg("");
    try {
      await api(`/api/v1/connectors`, {
        method: "PATCH",
        body: JSON.stringify({ id: c.id, endpoint, status: endpoint ? "pending" : c.status }),
      });
      setMsg(`Updated ${c.name}`);
      await load();
    } catch (ex) {
      setMsg(ex instanceof Error ? ex.message : "update failed");
    } finally {
      setBusy("");
    }
  }

  async function runAction(c: Connector, action: string) {
    setBusy(c.id + action);
    setMsg("");
    try {
      const res = await api<ActionResult>("/api/v1/actions", {
        method: "POST",
        body: JSON.stringify({ connector_id: c.id, action, payload: "{}" }),
      });
      setMsg(`${action}: ${res.status} — ${res.result?.slice(0, 160) || ""}`);
      await load();
    } catch (ex) {
      setMsg(ex instanceof Error ? ex.message : "action failed");
    } finally {
      setBusy("");
    }
  }

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Integrations</h1>
          <p className="lede">Agents, HTTP ingestion, and optional Zyvor connectors. Device Agent can sync from here.</p>
        </div>
      </div>
      {msg && <p className="lede" style={{ marginBottom: 12 }}>{msg}</p>}
      <div className="grid" style={{ gridTemplateColumns: "repeat(auto-fill,minmax(300px,1fr))" }}>
        {rows.map((c) => {
          const actions = parseActions(c.actions);
          const executable = c.kind === "device-agent";
          return (
            <div className="card" key={c.id}>
              <h2>{c.kind}</h2>
              <div style={{ fontSize: 18, fontWeight: 650 }}>{c.name}</div>
              <p>
                <span className={`pill ${c.status === "connected" ? "ok" : c.status === "available" ? "info" : "stale"}`}>{c.status}</span>
              </p>
              <EndpointForm
                key={c.id + (c.endpoint || "")}
                initial={c.endpoint || ""}
                disabled={busy.startsWith(c.id)}
                onSave={(endpoint) => saveEndpoint(c, endpoint)}
              />
              <p className="lede">Actions: {actions.join(", ") || "none"}</p>
              {c.token_hint && <p className="lede">Token {c.token_hint}</p>}
              <p className="lede">Last sync {fmt(c.last_sync_at)}</p>
              {executable && (
                <div className="row-actions" style={{ marginTop: 10 }}>
                  {actions.map((a) => (
                    <button
                      key={a}
                      type="button"
                      className="btn small accent"
                      disabled={!!busy}
                      onClick={() => runAction(c, a)}
                    >
                      {a}
                    </button>
                  ))}
                </div>
              )}
              {!executable && c.status === "available" && (
                <p className="lede">Catalog only until the product API is wired.</p>
              )}
            </div>
          );
        })}
      </div>
    </>
  );
}

function EndpointForm({ initial, disabled, onSave }: { initial: string; disabled: boolean; onSave: (v: string) => void }) {
  const [value, setValue] = useState(initial);
  function submit(e: FormEvent) {
    e.preventDefault();
    onSave(value.trim());
  }
  return (
    <form onSubmit={submit} className="endpoint-form">
      <label className="lede" style={{ display: "block", marginTop: 8 }}>
        Endpoint
        <input value={value} onChange={(e) => setValue(e.target.value)} placeholder="http://127.0.0.1:9188" disabled={disabled} />
      </label>
      <button className="btn small ghost" type="submit" disabled={disabled} style={{ marginTop: 8 }}>Save</button>
    </form>
  );
}
