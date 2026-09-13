import { useTheme } from "../lib/theme";

export default function Settings() {
  const { theme, setTheme } = useTheme();

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Settings</h1>
          <p className="lede">Appearance and session preferences for this console.</p>
        </div>
      </div>
      <div className="card form-card" style={{ maxWidth: 420 }}>
        <h2>Appearance</h2>
        <p className="lede" style={{ marginBottom: 12 }}>
          Choose light or dark surfaces. The map basemap follows this setting.
        </p>
        <div className="theme-toggle" role="group" aria-label="Theme">
          <button type="button" className={theme === "light" ? "active" : ""} onClick={() => setTheme("light")}>
            Light
          </button>
          <button type="button" className={theme === "dark" ? "active" : ""} onClick={() => setTheme("dark")}>
            Dark
          </button>
        </div>
      </div>
    </>
  );
}
