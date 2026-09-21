import { FormEvent, useEffect, useState } from "react";
import { api, getToken, WorkOrder } from "../lib/api";
import { flushQueue, queueChange, readOrders, readQueue, writeOrders, type FieldOrder } from "../lib/fieldQueue";

function toField(row: WorkOrder): FieldOrder {
  return { id: row.id, title: row.title, status: row.status, notes: row.notes || "", kind: row.kind, priority: row.priority, checklist: row.checklist, asset_id: row.asset_id };
}

function readManuals(id: string): { name: string; data: string }[] {
  try {
    const raw = localStorage.getItem("yard.field.manuals." + id);
    const parsed = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function writeManuals(id: string, files: { name: string; data: string }[]) {
  localStorage.setItem("yard.field.manuals." + id, JSON.stringify(files));
}

function openManual(file: { name: string; data: string }) {
  const bin = atob(file.data);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  const url = URL.createObjectURL(new Blob([bytes]));
  window.open(url, "_blank", "noopener");
}

function checklistItems(raw?: string): { label?: string; done?: boolean }[] {
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

export default function Field() {
  const [rows, setRows] = useState<FieldOrder[]>([]);
  const [sel, setSel] = useState<FieldOrder | null>(null);
  const [notes, setNotes] = useState("");
  const [pending, setPending] = useState(0);
  const [online, setOnline] = useState(navigator.onLine);
  const [msg, setMsg] = useState("");
  const [permit, setPermit] = useState("");
  const [manuals, setManuals] = useState<{ name: string; data: string }[]>([]);

  function refreshLocal() {
    setRows(readOrders(localStorage));
    setPending(readQueue(localStorage).length);
  }

  async function pull() {
    const list = await api<WorkOrder[]>("/api/v1/work-orders");
    const open = list.filter((row) => row.status !== "done" && row.status !== "cancelled").map(toField);
    const queued = new Map(readQueue(localStorage).map((item) => [item.id, item]));
    const merged = open.map((row) => {
      const change = queued.get(row.id);
      return change ? { ...row, status: change.status, notes: change.notes } : row;
    });
    writeOrders(localStorage, merged);
    refreshLocal();
  }

  async function sync() {
    if (!navigator.onLine) return;
    const sent = await flushQueue(localStorage, async (change) => {
      await api(`/api/v1/work-orders/${change.id}`, {
        method: "PATCH",
        body: JSON.stringify({ status: change.status, notes: change.notes }),
      });
    });
    if (sent) setMsg(`Synced ${sent} update${sent === 1 ? "" : "s"}`);
    try {
      await pull();
    } catch {
      refreshLocal();
    }
  }

  useEffect(() => {
    refreshLocal();
    if (navigator.onLine) void sync();
    function on() { setOnline(true); void sync(); }
    function off() { setOnline(false); }
    window.addEventListener("online", on);
    window.addEventListener("offline", off);
    return () => {
      window.removeEventListener("online", on);
      window.removeEventListener("offline", off);
    };
  }, []);

  async function open(row: FieldOrder) {
    setSel(row);
    setNotes(row.notes || "");
    setMsg("");
    setPermit("");
    setManuals(readManuals(row.id));
    if (!navigator.onLine) return;
    try {
      const list = await api<{ status: string }[]>(`/api/v1/work-orders/${row.id}/permits`);
      if (list.some((item) => item.status === "approved")) setPermit("approved");
      else if (list.length) setPermit("pending");
      else setPermit("none");
    } catch {
      setPermit("");
    }
    if (!row.asset_id) return;
    try {
      const files = await api<{ id: string; name: string }[]>(`/api/v1/assets/${row.asset_id}/attachments`);
      const cached: { name: string; data: string }[] = [];
      for (const file of files) {
        const res = await fetch(`/api/v1/assets/${row.asset_id}/attachments/${file.id}`, { headers: { Authorization: `Bearer ${getToken()}` } });
        if (!res.ok) continue;
        const buf = await res.arrayBuffer();
        const bytes = new Uint8Array(buf);
        let binary = "";
        bytes.forEach((b) => { binary += String.fromCharCode(b); });
        cached.push({ name: file.name, data: btoa(binary) });
      }
      writeManuals(row.id, cached);
      setManuals(cached);
    } catch {
      setManuals(readManuals(row.id));
    }
  }

  function complete(e: FormEvent) {
    e.preventDefault();
    if (!sel) return;
    const change = { id: sel.id, status: "done", notes: notes.trim() };
    queueChange(localStorage, change);
    refreshLocal();
    setSel({ ...sel, ...change });
    setMsg(navigator.onLine ? "Saving…" : "Saved on this device. It will sync when you are back online.");
    if (navigator.onLine) void sync();
  }

  const visible = rows.filter((row) => row.status !== "done");

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Field</h1>
          <p className="lede">{online ? "Online" : "Offline"} · {pending ? `${pending} waiting to sync` : "Nothing waiting"}</p>
        </div>
      </div>
      {msg && <p className="lede" style={{ marginBottom: 12 }}>{msg}</p>}
      <div className="split">
        <div className="card">
          {visible.map((row) => (
            <button key={row.id} type="button" className="btn ghost" style={{ display: "block", width: "100%", textAlign: "left", marginBottom: 8 }} onClick={() => open(row)}>
              <strong>{row.title}</strong>
              <span className="lede"> {row.kind} · {row.priority} · {row.status}</span>
            </button>
          ))}
          {!visible.length && <p className="empty">No open work orders on this device. Open Field once while online to cache them.</p>}
        </div>
        <aside className="panel">
          {!sel && <p className="empty">Choose a work order.</p>}
          {sel && (
            <form onSubmit={complete}>
              <h2>{sel.title}</h2>
              <p className="lede">{sel.status}{permit ? ` · permit ${permit}` : ""}</p>
              {checklistItems(sel.checklist).length > 0 && (
                <ul>
                  {checklistItems(sel.checklist).map((item, i) => (
                    <li key={i}>{item.done ? "Done" : "Open"} · {item.label || "Step"}</li>
                  ))}
                </ul>
              )}
              {manuals.map((file) => (
                <button key={file.name} type="button" className="btn small ghost" style={{ marginRight: 8 }} onClick={() => openManual(file)}>
                  {file.name}
                </button>
              ))}
              <label>Notes
                <textarea value={notes} onChange={(e) => setNotes(e.target.value)} rows={4} style={{ width: "100%", marginTop: 8 }} />
              </label>
              <button className="btn accent" type="submit" style={{ marginTop: 12 }} disabled={sel.status === "done"}>Complete</button>
            </form>
          )}
        </aside>
      </div>
    </>
  );
}
