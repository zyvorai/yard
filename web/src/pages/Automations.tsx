import { FormEvent, useEffect, useState } from "react";
import { api, Automation } from "../lib/api";

type ActionField = { key: string; label: string; placeholder?: string };
type ActionSpec = { fields: ActionField[]; buildConfig: (v: Record<string, string>) => string; describe: (cfg: Record<string, unknown>) => string };

function simpleAction(field?: ActionField): ActionSpec {
  if (!field) return { fields: [], buildConfig: () => "{}", describe: () => "" };
  return {
    fields: [field],
    buildConfig: (v) => JSON.stringify({ [field.key]: (v[field.key] || "").trim() }),
    describe: (cfg) => (typeof cfg[field.key] === "string" ? (cfg[field.key] as string) : ""),
  };
}

// Every action type declares the config fields its rule form needs and how
// to build/describe the JSON blob stored in Automation.config — adding a new
// action type (a new integration) means one entry here, not a new
// hand-rolled conditional block.
const ACTION_SPECS: Record<string, ActionSpec> = {
  open_incident: simpleAction(),
  notify: simpleAction(),
  webhook: simpleAction({ key: "url", label: "Webhook URL", placeholder: "https://example.com/hook" }),
  slack: simpleAction({ key: "slack_url", label: "Slack webhook URL", placeholder: "https://hooks.slack.com/services/..." }),
  pagerduty: simpleAction({ key: "routing_key", label: "PagerDuty routing key" }),
  email: simpleAction({ key: "to", label: "Recipient email", placeholder: "ops@example.com" }),
};

function specFor(action: string): ActionSpec {
  return ACTION_SPECS[action] || simpleAction();
}

function describeAction(a: Automation): string {
  let cfg: Record<string, unknown> = {};
  try { cfg = JSON.parse(a.config || "{}"); } catch { /* malformed config, show action name only */ }
  const detail = specFor(a.action).describe(cfg);
  return detail ? `${a.action} · ${detail}` : a.action;
}

const emptyForm = {
  name: "", trigger_kind: "threshold", capability: "temperature", operator: "gt",
  threshold: "75", action: "open_incident",
};

export default function Automations() {
  const [rows, setRows] = useState<Automation[]>([]);
  const [open, setOpen] = useState(false);
  const [err, setErr] = useState("");
  const [form, setForm] = useState(emptyForm);
  const [actionValues, setActionValues] = useState<Record<string, string>>({});

  async function load() {
    setRows(await api<Automation[]>("/api/v1/automations"));
  }
  useEffect(() => { load(); }, []);

  async function toggle(a: Automation) {
    await api("/api/v1/automations", { method: "PATCH", body: JSON.stringify({ id: a.id, enabled: !a.enabled }) });
    await load();
  }

  async function remove(a: Automation) {
    if (!confirm(`Delete automation ${a.name}?`)) return;
    await api("/api/v1/automations", { method: "PATCH", body: JSON.stringify({ id: a.id, delete: true }) });
    await load();
  }

  async function create(e: FormEvent) {
    e.preventDefault();
    setErr("");
    const body: Record<string, unknown> = {
      name: form.name.trim(),
      trigger_kind: form.trigger_kind,
      capability: form.capability.trim(),
      operator: form.operator,
      threshold: Number(form.threshold) || 0,
      action: form.action,
      config: specFor(form.action).buildConfig(actionValues),
    };
    try {
      await api("/api/v1/automations", { method: "POST", body: JSON.stringify(body) });
      setOpen(false);
      setForm(emptyForm);
      setActionValues({});
      await load();
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : "create failed");
    }
  }

  const needsCapability = form.trigger_kind === "threshold" || form.trigger_kind === "capability_min" || form.trigger_kind === "capability_max";
  const needsOperatorThreshold = form.trigger_kind === "threshold";

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Automations</h1>
          <p className="lede">Thresholds, capability ranges, stale checks, and notification actions.</p>
        </div>
        <button type="button" className="btn accent" onClick={() => setOpen((v) => !v)}>{open ? "Cancel" : "New rule"}</button>
      </div>
      {open && (
        <form className="card form-card" onSubmit={create} style={{ marginBottom: 16 }}>
          <h2>New automation</h2>
          <div className="form-grid">
            <label className="span-2">Name<input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></label>
            <label>Trigger
              <select value={form.trigger_kind} onChange={(e) => setForm({ ...form, trigger_kind: e.target.value })}>
                <option value="threshold">threshold</option>
                <option value="capability_min">capability_min</option>
                <option value="capability_max">capability_max</option>
                <option value="stale">stale</option>
              </select>
            </label>
            {needsCapability && (
              <label>Capability<input value={form.capability} onChange={(e) => setForm({ ...form, capability: e.target.value })} /></label>
            )}
            {needsOperatorThreshold && (
              <>
                <label>Operator
                  <select value={form.operator} onChange={(e) => setForm({ ...form, operator: e.target.value })}>
                    {["gt", "gte", "lt", "lte"].map((o) => <option key={o}>{o}</option>)}
                  </select>
                </label>
                <label>Threshold<input type="number" step="any" value={form.threshold} onChange={(e) => setForm({ ...form, threshold: e.target.value })} /></label>
              </>
            )}
            {(form.trigger_kind === "capability_min" || form.trigger_kind === "capability_max") && (
              <p className="lede span-2" style={{ margin: 0 }}>
                Fires off the capability's own {form.trigger_kind === "capability_min" ? "Min" : "Max"} value — no threshold needed.
              </p>
            )}
            <label>Action
              <select value={form.action} onChange={(e) => { setForm({ ...form, action: e.target.value }); setActionValues({}); }}>
                {Object.keys(ACTION_SPECS).map((a) => <option key={a} value={a}>{a}</option>)}
              </select>
            </label>
            {specFor(form.action).fields.map((f) => (
              <label className="span-2" key={f.key}>
                {f.label}
                <input
                  value={actionValues[f.key] || ""}
                  onChange={(e) => setActionValues({ ...actionValues, [f.key]: e.target.value })}
                  placeholder={f.placeholder}
                  required
                />
              </label>
            ))}
          </div>
          {err && <p className="form-error">{err}</p>}
          <div className="row-actions" style={{ marginTop: 12 }}>
            <button className="btn accent" type="submit">Create</button>
          </div>
        </form>
      )}
      <div className="card table-wrap">
        <table>
          <thead><tr><th>Name</th><th>Trigger</th><th>Rule</th><th>Action</th><th></th></tr></thead>
          <tbody>
            {rows.map((a) => (
              <tr key={a.id}>
                <td>{a.name}</td>
                <td>{a.trigger_kind}</td>
                <td>{a.trigger_kind === "threshold" ? `${a.capability} ${a.operator} ${a.threshold}` : a.capability}</td>
                <td>{describeAction(a)}</td>
                <td className="row-actions">
                  <button className="btn small ghost" onClick={() => toggle(a)}>{a.enabled ? "Enabled" : "Paused"}</button>
                  <button className="btn small ghost" onClick={() => remove(a)}>Delete</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
