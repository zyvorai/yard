import { FormEvent, useState } from "react";
import { api, setToken } from "../lib/api";

export default function Login({ onIn }: { onIn: () => void }) {
  const [email, setEmail] = useState("admin@estate.local");
  const [password, setPassword] = useState("estate-admin");
  const [err, setErr] = useState("");
  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      const res = await api<{ token: string }>("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password }),
      });
      setToken(res.token);
      onIn();
    } catch {
      setErr("Those credentials were not accepted.");
    }
  }
  return (
    <div className="auth">
      <form className="auth-card" onSubmit={submit}>
        <img src="/logo.svg" width={40} height={40} alt="Zyvor" />
        <p className="kicker" style={{ marginTop: 16 }}>Zyvor</p>
        <h1>Estate</h1>
        <p className="lede">Sign in to the asset and operations workspace.</p>
        <div className="field">
          <label htmlFor="email">Email</label>
          <input id="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="username" />
        </div>
        <div className="field">
          <label htmlFor="password">Password</label>
          <input id="password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" />
        </div>
        {err && <p style={{ color: "var(--bad)" }}>{err}</p>}
        <button className="btn accent" type="submit" style={{ width: "100%", marginTop: 8 }}>
          Continue
        </button>
        <p className="hint">Demo: admin@estate.local / estate-admin</p>
      </form>
    </div>
  );
}
