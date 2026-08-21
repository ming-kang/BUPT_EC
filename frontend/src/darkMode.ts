/**
 * Single source of truth for dark mode: OS/browser prefers-color-scheme.
 * There is no in-app toggle; user preference is not persisted.
 */

/** True only for the literal boolean true (mirrors the JS `=== true` guard). */
export function resolveDarkMode(systemPrefersDark: unknown): boolean {
  return systemPrefersDark === true;
}

interface ClassListLike {
  add(name: string): void;
  remove(name: string): void;
}

/**
 * Apply or remove the `dark` class on a document body element.
 */
export function applyDarkClass(
  isDark: boolean,
  body?: { classList?: ClassListLike } | null
): void {
  const el =
    body ?? (typeof document !== "undefined" ? document.body : null);
  if (!el || !el.classList) return;
  if (isDark) {
    el.classList.add("dark");
  } else {
    el.classList.remove("dark");
  }
}

interface MatchMediaHost {
  matchMedia?(query: string): { matches: boolean };
}

/**
 * Read the current system color-scheme preference.
 * Safe when matchMedia is missing.
 */
export function getSystemPrefersDark(win?: MatchMediaHost | null): boolean {
  const w = win ?? (typeof window !== "undefined" ? window : null);
  if (!w || typeof w.matchMedia !== "function") return false;
  return w.matchMedia("(prefers-color-scheme: dark)").matches;
}
