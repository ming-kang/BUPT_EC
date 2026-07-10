import {
  applyDarkClass,
  getSystemPrefersDark,
  resolveDarkMode,
} from "./darkMode.js";

// CSP-safe pre-hydration bootstrap (script-src 'self' module, no inline JS).
applyDarkClass(resolveDarkMode(getSystemPrefersDark()));
