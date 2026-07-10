import { describe, expect, it } from "vitest";
import {
  FAILURE_RETRY_DELAYS_MS,
  MIN_FRESH_DELAY_MS,
  PARTIAL_POLL_MS,
  STALE_POLL_MS,
  failureRetryDelay,
  nextReloadDelay,
} from "./reloadSchedule";

describe("nextReloadDelay", () => {
  const now = Date.parse("2026-07-09T12:00:00+08:00");

  it("does not schedule before the first result, but retries malformed data", () => {
    expect(nextReloadDelay(null, { nowMs: now })).toBeNull();
    expect(nextReloadDelay({}, { nowMs: now })).toBe(MIN_FRESH_DELAY_MS);
  });

  it("polls quickly when stale", () => {
    expect(
      nextReloadDelay(
        {
          date: "2026-07-09",
          expires_at: "2026-07-09T11:00:00+08:00",
          stale_until: "2026-07-09T23:59:59.999+08:00",
          campuses: [],
          stale: true,
        },
        { nowMs: now }
      )
    ).toBe(STALE_POLL_MS);
  });

  it("aligns partial payload polling with the backend backoff", () => {
    expect(
      nextReloadDelay(
        {
          date: "2026-07-09",
          expires_at: "2026-07-09T12:10:00+08:00",
          stale_until: "2026-07-09T23:59:59.999+08:00",
          campuses: [],
          stale: false,
          partial_campuses: ["04"],
          error: { type: "jw_query_failed", message: "partial" },
        },
        { nowMs: now }
      )
    ).toBe(PARTIAL_POLL_MS);

    // Older servers may omit partial_campuses; a fresh payload with an error
    // still follows the partial-refresh interval.
    expect(
      nextReloadDelay(
        {
          date: "2026-07-09",
          expires_at: "2026-07-09T12:10:00+08:00",
          stale_until: "2026-07-09T23:59:59.999+08:00",
          campuses: [],
          stale: false,
          error: { type: "jw_query_failed", message: "partial" },
        },
        { nowMs: now }
      )
    ).toBe(PARTIAL_POLL_MS);
  });

  it("waits until expires_at for fresh same-day data with a small floor", () => {
    expect(
      nextReloadDelay(
        {
          date: "2026-07-09",
          expires_at: "2026-07-09T12:04:00+08:00",
          stale_until: "2026-07-09T23:59:59.999+08:00",
          campuses: [],
          stale: false,
        },
        { nowMs: now }
      )
    ).toBe(4 * 60 * 1000);

    expect(
      nextReloadDelay(
        {
          date: "2026-07-09",
          expires_at: "2026-07-09T11:59:00+08:00",
          stale_until: "2026-07-09T23:59:59.999+08:00",
          campuses: [],
          stale: false,
        },
        { nowMs: now }
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
          campuses: [],
          stale: false,
        },
        { nowMs: afterMidnight }
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
          campuses: [],
          stale: false,
        },
        { nowMs: pastStale }
      )
    ).toBe(MIN_FRESH_DELAY_MS);

    // Strictly after stale_until as well
    expect(
      nextReloadDelay(
        {
          date: "2026-07-09",
          expires_at: "2026-07-10T00:04:00+08:00",
          stale_until: "2026-07-09T23:59:59.999+08:00",
          campuses: [],
          stale: false,
        },
        { nowMs: pastStale + 1 }
      )
    ).toBe(MIN_FRESH_DELAY_MS);
  });

  it("wakes at stale_until even when fresh expiry or failure backoff is later", () => {
    const beforeMidnight = Date.parse("2026-07-09T23:59:58+08:00");
    const data = {
      date: "2026-07-09",
      expires_at: "2026-07-10T00:04:00+08:00",
      stale_until: "2026-07-09T23:59:59.999+08:00",
      campuses: [],
      stale: false,
    };

    expect(nextReloadDelay(data, { nowMs: beforeMidnight })).toBe(1_999);
    expect(
      nextReloadDelay(data, { failureCount: 4, nowMs: beforeMidnight })
    ).toBe(1_999);
  });
});

describe("failureRetryDelay", () => {
  it("backs off at 5s, 10s, 20s, then caps at 30s", () => {
    expect([1, 2, 3, 4, 5].map(failureRetryDelay)).toEqual([
      ...FAILURE_RETRY_DELAYS_MS,
      30_000,
    ]);
  });

  it("schedules hard-empty and client refresh failures", () => {
    expect(nextReloadDelay(null, { failureCount: 1 })).toBe(5_000);
    expect(
      nextReloadDelay(
        {
          date: "2026-07-09",
          stale_until: "2026-07-09T23:59:59.999+08:00",
          campuses: [],
          error: { type: "client_refresh_failed", message: "offline" },
        },
        { failureCount: 3, nowMs: Date.parse("2026-07-09T12:00:00+08:00") }
      )
    ).toBe(20_000);
  });
});
