import { describe, expect, it, vi } from "vitest";
import {
  FAILURE_RETRY_DELAYS_MS,
  JITTER_MAX_MS,
  JITTER_RATIO,
  MIN_FRESH_DELAY_MS,
  PARTIAL_POLL_MS,
  STALE_POLL_MS,
  failureRetryDelay,
  nextReloadDelay,
  normalizeRandomSample,
  withJitter,
} from "./reloadSchedule";

// sample=0 keeps positive jitter at the base interval for exact assertions.
const stable = { random: () => 0 };

function spreadFor(base) {
  return Math.min(base * JITTER_RATIO, JITTER_MAX_MS);
}

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

  it("clamps after jitter so sample=1 never exceeds stale_until", () => {
    const nowMs = Date.parse("2026-07-09T12:00:00+08:00");
    // hard deadline in 16s: base stale 15s + max positive jitter would be 16.5s
    // without post-jitter clamp.
    const hardMs = 16_000;
    const data = {
      date: "2026-07-09",
      expires_at: "2026-07-09T11:00:00+08:00",
      stale_until: new Date(nowMs + hardMs).toISOString(),
      campuses: [],
      stale: true,
    };
    const delay = nextReloadDelay(data, { nowMs, random: () => 1 });
    expect(delay).toBeLessThanOrEqual(hardMs);
    expect(delay).toBe(hardMs);
  });

  it("prefers hard deadline when it is earlier than the rate-limit floor", () => {
    const nowMs = Date.parse("2026-07-09T12:00:00+08:00");
    const hardMs = 4_000; // below STALE_POLL_MS
    const data = {
      date: "2026-07-09",
      expires_at: "2026-07-09T11:00:00+08:00",
      stale_until: new Date(nowMs + hardMs).toISOString(),
      campuses: [],
      stale: true,
    };
    expect(nextReloadDelay(data, { nowMs, random: () => 1 })).toBe(hardMs);
    expect(nextReloadDelay(data, { nowMs, random: () => 0 })).toBe(hardMs);
  });

  it("reads the random source exactly once per delay computation", () => {
    const random = vi.fn(() => 0.25);
    nextReloadDelay(
      {
        date: "2026-07-09",
        expires_at: "2026-07-09T11:00:00+08:00",
        stale_until: "2026-07-09T23:59:59.999+08:00",
        campuses: [],
        stale: true,
      },
      { nowMs: now, random }
    );
    expect(random).toHaveBeenCalledTimes(1);
  });
});

describe("withJitter", () => {
  it("applies positive-only bounded jitter for samples 0 / 0.5 / 1", () => {
    const base = 10_000;
    const spread = spreadFor(base);
    expect(spread).toBe(1_000);
    expect(withJitter(base, () => 0)).toBe(base);
    expect(withJitter(base, () => 0.5)).toBe(Math.round(base + 0.5 * spread));
    expect(withJitter(base, () => 1)).toBe(base + spread);
  });

  it("caps absolute jitter at 5 seconds for long base delays", () => {
    const base = 4 * 60 * 1000;
    expect(spreadFor(base)).toBe(JITTER_MAX_MS);
    expect(withJitter(base, () => 1)).toBe(base + JITTER_MAX_MS);
  });

  it("never shortens the base minimum interval", () => {
    for (const base of [STALE_POLL_MS, PARTIAL_POLL_MS, 10_000, 60_000]) {
      for (const sample of [0, 0.5, 1]) {
        expect(withJitter(base, () => sample)).toBeGreaterThanOrEqual(base);
      }
    }
  });

  it("handles throwing, NaN, Inf, and out-of-range random sources", () => {
    const base = 10_000;
    const mid = Math.round(base + 0.5 * spreadFor(base));
    expect(withJitter(base, () => Number.NaN)).toBe(mid);
    expect(withJitter(base, () => Number.POSITIVE_INFINITY)).toBe(mid);
    expect(withJitter(base, () => Number.NEGATIVE_INFINITY)).toBe(mid);
    expect(withJitter(base, () => -3)).toBe(base);
    expect(withJitter(base, () => 2)).toBe(base + spreadFor(base));
    expect(
      withJitter(base, () => {
        throw new Error("rng broken");
      })
    ).toBe(mid);
    expect(withJitter(base, null)).toBe(mid);
    expect(withJitter(null, () => 1)).toBeNull();
    expect(withJitter(-1, () => 1)).toBe(-1);
  });

  it("normalizeRandomSample matches withJitter fallbacks", () => {
    expect(normalizeRandomSample(() => 0.2)).toBe(0.2);
    expect(normalizeRandomSample(() => Number.NaN)).toBe(0.5);
    expect(normalizeRandomSample(() => 3)).toBe(1);
    expect(
      normalizeRandomSample(() => {
        throw new Error("boom");
      })
    ).toBe(0.5);
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
