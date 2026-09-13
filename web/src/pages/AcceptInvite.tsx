import { FormEvent, useState } from "react";
import { api, setToken } from "../lib/api";

export default function AcceptInvite({ onIn }: { onIn: () => void }) {
  const token = new URLSearchParams(window.location.search).get("token") || "";
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [err, setErr] = useState("");

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    if (password.length < 8) { setErr("Password must be at least 8 characters."); return; }
    if (password !== confirm) { setErr("Passwords don't match."); return; }
    try {
      const res = await api<{ token: string }>("/api/v1/auth/accept-invite", {
        method: "POST",
        body: JSON.stringify({ token, password }),
      });
      setToken(res.token);
      onIn();
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : "That invite link is invalid or has expired.");
    }
  }

  if (!token) {
    return (
      <div className="auth">
        <div className="auth-card">
          <h1>Yard</h1>
          <p className="lede">This invite link is missing its token. Ask whoever invited you to resend it.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="auth">
      <form className="auth-card" onSubmit={submit}>
        <img src="/logo.svg" width={40} height={40} alt="Zyvor" />
        <p className="kicker" style={{ marginTop: 16 }}>zyvor.dev</p>
        <h1>Yard</h1>
        <p className="lede">Set a password to finish joining the workspace.</p>
        <div className="field">
          <label htmlFor="password">Password</label>
          <input id="password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" />
        </div>
        <div className="field">
          <label htmlFor="confirm">Confirm password</label>
          <input id="confirm" type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="new-password" />
        </div>
        {err && <p style={{ color: "var(--bad)" }}>{err}</p>}
        <button className="btn accent" type="submit" style={{ width: "100%", marginTop: 8 }}>
          Set password and sign in
        </button>
      </form>
    </div>
  );
}
