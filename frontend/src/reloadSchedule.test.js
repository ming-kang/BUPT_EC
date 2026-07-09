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
          expires_at: "2026-07-09T11:00:00+08:00",
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
          expires_at: "2026-07-09T12:10:00+08:00",
          stale: false,
          error: { type: "jw_query_failed", message: "partial" },
        },
        now
      )
    ).toBe(STALE_POLL_MS);
  });

  it("waits until expires_at for fresh data with a small floor", () => {
    expect(
      nextReloadDelay(
        {
          expires_at: "2026-07-09T12:04:00+08:00",
          stale: false,
        },
        now
      )
    ).toBe(4 * 60 * 1000);

    expect(
      nextReloadDelay(
        {
          expires_at: "2026-07-09T11:59:00+08:00",
          stale: false,
        },
        now
      )
    ).toBe(MIN_FRESH_DELAY_MS);
  });
});
