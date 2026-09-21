import { FormEvent, useEffect, useState } from "react";
import { api } from "../lib/api";
import { useTheme } from "../lib/theme";
import { GroupedList, GroupedRow } from "../components/GroupedList";

function AppearanceIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="1.75" />
      <path d="M12 3a9 9 0 0 1 0 18Z" fill="currentColor" />
    </svg>
  );
}

export default function Settings() {
  const { theme, setTheme } = useTheme();
  const [admin, setAdmin] = useState(false);
  const [days, setDays] = useState("0");
  const [msg, setMsg] = useState("");
  const [otpSecret, setOtpSecret] = useState("");
  const [otpCode, setOtpCode] = useState("");

  useEffect(() => {
    void (async () => {
      const [me, org] = await Promise.all([
        api<{ role: string }>("/api/v1/auth/me"),
        api<{ retention_days: number }>("/api/v1/org"),
      ]);
      setAdmin(me.role === "admin");
      setDays(String(org.retention_days ?? 0));
    })();
  }, []);

  async function startOtp() {
    const row = await api<{ secret: string }>("/api/v1/auth/totp/setup", { method: "POST", body: "{}" });
    setOtpSecret(row.secret);
    setMsg("Confirm the code from your authenticator.");
  }
  async function confirmOtp(e: FormEvent) {
    e.preventDefault();
    await api("/api/v1/auth/totp/confirm", { method: "POST", body: JSON.stringify({ code: otpCode }) });
    setOtpSecret("");
    setMsg("Authenticator codes are required at sign-in.");
  }
  async function saveRetention(e: FormEvent) {
    e.preventDefault();
    setMsg("");
    try {
      await api("/api/v1/org", { method: "PATCH", body: JSON.stringify({ retention_days: Number(days) }) });
      setMsg(Number(days) === 0 ? "Observations are kept" : `Observations older than ${days} days are removed`);
    } catch (ex) {
      setMsg(ex instanceof Error ? ex.message : "save failed");
    }
  }

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Settings</h1>
          <p className="lede">Appearance and session preferences for this console.</p>
        </div>
      </div>
      <div className="settings-stack">
        <GroupedList title="Appearance">
          <GroupedRow
            icon={<AppearanceIcon />}
            label="Appearance"
            description="Light or dark surfaces. The map basemap follows this setting."
            trailing={
              <div className="theme-toggle" role="group" aria-label="Theme">
                <button type="button" className={theme === "light" ? "active" : ""} onClick={() => setTheme("light")}>
                  Light
                </button>
                <button type="button" className={theme === "dark" ? "active" : ""} onClick={() => setTheme("dark")}>
                  Dark
                </button>
              </div>
            }
          />
        </GroupedList>
        {admin && (
          <form className="card" onSubmit={saveRetention} style={{ marginTop: 16 }}>
            <h2>Telemetry retention</h2>
            <p className="lede">0 keeps every observation. Any other number deletes readings older than that many days. The leader replica runs the purge about once an hour.</p>
            <label>Days
              <input type="number" min={0} max={3650} value={days} onChange={(e) => setDays(e.target.value)} />
            </label>
            <button className="btn small accent" type="submit" style={{ marginTop: 8 }}>Save</button>
            {msg && <p className="lede">{msg}</p>}
          </form>
        )}
        <form className="card" onSubmit={confirmOtp} style={{ marginTop: 16 }}>
          <h2>Authenticator</h2>
          <p className="lede">After you confirm a code, sign-in asks for it.</p>
          {!otpSecret && <button className="btn small" type="button" onClick={startOtp}>Set up</button>}
          {otpSecret && (
            <>
              <p className="lede">Secret: {otpSecret}</p>
              <label>Code
                <input value={otpCode} onChange={(e) => setOtpCode(e.target.value)} inputMode="numeric" />
              </label>
              <button className="btn small accent" type="submit" style={{ marginTop: 8 }}>Confirm</button>
            </>
          )}
        </form>
      </div>
    </>
  );
}
