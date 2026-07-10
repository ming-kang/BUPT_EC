import { describe, expect, it } from "vitest";
import {
  FAILURE_RETRY_DELAYS_MS,
  MIN_FRESH_DELAY_MS,
  PARTIAL_POLL_MS,
  STALE_POLL_MS,
  failureRetryDelay,
  nextReloadDelay,
  withJitter,
} from "./reloadSchedule";

// Mid-range sample keeps withJitter() net-zero so delay assertions stay exact.
const stable = { random: () => 0.5 };

describe("nextReloadDelay", () => {
  const now = Date.parse("2026-07-09T12:00:00+08:00");

  it("does not schedule before the first result, but retries malformed data", () => {
    expect(nextReloadDelay(null, { nowMs: now, ...stable })).toBeNull();
    expect(nextReloadDelay({}, { nowMs: now, ...stable })).toBe(MIN_FRESH_DELAY_MS);
  });

  it("polls on a rate-aware stale interval", () => {
    expect(
      nextReloadDelay(
        {
          date: "2026-07-09",
          expires_at: "2026-07-09T11:00:00+08:00",
          stale_until: "2026-07-09T23:59:59.999+08:00",
          campuses: [],
          stale: true,
        },
        { nowMs: now, ...stable }
      )
    ).toBe(STALE_POLL_MS);
    expect(STALE_POLL_MS).toBe(15_000);
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
        { nowMs: now, ...stable }
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
        { nowMs: now, ...stable }
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
        { nowMs: now, ...stable }
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
        { nowMs: now, ...stable }
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
        { nowMs: afterMidnight, ...stable }
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
        { nowMs: pastStale, ...stable }
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
        { nowMs: pastStale + 1, ...stable }
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

    expect(nextReloadDelay(data, { nowMs: beforeMidnight, ...stable })).toBe(1_999);
    expect(
      nextReloadDelay(data, { failureCount: 4, nowMs: beforeMidnight, ...stable })
    ).toBe(1_999);
  });

  it("applies bounded jitter when a random source is provided", () => {
    expect(withJitter(10_000, () => 0)).toBe(9_000);
    expect(withJitter(10_000, () => 1)).toBe(11_000);
  });
});

describe("failureRetryDelay", () => {
  it("backs off at 10s, 20s, 30s, then caps at 60s", () => {
    expect([1, 2, 3, 4, 5].map(failureRetryDelay)).toEqual([
      ...FAILURE_RETRY_DELAYS_MS,
      60_000,
    ]);
  });

  it("schedules hard-empty and client refresh failures", () => {
    expect(nextReloadDelay(null, { failureCount: 1, ...stable })).toBe(10_000);
    expect(
      nextReloadDelay(
        {
          date: "2026-07-09",
          stale_until: "2026-07-09T23:59:59.999+08:00",
          campuses: [],
          error: { type: "client_refresh_failed", message: "offline" },
        },
        {
          failureCount: 3,
          nowMs: Date.parse("2026-07-09T12:00:00+08:00"),
          ...stable,
        }
      )
    ).toBe(30_000);
  });
});
