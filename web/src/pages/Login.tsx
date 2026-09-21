import { FormEvent, useEffect, useState } from "react";
import { api, setToken } from "../lib/api";

export default function Login({ onIn }: { onIn: () => void }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [demo, setDemo] = useState(false);
  const [otp, setOtp] = useState("");
  const [needOtp, setNeedOtp] = useState(false);
  const [err, setErr] = useState("");
  const [forgot, setForgot] = useState(false);
  const [resetSent, setResetSent] = useState(false);

  useEffect(() => {
    fetch("/api/v1/meta")
      .then((r) => r.json())
      .then((m: { mode?: string }) => {
        if (m.mode === "demo") {
          setDemo(true);
          setEmail("admin@yard.local");
          setPassword("yard-admin");
        }
      })
      .catch(() => {});
  }, []);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      const res = await api<{ token: string }>("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password, otp }),
      });
      setToken(res.token);
      onIn();
    } catch (ex) {
      const text = ex instanceof Error ? ex.message : "";
      if (text.includes("otp")) {
        setNeedOtp(true);
        setErr("Enter the authenticator code.");
        return;
      }
      setErr("Those credentials were not accepted.");
    }
  }

  async function requestReset(e: FormEvent) {
    e.preventDefault();
    await api("/api/v1/auth/request-reset", { method: "POST", body: JSON.stringify({ email }) });
    setResetSent(true);
  }

  if (forgot) {
    return (
      <div className="auth">
        <form className="auth-card" onSubmit={requestReset}>
          <img src="/logo.svg" width={40} height={40} alt="Zyvor" />
          <p className="kicker" style={{ marginTop: 16 }}>zyvor.dev</p>
          <h1>Reset password</h1>
          {resetSent ? (
            <p className="lede">{demo
              ? "If that email has an account, an administrator can find the reset link in the server log — ask them for it."
              : "If that email has an account, a reset link has been sent."}</p>
          ) : (
            <>
              <p className="lede">{demo
                ? "Enter your email and ask your administrator to check the server log for the reset link."
                : "Enter your email. If an account exists, we will send a reset link."}</p>
              <div className="field">
                <label htmlFor="reset-email">Email</label>
                <input id="reset-email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="username" />
              </div>
              <button className="btn accent" type="submit" style={{ width: "100%", marginTop: 8 }}>
                Send reset link
              </button>
            </>
          )}
          <p className="hint"><a href="#" onClick={(e) => { e.preventDefault(); setForgot(false); setResetSent(false); }}>Back to sign in</a></p>
        </form>
      </div>
    );
  }

  return (
    <div className="auth">
      <form className="auth-card" onSubmit={submit}>
        <img src="/logo.svg" width={40} height={40} alt="Zyvor" />
        <p className="kicker" style={{ marginTop: 16 }}>zyvor.dev</p>
        <h1>Yard</h1>
        <p className="lede">Sign in to the asset and operations workspace.</p>
        <div className="field">
          <label htmlFor="email">Email</label>
          <input id="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="username" />
        </div>
        <div className="field">
          <label htmlFor="password">Password</label>
          <input id="password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" />
        </div>
        {needOtp && (
          <div className="field">
            <label htmlFor="otp">Authenticator code</label>
            <input id="otp" value={otp} onChange={(e) => setOtp(e.target.value)} inputMode="numeric" autoComplete="one-time-code" />
          </div>
        )}
        {err && <p style={{ color: "var(--bad)" }}>{err}</p>}
        <button className="btn accent" type="submit" style={{ width: "100%", marginTop: 8 }}>
          Continue
        </button>
        <p className="hint">
          {demo && <>Demo: admin@yard.local / yard-admin · </>}
          <a href="#" onClick={(e) => { e.preventDefault(); setForgot(true); }}>Forgot password?</a>
        </p>
      </form>
    </div>
  );
}
