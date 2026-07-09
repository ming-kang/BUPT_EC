import { describe, expect, it } from "vitest";
import {
  applyDarkClass,
  getSystemPrefersDark,
  resolveDarkMode,
} from "./darkMode";

describe("resolveDarkMode", () => {
  it("is true only when system prefers dark", () => {
    expect(resolveDarkMode(true)).toBe(true);
    expect(resolveDarkMode(false)).toBe(false);
  });

  it("treats non-true values as light", () => {
    expect(resolveDarkMode(undefined)).toBe(false);
    expect(resolveDarkMode(null)).toBe(false);
    expect(resolveDarkMode(0)).toBe(false);
    expect(resolveDarkMode("true")).toBe(false);
  });
});

describe("applyDarkClass", () => {
  it("adds and removes the dark class", () => {
    const classes = new Set();
    const el = {
      classList: {
        add: (c) => classes.add(c),
        remove: (c) => classes.delete(c),
      },
    };

    applyDarkClass(true, el);
    expect(classes.has("dark")).toBe(true);

    applyDarkClass(false, el);
    expect(classes.has("dark")).toBe(false);
  });

  it("no-ops when body is missing", () => {
    expect(() => applyDarkClass(true, null)).not.toThrow();
    expect(() => applyDarkClass(true, undefined)).not.toThrow();
  });
});

describe("getSystemPrefersDark", () => {
  it("returns false without matchMedia", () => {
    expect(getSystemPrefersDark({})).toBe(false);
    expect(getSystemPrefersDark(null)).toBe(false);
  });

  it("reads matchMedia prefers-color-scheme", () => {
    expect(
      getSystemPrefersDark({
        matchMedia: (q) => ({
          matches: q === "(prefers-color-scheme: dark)",
        }),
      })
    ).toBe(true);

    expect(
      getSystemPrefersDark({
        matchMedia: () => ({ matches: false }),
      })
    ).toBe(false);
  });
});
