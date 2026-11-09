import { useEffect, useState } from "react";
import { useLeaveGuard } from "./leaveGuard";

type ThemeChoice = "light" | "dark" | "system";

const THEME_KEY = "yourmove:theme";

function currentTheme(): ThemeChoice {
  try {
    const stored = window.localStorage.getItem(THEME_KEY);
    if (stored === "light" || stored === "dark") return stored;
  } catch {
    // Storage unavailable: fall through to system.
  }
  return "system";
}

export function AppTopbar(): React.JSX.Element {
  const [theme, setTheme] = useState<ThemeChoice>("system");

  useEffect(() => {
    setTheme(currentTheme());
  }, []);

  const handleChange = (next: ThemeChoice) => {
    setTheme(next);
    if (next === "system") {
      document.documentElement.removeAttribute("data-theme");
    } else {
      document.documentElement.dataset.theme = next;
    }
    try {
      window.localStorage.setItem(THEME_KEY, next);
    } catch {
      // Storage unavailable: the explicit choice applies for this load only.
    }
  };

  return (
    <header className="topbar">
      <div className="mobile-wordmark">
        <a href="/jobs">YOURMOVE</a>
      </div>
      <label className="theme-control">
        Appearance{" "}
        <select
          aria-label="Appearance"
          value={theme}
          onChange={(event) => handleChange(event.target.value as ThemeChoice)}
        >
          <option value="system">System</option>
          <option value="light">Light</option>
          <option value="dark">Dark</option>
        </select>
      </label>
    </header>
  );
}

export function AppNavigation({ active }: { active?: "opportunities" | "documents" | "discovery" | "studio" }): React.JSX.Element {
  const { intercept, banner } = useLeaveGuard();
  return (
      <aside className="sidebar" aria-label="Primary navigation" onClick={intercept}>
        <a className="brand" href="/jobs" aria-label="YourMove — Opportunities">
          <img
            className="brand-wordmark brand-light"
            src="/static/yourmove-wordmark-primary.svg"
            alt="YourMove"
            width="166"
            height="49"
          />
          <img
            className="brand-wordmark brand-dark"
            src="/static/yourmove-wordmark-reverse.svg"
            alt=""
            width="166"
            height="49"
          />
        </a>
        <nav className="primary-nav" aria-label="Workspace">
          <div className="nav-group">
            <p className="nav-caption">Work</p>
            <a className={active === "opportunities" ? "nav-item is-active" : "nav-item"} aria-current={active === "opportunities" ? "page" : undefined} href="/jobs">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <rect x="3" y="7" width="18" height="14" rx="2" />
                <path d="M8 7V5a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M3 13h18" />
              </svg>
              Opportunities
            </a>
            <a className="nav-item" href="/applications">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="m4 12 5 5L20 6" />
                <path d="M20 13v6a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h10" />
              </svg>
              Submissions
            </a>
          </div>
          <div className="nav-group">
            <p className="nav-caption">Your materials</p>
            <a className={active === "documents" ? "nav-item is-active" : "nav-item"} aria-current={active === "documents" ? "page" : undefined} href="/resumes">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8zM14 2v6h6M8 13h8M8 17h8" />
              </svg>
              Documents
            </a>
            <a className="nav-item" href="/candidate">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <circle cx="12" cy="8" r="4" />
                <path d="M4 21v-3a8 8 0 0 1 16 0v3" />
              </svg>
              Candidate profile
            </a>
            <a className="nav-item" href="/answer-library">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M5 4h14v16H5zM8 8h8M8 12h8M8 16h5" />
              </svg>
            Answer Library
          </a>
            <a className="nav-item" href="/career-evidence">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M4 19V5m0 14h16M7 15l4-4 3 2 5-6" />
                <circle cx="7" cy="15" r="1" />
                <circle cx="11" cy="11" r="1" />
                <circle cx="14" cy="13" r="1" />
                <circle cx="19" cy="7" r="1" />
              </svg>
              Career Evidence
            </a>
            {/* Career Discovery and Career Studio are parked for this
                version: no menu entry. Their components and Go handlers
                stay intact; see careerDiscoveryEnabled in main.tsx. */}
          </div>
          <div className="nav-group">
            <p className="nav-caption">Preferences and tools</p>
            <a className="nav-item" href="/ai">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="m12 2 1.6 5.4L19 9l-5.4 1.6L12 16l-1.6-5.4L5 9l5.4-1.6L12 2Z" />
                <path d="m19 16 .6 2.4L22 19l-2.4.6L19 22l-.6-2.4L16 19l2.4-.6L19 16Z" />
              </svg>
              AI connections
            </a>
            <a className="nav-item" href="/utilities">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M12 3v3m0 12v3m9-9h-3M6 12H3m15.4-6.4-2.1 2.1M7.7 16.3l-2.1 2.1m12.8 0-2.1-2.1M7.7 7.7 5.6 5.6" />
                <circle cx="12" cy="12" r="4" />
              </svg>
              Utilities
            </a>
          </div>
        </nav>
        <div className="sidebar-bottom">
          <span className="local-dot" aria-hidden="true" />
          <span>Local workspace</span>
        </div>
        {banner}
      </aside>
  );
}
