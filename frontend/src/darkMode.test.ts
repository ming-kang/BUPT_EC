import { describe, expect, it } from "vitest";
import {
  applyDarkClass,
  getSystemPrefersDark,
} from "./darkMode";

describe("applyDarkClass", () => {
  it("adds and removes the dark class", () => {
    const classes = new Set();
    const el = {
      classList: {
        add: (c: string) => classes.add(c),
        remove: (c: string) => classes.delete(c),
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
