// Applies the theme to <html data-theme> and keeps the browser/PWA chrome
// colour (status bar on iOS) in sync with it.

const colors: Record<string, string> = { light: "#eef1ec", sepia: "#efe6d3", night: "#111613" };

export function applyTheme(name: string) {
  const root = document.documentElement;
  root.dataset.theme = name || "system";
  const dark = window.matchMedia("(prefers-color-scheme: dark)").matches;
  const effective = name === "system" || !name ? (dark ? "night" : "light") : name;
  document.querySelectorAll('meta[name="theme-color"]').forEach(m => m.remove());
  const meta = document.createElement("meta");
  meta.name = "theme-color";
  meta.content = colors[effective] ?? colors.light;
  document.head.appendChild(meta);
}

export function watchSystemTheme(get: () => string) {
  window.matchMedia("(prefers-color-scheme: dark)").addEventListener?.("change", () => applyTheme(get()));
}
