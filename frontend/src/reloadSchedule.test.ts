import { describe, expect, it, vi } from "vitest";
import { nextReloadDelay } from "./reloadSchedule";

// sample=0 keeps positive jitter at the base interval for exact assertions.
const stable = { random: () => 0 };

describe("nextReloadDelay", () => {
  const now = Date.parse("2026-07-09T12:00:00+08:00");

  it("does not schedule before the first result, but retries malformed data", () => {
    expect(nextReloadDelay(null, { nowMs: now, ...stable })).toBeNull();
    expect(nextReloadDelay({}, { nowMs: now, ...stable })).toBe(1_000);
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
    ).toBe(15_000);
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
    ).toBe(30_000);

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
    ).toBe(30_000);
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

    // expires_at already passed: reload at the 1s floor.
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
    ).toBe(1_000);
  });

  it("reloads ASAP when the payload date is not today's Shanghai business date", () => {
    // Just after Shanghai midnight; the payload still carries yesterday's
    // date and a future expires_at from the previous evening's fresh TTL.
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
    ).toBe(1_000);
  });

  it("reloads ASAP when now is at or past stale_until", () => {
    const pastStale = Date.parse("2026-07-09T23:59:59.999+08:00");
    const data = {
      date: "2026-07-09",
      // expires_at still in the future (would otherwise wait)
      expires_at: "2026-07-10T00:04:00+08:00",
      stale_until: "2026-07-09T23:59:59.999+08:00",
      campuses: [],
      stale: false,
    };

    expect(nextReloadDelay(data, { nowMs: pastStale, ...stable })).toBe(1_000);
    expect(nextReloadDelay(data, { nowMs: pastStale + 1, ...stable })).toBe(1_000);
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
    // hard deadline in 16s: base stale 15s + max positive jitter (1.5s) would
    // overshoot without the post-jitter clamp.
    const hardMs = 16_000;
    const data = {
      date: "2026-07-09",
      expires_at: "2026-07-09T11:00:00+08:00",
      stale_until: new Date(nowMs + hardMs).toISOString(),
      campuses: [],
      stale: true,
    };
    expect(nextReloadDelay(data, { nowMs, random: () => 1 })).toBe(hardMs);
  });

  it("prefers the hard deadline when it is earlier than the rate-limit floor", () => {
    const nowMs = Date.parse("2026-07-09T12:00:00+08:00");
    const hardMs = 4_000; // below the 15s stale poll
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

  it("keeps positive-only bounded jitter on every schedule", () => {
    const nowMs = Date.parse("2026-07-09T12:00:00+08:00");
    const stale = {
      date: "2026-07-09",
      expires_at: "2026-07-09T11:00:00+08:00",
      stale_until: "2026-07-09T23:59:59.999+08:00",
      campuses: [],
      stale: true,
    };
    // 15s base: spread is 10% → [15s, 16.5s].
    expect(nextReloadDelay(stale, { nowMs, random: () => 0 })).toBe(15_000);
    expect(nextReloadDelay(stale, { nowMs, random: () => 1 })).toBe(16_500);
    expect(nextReloadDelay(stale, { nowMs, random: () => 0.5 })).toBe(15_750);
    // Garbage samples fall back to the midpoint instead of NaN delays.
    expect(
      nextReloadDelay(stale, { nowMs, random: () => Number.NaN })
    ).toBe(15_750);
    expect(nextReloadDelay(stale, { nowMs, random: null })).toBe(15_750);
    // Out-of-range samples clamp into [0, 1].
    expect(nextReloadDelay(stale, { nowMs, random: () => -3 })).toBe(15_000);
    expect(nextReloadDelay(stale, { nowMs, random: () => 9 })).toBe(16_500);
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

describe("failure backoff", () => {
  const nowMs = Date.parse("2026-07-09T12:00:00+08:00");
  // A still-displayable snapshot clamps the ladder to its hard deadline;
  // no snapshot leaves the ladder unclamped.
  const snapshot = {
    date: "2026-07-09",
    expires_at: "2026-07-09T11:00:00+08:00",
    stale_until: "2026-07-09T23:59:59.999+08:00",
    campuses: [],
    stale: true,
  };

  it("backs off at 10s, 20s, 30s, then caps at 60s", () => {
    expect(
      [1, 2, 3, 4, 5, 9].map((failureCount) =>
        nextReloadDelay(snapshot, { failureCount, nowMs, ...stable })
      )
    ).toEqual([10_000, 20_000, 30_000, 60_000, 60_000, 60_000]);
  });

  it("paces hard-empty failures too (no snapshot to clamp against)", () => {
    expect(nextReloadDelay(null, { failureCount: 1, ...stable })).toBe(10_000);
    expect(nextReloadDelay(null, { failureCount: 3, ...stable })).toBe(30_000);
  });

  it("clamps the ladder to a displayable snapshot's stale_until", () => {
    const nearDeadline = {
      ...snapshot,
      stale_until: new Date(nowMs + 3_000).toISOString(),
    };
    // Even with maximum jitter, the retry wakes at the hard deadline.
    expect(
      nextReloadDelay(nearDeadline, { failureCount: 1, nowMs, random: () => 1 })
    ).toBe(3_000);
    expect(
      nextReloadDelay(nearDeadline, { failureCount: 4, nowMs, random: () => 1 })
    ).toBe(3_000);
  });
});
