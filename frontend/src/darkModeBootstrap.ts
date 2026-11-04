import { applyDarkClass, getSystemPrefersDark } from "./darkMode";

// CSP-safe pre-hydration bootstrap (script-src 'self' module, no inline JS).
applyDarkClass(getSystemPrefersDark());
