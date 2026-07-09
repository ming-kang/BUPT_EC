/**
 * Single source of truth for dark mode: OS/browser prefers-color-scheme.
 * There is no in-app toggle; user preference is not persisted.
 *
 * @param {boolean} systemPrefersDark - result of matchMedia("(prefers-color-scheme: dark)").matches
 * @returns {boolean}
 */
export function resolveDarkMode(systemPrefersDark) {
  return systemPrefersDark === true;
}

/**
 * Apply or remove the `dark` class on a document body element.
 * @param {boolean} isDark
 * @param {Element | null | undefined} [body]
 */
export function applyDarkClass(isDark, body) {
  const el =
    body ?? (typeof document !== "undefined" ? document.body : null);
  if (!el || !el.classList) return;
  if (isDark) {
    el.classList.add("dark");
  } else {
    el.classList.remove("dark");
  }
}

/**
 * Read the current system color-scheme preference.
 * Safe when matchMedia is missing.
 * @param {Window | null | undefined} [win]
 * @returns {boolean}
 */
export function getSystemPrefersDark(win) {
  const w = win ?? (typeof window !== "undefined" ? window : null);
  if (!w || typeof w.matchMedia !== "function") return false;
  return w.matchMedia("(prefers-color-scheme: dark)").matches;
}
