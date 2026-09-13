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
      </div>
    </>
  );
}
