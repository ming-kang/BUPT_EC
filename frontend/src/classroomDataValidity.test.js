import { describe, expect, it } from "vitest";
import { isUsableBusinessDaySnapshot } from "./classroomDataValidity";

const now = Date.parse("2026-07-10T12:00:00+08:00");

function snapshot(overrides = {}) {
  return {
    date: "2026-07-10",
    stale_until: "2026-07-10T23:59:59.999+08:00",
    campuses: [],
    ...overrides,
  };
}

describe("isUsableBusinessDaySnapshot", () => {
  it("accepts a same-day snapshot inside its stale window", () => {
    expect(isUsableBusinessDaySnapshot(snapshot(), now)).toBe(true);
  });

  it("rejects previous-day and expired snapshots", () => {
    expect(
      isUsableBusinessDaySnapshot(snapshot({ date: "2026-07-09" }), now)
    ).toBe(false);
    expect(
      isUsableBusinessDaySnapshot(
        snapshot({ stale_until: "2026-07-10T12:00:00+08:00" }),
        now
      )
    ).toBe(false);
  });

  it("fails closed for malformed required fields without throwing", () => {
    expect(isUsableBusinessDaySnapshot(null, now)).toBe(false);
    expect(
      isUsableBusinessDaySnapshot(snapshot({ campuses: null }), now)
    ).toBe(false);
    expect(
      isUsableBusinessDaySnapshot(snapshot({ stale_until: "not-a-time" }), now)
    ).toBe(false);
    expect(
      isUsableBusinessDaySnapshot(snapshot({ stale_until: undefined }), now)
    ).toBe(false);
    expect(isUsableBusinessDaySnapshot(snapshot(), Number.NaN)).toBe(false);
  });
});
