import { describe, expect, it } from "vitest";
import {
  MIN_FRESH_DELAY_MS,
  STALE_POLL_MS,
  nextReloadDelay,
} from "./reloadSchedule";

describe("nextReloadDelay", () => {
  const now = Date.parse("2026-07-09T12:00:00+08:00");

  it("returns null without expires_at", () => {
    expect(nextReloadDelay({}, now)).toBeNull();
    expect(nextReloadDelay(null, now)).toBeNull();
  });

  it("polls quickly when stale", () => {
    expect(
      nextReloadDelay(
        {
          date: "2026-07-09",
          expires_at: "2026-07-09T11:00:00+08:00",
          stale_until: "2026-07-09T23:59:59.999+08:00",
          stale: true,
        },
        now
      )
    ).toBe(STALE_POLL_MS);
  });

  it("polls quickly when partial error is present", () => {
    expect(
      nextReloadDelay(
        {
          date: "2026-07-09",
          expires_at: "2026-07-09T12:10:00+08:00",
          stale_until: "2026-07-09T23:59:59.999+08:00",
          stale: false,
          error: { type: "jw_query_failed", message: "partial" },
        },
        now
      )
    ).toBe(STALE_POLL_MS);
  });

  it("waits until expires_at for fresh same-day data with a small floor", () => {
    expect(
      nextReloadDelay(
        {
          date: "2026-07-09",
          expires_at: "2026-07-09T12:04:00+08:00",
          stale_until: "2026-07-09T23:59:59.999+08:00",
          stale: false,
        },
        now
      )
    ).toBe(4 * 60 * 1000);

    expect(
      nextReloadDelay(
        {
          date: "2026-07-09",
          expires_at: "2026-07-09T11:59:00+08:00",
          stale_until: "2026-07-09T23:59:59.999+08:00",
          stale: false,
        },
        now
      )
    ).toBe(MIN_FRESH_DELAY_MS);
  });

  it("reloads ASAP when payload date is not today's Shanghai business date", () => {
    // Just after Shanghai midnight; payload still has yesterday's date and a
    // future expires_at left over from the previous evening's fresh TTL.
    const afterMidnight = Date.parse("2026-07-10T00:01:00+08:00");
    expect(
      nextReloadDelay(
        {
          date: "2026-07-09",
          expires_at: "2026-07-10T00:04:00+08:00",
          stale_until: "2026-07-09T23:59:59.999+08:00",
          stale: false,
        },
        afterMidnight
      )
    ).toBe(MIN_FRESH_DELAY_MS);
  });

  it("reloads ASAP when now is past stale_until", () => {
    const pastStale = Date.parse("2026-07-09T23:59:59.999+08:00");
    expect(
      nextReloadDelay(
        {
          date: "2026-07-09",
          // expires_at still in the future (would otherwise wait)
          expires_at: "2026-07-10T00:04:00+08:00",
          stale_until: "2026-07-09T23:59:59.999+08:00",
          stale: false,
        },
        pastStale
      )
    ).toBe(MIN_FRESH_DELAY_MS);

    // Strictly after stale_until as well
    expect(
      nextReloadDelay(
        {
          date: "2026-07-09",
          expires_at: "2026-07-10T00:04:00+08:00",
          stale_until: "2026-07-09T23:59:59.999+08:00",
          stale: false,
        },
        pastStale + 1
      )
    ).toBe(MIN_FRESH_DELAY_MS);
  });
});
