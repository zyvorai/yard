const TOKEN = "estate.token";
const root = document.getElementById("root");

const routes = [
  ["/", "Overview"],
  ["/assets", "Assets"],
  ["/sites", "Sites"],
  ["/map", "Map"],
  ["/telemetry", "Telemetry"],
  ["/work", "Work orders"],
  ["/incidents", "Incidents"],
  ["/automations", "Automations"],
  ["/integrations", "Integrations"],
  ["/admin", "Administration"],
  ["/diagnostics", "Diagnostics"],
];

function token() { return localStorage.getItem(TOKEN) || ""; }
function setTok(t) { localStorage.setItem(TOKEN, t); }
function clearTok() { localStorage.removeItem(TOKEN); }

async function api(path, init = {}) {
  const headers = Object.assign({ Accept: "application/json" }, init.headers || {});
  if (init.body && !headers["Content-Type"]) headers["Content-Type"] = "application/json";
  if (token()) headers.Authorization = "Bearer " + token();
  const res = await fetch(path, Object.assign({}, init, { headers }));
  if (res.status === 401) {
    clearTok();
    location.hash = "#/login";
    throw new Error("unauthorized");
  }
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

function fmt(ts) {
  if (!ts) return "—";
  const d = new Date(ts);
  return Number.isNaN(d.getTime()) ? ts : d.toLocaleString();
}

function pill(value) {
  const v = (value || "").toLowerCase();
  const cls = v === "healthy" || v === "connected" || v === "fresh" || v === "ok" ? "ok"
    : v === "degraded" || v === "warning" || v === "high" ? "warn"
    : v === "critical" || v === "bad" ? "bad"
    : v === "info" || v === "available" ? "info" : "stale";
  return `<span class="pill ${cls}">${esc(value || "")}</span>`;
}

function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

function path() {
  const h = location.hash.replace(/^#/, "") || "/";
  return h.startsWith("/") ? h : "/" + h;
}

function nav(to) {
  location.hash = "#" + to;
}

async function render() {
  const p = path();
  if (!token() && p !== "/login") return nav("/login");
  if (p === "/login") return renderLogin();
  const page = await pageFor(p);
  root.innerHTML = `
    <div class="app">
      <aside class="sidebar">
        <div class="brand">
          <img src="/logo.svg" alt="Zyvor" />
          <div><div class="name">Estate</div><div class="sub">Zyvor operations</div></div>
        </div>
        <nav class="nav">
          ${routes.slice(0, 7).map(([to, label]) => `<a href="#${to}" class="${p === to ? "active" : ""}">${label}</a>`).join("")}
          <div class="sec">Platform</div>
          ${routes.slice(7).map(([to, label]) => `<a href="#${to}" class="${p === to ? "active" : ""}">${label}</a>`).join("")}
        </nav>
        <div class="foot">
          <div>Northwind Operations</div>
          <button class="btn ghost small" id="out" style="margin-top:8px">Sign out</button>
        </div>
      </aside>
      <main class="main">${page.html}</main>
    </div>`;
  document.getElementById("out").onclick = () => { clearTok(); nav("/login"); };
  if (page.bind) page.bind();
}

function renderLogin() {
  root.innerHTML = `
    <div class="auth">
      <form class="auth-card" id="login">
        <img src="/logo.svg" width="40" height="40" alt="Zyvor" />
        <p class="kicker" style="margin-top:16px">Zyvor</p>
        <h1>Estate</h1>
        <p class="lede">Sign in to the asset and operations workspace.</p>
        <div class="field"><label for="email">Email</label><input id="email" value="admin@estate.local" autocomplete="username" /></div>
        <div class="field"><label for="password">Password</label><input id="password" type="password" value="estate-admin" autocomplete="current-password" /></div>
        <p id="err" style="color:var(--bad)"></p>
        <button class="btn accent" type="submit" style="width:100%">Continue</button>
        <p class="hint">Demo: admin@estate.local / estate-admin</p>
      </form>
    </div>`;
  document.getElementById("login").onsubmit = async (e) => {
    e.preventDefault();
    try {
      const res = await api("/api/v1/auth/login", { method: "POST", body: JSON.stringify({
        email: document.getElementById("email").value,
        password: document.getElementById("password").value,
      })});
      setTok(res.token);
      nav("/");
    } catch {
      document.getElementById("err").textContent = "Those credentials were not accepted.";
    }
  };
}

async function pageFor(p) {
  try {
    if (p === "/") return overview();
    if (p === "/assets") return assets();
    if (p === "/sites") return sites();
    if (p === "/map") return mapPage();
    if (p === "/telemetry") return telemetry();
    if (p === "/work") return work();
    if (p === "/incidents") return incidents();
    if (p === "/automations") return automations();
    if (p === "/integrations") return integrations();
    if (p === "/admin") return admin();
    if (p === "/diagnostics") return diagnostics();
    return { html: "<h1>Not found</h1>" };
  } catch (e) {
    return { html: `<p class="lede">${esc(e.message)}</p>` };
  }
}

async function overview() {
  const d = await api("/api/v1/overview");
  return { html: `
    <div class="topbar"><div><h1>Overview</h1><p class="lede">Asset health, active work, incidents, and recent activity.</p></div></div>
    <div class="grid stats">
      <div class="card"><h2>Assets</h2><div class="metric">${d.assets_total}</div></div>
      <div class="card"><h2>Healthy</h2><div class="metric">${d.assets_healthy}<small>${d.assets_stale} stale</small></div></div>
      <div class="card"><h2>Open incidents</h2><div class="metric">${d.open_incidents}<small>${d.assets_critical} critical</small></div></div>
      <div class="card"><h2>Work orders</h2><div class="metric">${d.open_work_orders}<small>${d.active_connectors} connectors</small></div></div>
    </div>
    <div class="grid split" style="margin-top:16px">
      <div class="card"><h2>Recent events</h2>${table(["Title","Severity","When"], (d.recent_events||[]).map(e => [e.title, e.severity, fmt(e.created_at)]))}</div>
      <div class="card"><h2>Activity</h2>${(d.recent_activity||[]).map(a => `<p style="margin:10px 0;font-size:13px"><strong>${esc(a.action)}</strong> · ${esc(a.actor)}<br><span class="lede">${esc(a.detail||a.object)}</span></p>`).join("") || '<p class="empty">No activity.</p>'}</div>
    </div>` };
}

function table(headers, rows) {
  if (!rows.length) return '<p class="empty">Nothing here yet.</p>';
  return `<div class="table-wrap"><table><thead><tr>${headers.map(h => `<th>${h}</th>`).join("")}</tr></thead><tbody>${rows.map(r => `<tr>${r.map((c,i) => `<td>${i===1 && typeof c==='string' && /healthy|critical|degraded|stale|warning|open|connected|fresh|info/.test(c) ? pill(c) : esc(c)}</td>`).join("")}</tr>`).join("")}</tbody></table></div>`;
}

let assetState = { rows: [], sel: null, tab: "Overview" };

async function assets() {
  const rows = await api("/api/v1/assets");
  assetState.rows = rows;
  return {
    html: `
      <div class="topbar"><div><h1>Assets</h1><p class="lede">Devices, vehicles, machines, sensors, and other equipment.</p></div>
        <input class="search" id="q" placeholder="Search name, ref, serial" />
      </div>
      <div class="split">
        <div class="card" id="alist">${assetTable(rows)}</div>
        <aside class="panel" id="adetail"><p class="empty">Select an asset. The list stays in place.</p></aside>
      </div>`,
    bind() {
      document.getElementById("q").onkeydown = (e) => {
        if (e.key !== "Enter") return;
        const q = e.target.value.toLowerCase();
        document.getElementById("alist").innerHTML = assetTable(assetState.rows.filter(a => (a.name+a.external_ref+a.serial).toLowerCase().includes(q)));
        bindAssetRows();
      };
      bindAssetRows();
    }
  };
}

function assetTable(rows) {
  return `<div class="table-wrap"><table><thead><tr><th>Name</th><th>Kind</th><th>Health</th><th>Ref</th><th>Last seen</th></tr></thead><tbody>
    ${rows.map(a => `<tr data-id="${a.id}" style="cursor:pointer"><td>${esc(a.name)}</td><td>${esc(a.kind)}</td><td>${pill(a.health)}</td><td>${esc(a.external_ref)}</td><td>${fmt(a.last_seen_at)}</td></tr>`).join("")}
  </tbody></table></div>`;
}

function bindAssetRows() {
  document.querySelectorAll("#alist tr[data-id]").forEach(tr => {
    tr.onclick = async () => {
      const d = await api("/api/v1/assets/" + tr.dataset.id);
      assetState.sel = d;
      document.getElementById("adetail").innerHTML = assetDetail(d, "Overview");
      bindTabs(d);
    };
  });
}

function assetDetail(d, tab) {
  const a = d.asset;
  const tabs = ["Overview","Telemetry","Activity","Work","Integrations"];
  let body = "";
  if (tab === "Overview") body = `<p>${pill(a.health)} · stale after ${a.stale_after_sec}s</p><p class="lede">Capabilities</p>${(d.capabilities||[]).map(c => `<span class="pill" style="margin:4px">${esc(c.name)} ${esc(c.unit||"")}</span>`).join("")}`;
  if (tab === "Telemetry") body = `<table><tbody>${(d.observations||[]).slice(0,12).map(o => `<tr><td>${esc(o.capability)}</td><td>${Number(o.value).toFixed(2)} ${esc(o.unit)}</td><td>${esc(o.quality)}</td></tr>`).join("")}</tbody></table>`;
  if (tab === "Activity") body = `<p class="lede">Incidents and audit entries stay attached to this asset id.</p>`;
  if (tab === "Work") body = `<p class="lede">Open a work order from Incidents when inspection or repair is required.</p>`;
  if (tab === "Integrations") body = `<p class="lede">Last publisher becomes the source of record for inventory.</p>`;
  return `<p class="kicker">${esc(a.kind)}</p><h1 style="font-size:22px">${esc(a.name)}</h1><p class="lede">${esc(a.manufacturer)} ${esc(a.model)} · ${esc(a.serial)}</p>
    <div class="tabs">${tabs.map(t => `<button data-tab="${t}" class="${t===tab?"active":""}">${t}</button>`).join("")}</div>${body}`;
}

function bindTabs(d) {
  document.querySelectorAll("#adetail [data-tab]").forEach(b => {
    b.onclick = () => {
      document.getElementById("adetail").innerHTML = assetDetail(d, b.dataset.tab);
      bindTabs(d);
    };
  });
}

async function sites() {
  const rows = await api("/api/v1/sites");
  return { html: `
    <div class="topbar"><div><h1>Sites</h1><p class="lede">Factories, warehouses, offices, and customer locations.</p></div></div>
    <div class="grid" style="grid-template-columns:repeat(auto-fill,minmax(260px,1fr))">
      ${rows.map(s => `<div class="card"><h2>${esc(s.kind)}</h2><div style="font-size:20px;font-weight:650">${esc(s.name)}</div><p class="lede">${esc(s.address)}</p></div>`).join("")}
    </div>` };
}

async function mapPage() {
  const rows = (await api("/api/v1/assets")).filter(a => a.latitude && a.longitude);
  const minLat = Math.min(...rows.map(a => a.latitude));
  const maxLat = Math.max(...rows.map(a => a.latitude));
  const minLng = Math.min(...rows.map(a => a.longitude));
  const maxLng = Math.max(...rows.map(a => a.longitude));
  const pins = rows.map(a => {
    const x = ((a.longitude - minLng) / (maxLng - minLng || 1)) * 80 + 10;
    const y = (1 - (a.latitude - minLat) / (maxLat - minLat || 1)) * 70 + 12;
    const cls = a.health === "critical" ? "critical" : a.health === "stale" ? "stale" : "";
    return `<div class="pin" style="left:${x}%;top:${y}%"><span class="lbl">${esc(a.name)}</span><span class="dot ${cls}"></span></div>`;
  }).join("");
  return { html: `<div class="topbar"><div><h1>Map</h1><p class="lede">Assets with known locations. Offline assets stay visibly stale.</p></div></div><div class="map">${pins}</div>` };
}

async function telemetry() {
  const rows = await api("/api/v1/telemetry");
  return { html: `
    <div class="topbar"><div><h1>Telemetry</h1><p class="lede">Measurements, freshness, and quality. Stale points never look healthy.</p></div></div>
    <div class="card">${table(["Asset","Signal","Value","Quality","Fresh","Observed","Source"], rows.map(r => [r.asset_name, r.capability, `${r.value.toFixed(2)} ${r.unit}`, r.quality, r.fresh ? "fresh" : "stale", fmt(r.observed_at), r.source]))}</div>` };
}

async function work() {
  const rows = await api("/api/v1/work-orders");
  return {
    html: `
      <div class="topbar"><div><h1>Work orders</h1><p class="lede">Inspections, repairs, installations, and maintenance.</p></div></div>
      <div class="card table-wrap"><table><thead><tr><th>Title</th><th>Kind</th><th>Priority</th><th>Status</th><th>Assignee</th><th></th></tr></thead>
      <tbody>${rows.map(w => `<tr><td>${esc(w.title)}</td><td>${esc(w.kind)}</td><td>${esc(w.priority)}</td><td>${esc(w.status)}</td><td>${esc(w.assignee)}</td><td>${w.status!=="done"?`<button class="btn small" data-done="${w.id}">Complete</button>`:""}</td></tr>`).join("") || '<tr><td colspan="6" class="empty">No work orders yet.</td></tr>'}</tbody></table></div>`,
    bind() {
      document.querySelectorAll("[data-done]").forEach(b => b.onclick = async () => {
        await api("/api/v1/work-orders/"+b.dataset.done, { method: "PATCH", body: JSON.stringify({ status: "done", notes: "Completed in the field." })});
        render();
      });
    }
  };
}

async function incidents() {
  const rows = await api("/api/v1/incidents");
  return {
    html: `
      <div class="topbar"><div><h1>Incidents</h1><p class="lede">Acknowledge problems, assign owners, record resolution.</p></div></div>
      <div class="split">
        <div class="card table-wrap"><table><thead><tr><th>Title</th><th>Severity</th><th>Status</th><th>Opened</th></tr></thead>
        <tbody>${rows.map(i => `<tr data-id="${i.id}" style="cursor:pointer"><td>${esc(i.title)}</td><td>${pill(i.severity)}</td><td>${esc(i.status)}</td><td>${fmt(i.opened_at)}</td></tr>`).join("") || '<tr><td colspan="4" class="empty">No incidents. A simulator temperature trip will open one.</td></tr>'}</tbody></table></div>
        <aside class="panel" id="idetail"><p class="empty">Select an incident.</p></aside>
      </div>`,
    bind() {
      document.querySelectorAll("tr[data-id]").forEach(tr => {
        tr.onclick = () => showIncident(rows.find(r => r.id === tr.dataset.id));
      });
    }
  };
}

function showIncident(i) {
  const el = document.getElementById("idetail");
  el.innerHTML = `<h1 style="font-size:22px">${esc(i.title)}</h1><p class="lede">${esc(i.summary)}</p><p>${pill(i.severity)} ${esc(i.status)} · ${esc(i.owner||"unassigned")}</p>
    ${i.resolution ? `<p class="lede">Resolution: ${esc(i.resolution)}</p>` : ""}
    <div class="row-actions" style="margin-top:16px">
      <button class="btn small" id="ack">Acknowledge</button>
      <button class="btn small accent" id="wo">Assign work order</button>
      <button class="btn small ghost" id="res">Resolve</button>
    </div>
    <p class="lede" id="imsg"></p>`;
  document.getElementById("ack").onclick = async () => {
    await api("/api/v1/incidents/"+i.id, { method: "PATCH", body: JSON.stringify({ status: "ack", owner: "Operations Admin" })});
    document.getElementById("imsg").textContent = "Acknowledged.";
  };
  document.getElementById("wo").onclick = async () => {
    await api("/api/v1/work-orders", { method: "POST", body: JSON.stringify({ title: "Maintenance for "+i.title, kind: "maintenance", priority: "high", incident_id: i.id, asset_id: i.asset_id, assignee: "Maya Chen" })});
    document.getElementById("imsg").textContent = "Work order created.";
  };
  document.getElementById("res").onclick = async () => {
    await api("/api/v1/incidents/"+i.id, { method: "PATCH", body: JSON.stringify({ status: "resolved", resolution: "Inspected on site; condition returned to band." })});
    document.getElementById("imsg").textContent = "Resolved and written to activity history.";
  };
}

async function automations() {
  const rows = await api("/api/v1/automations");
  return {
    html: `
      <div class="topbar"><div><h1>Automations</h1><p class="lede">Notifications and approved actions from events.</p></div></div>
      <div class="card table-wrap"><table><thead><tr><th>Name</th><th>Trigger</th><th>Rule</th><th>Action</th><th></th></tr></thead>
      <tbody>${rows.map(a => `<tr><td>${esc(a.name)}</td><td>${esc(a.trigger_kind)}</td><td>${esc(a.capability)} ${esc(a.operator)} ${a.threshold}</td><td>${esc(a.action)}</td>
        <td><button class="btn small ghost" data-id="${a.id}" data-on="${a.enabled}">${a.enabled?"Enabled":"Paused"}</button></td></tr>`).join("")}</tbody></table></div>`,
    bind() {
      document.querySelectorAll("button[data-id]").forEach(b => b.onclick = async () => {
        await api("/api/v1/automations", { method: "PATCH", body: JSON.stringify({ id: b.dataset.id, enabled: b.dataset.on !== "true" })});
        render();
      });
    }
  };
}

async function integrations() {
  const rows = await api("/api/v1/connectors");
  return { html: `
    <div class="topbar"><div><h1>Integrations</h1><p class="lede">Agents, HTTP ingestion, and optional Zyvor connectors.</p></div></div>
    <div class="grid" style="grid-template-columns:repeat(auto-fill,minmax(280px,1fr))">
      ${rows.map(c => `<div class="card"><h2>${esc(c.kind)}</h2><div style="font-size:18px;font-weight:650">${esc(c.name)}</div>
        <p class="lede">${esc(c.endpoint||"Not configured")}</p><p>${pill(c.status)}</p>
        <p class="lede">Actions: ${esc(c.actions)}</p>
        ${c.token_hint ? `<p class="lede">Token ${esc(c.token_hint)}</p>`:""}
        <p class="lede">Last sync ${fmt(c.last_sync_at)}</p></div>`).join("")}
    </div>` };
}

async function admin() {
  const rows = await api("/api/v1/audit");
  return { html: `
    <div class="topbar"><div><h1>Administration</h1><p class="lede">Workspace, connector credentials, and audit history.</p></div></div>
    <div class="card">${table(["When","Actor","Action","Object","Detail"], rows.map(r => [fmt(r.created_at), r.actor, r.action, r.object, r.detail]))}</div>` };
}

async function diagnostics() {
  const rows = await api("/api/v1/events");
  return {
    html: `
      <div class="topbar"><div><h1>Diagnostics</h1><p class="lede">Searchable operational log. Dark surface, readable type.</p></div></div>
      <div class="diag"><input id="dq" placeholder="Filter logs" /><pre id="dpre">${esc(rows.map(e => `${fmt(e.created_at)}  ${(e.severity||"").padEnd(8)}  ${(e.kind||"").padEnd(20)}  ${e.title}  ${e.body}`).join("\n") || "No events.")}</pre></div>`,
    bind() {
      document.getElementById("dq").oninput = (e) => {
        const q = e.target.value.toLowerCase();
        document.getElementById("dpre").textContent = rows.filter(r => (r.title+" "+r.body+" "+r.kind).toLowerCase().includes(q))
          .map(ev => `${fmt(ev.created_at)}  ${(ev.severity||"").padEnd(8)}  ${(ev.kind||"").padEnd(20)}  ${ev.title}  ${ev.body}`).join("\n") || "No matching events.";
      };
    }
  };
}

window.addEventListener("hashchange", render);
render();
