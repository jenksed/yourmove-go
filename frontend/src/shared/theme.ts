// Applies the stored appearance choice shared with legacy Go-rendered pages
// (same key and values as static/ui.js) so the React slice honors an
// explicit light/dark selection and otherwise follows the system theme.
const THEME_KEY = "yourmove:theme";
const LIGHT_META = "#f5f4f1";
const DARK_META = "#191c22";

type ThemeChoice = "light" | "dark" | "system";

function resolveTheme(): ThemeChoice {
  try {
    const stored = window.localStorage.getItem(THEME_KEY);
    if (stored === "light" || stored === "dark") return stored;
  } catch {
    // Storage unavailable: fall through to the system theme.
  }
  return "system";
}

function isDark(theme: ThemeChoice): boolean {
  if (theme === "dark") return true;
  if (theme === "light") return false;
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

export function applyStoredTheme(): void {
  const theme = resolveTheme();
  if (theme === "system") {
    document.documentElement.removeAttribute("data-theme");
  } else {
    document.documentElement.dataset.theme = theme;
  }
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute("content", isDark(theme) ? DARK_META : LIGHT_META);
  if (window.matchMedia) {
    window
      .matchMedia("(prefers-color-scheme: dark)")
      .addEventListener?.("change", () => applyStoredTheme());
  }
}
