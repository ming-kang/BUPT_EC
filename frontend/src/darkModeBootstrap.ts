import {
  applyDarkClass,
  getSystemPrefersDark,
  resolveDarkMode,
} from "./darkMode";

// CSP-safe pre-hydration bootstrap (script-src 'self' module, no inline JS).
applyDarkClass(resolveDarkMode(getSystemPrefersDark()));
