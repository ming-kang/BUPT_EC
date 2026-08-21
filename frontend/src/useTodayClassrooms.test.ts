import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  hasUsableClassroomData,
  mergeFetchResult,
  pollingInterval,
  retryDelayFor,
  shouldFullPageSpin,
} from "./useTodayClassrooms";
import { fallbackErrorMessage } from "./todayClassroomsResponse";
import {
  MIN_FRESH_DELAY_MS,
  PARTIAL_POLL_MS,
  STALE_POLL_MS,
} from "./reloadSchedule";

const now = Date.parse("2026-07-10T12:00:00+08:00");

const goodPrev = {
  code: 0,
  msg: "ok",
  data: {
    date: "2026-07-10",
    expires_at: "2026-07-10T12:05:00+08:00",
    stale_until: "2026-07-10T23:59:59.999+08:00",
    campuses: [
      {
        id: "04",
        name: "沙河",
        buildings: [{ name: "S1", rooms: [] }],
      },
    ],
  },
};

const hardError = {
  code: 500,
  msg: "网络中断",
  data: null,
};

const serviceError = {
  code: 503,
  msg: "教务系统暂时不可用",
  data: null,
};

describe("hasUsableClassroomData", () => {
  it("accepts code 0 payloads with campuses", () => {
    expect(hasUsableClassroomData(goodPrev, now)).toBe(true);
  });

  it("rejects loading, hard errors, and missing campuses", () => {
    expect(
      hasUsableClassroomData({ code: 1, msg: "加载中", data: null }, now)
    ).toBe(false);
    expect(hasUsableClassroomData(hardError, now)).toBe(false);
    expect(
      hasUsableClassroomData(
        { code: 0, msg: "", data: { campuses: null } },
        now
      )
    ).toBe(false);
  });

  it("rejects cross-day and expired classroom snapshots", () => {
    expect(
      hasUsableClassroomData(
        { ...goodPrev, data: { ...goodPrev.data, date: "2026-07-09" } },
        now
      )
    ).toBe(false);
    expect(
      hasUsableClassroomData(
        {
          ...goodPrev,
          data: {
            ...goodPrev.data,
            stale_until: "2026-07-10T12:00:00+08:00",
          },
        },
        now
      )
    ).toBe(false);
  });
});

describe("shouldFullPageSpin", () => {
  it("never full-page spins for background reload", () => {
    expect(shouldFullPageSpin(true, false)).toBe(false);
    expect(shouldFullPageSpin(true, true)).toBe(false);
  });

  it("full-page spins only for initial/manual when there is no usable data", () => {
    expect(shouldFullPageSpin(false, false)).toBe(true);
    expect(shouldFullPageSpin(false, true)).toBe(false);
  });
});

describe("pollingInterval", () => {
  // Pin Date.now() to the fixtures' business day so usability checks pass.
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(now);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("never returns a falsy interval (SWR chain-death guard)", () => {
    // SWR permanently terminates the polling chain on 0/null intervals and
    // nextReloadDelay(undefined) is null (pre-first-data), hence the floor.
    expect(pollingInterval(undefined)).toBe(MIN_FRESH_DELAY_MS);
    expect(pollingInterval(null)).toBe(MIN_FRESH_DELAY_MS);
    expect(pollingInterval({ code: 500, msg: "boom", data: null })).toBe(
      MIN_FRESH_DELAY_MS
    );
  });

  it("derives the poll delay from the cached envelope's snapshot", () => {
    vi.spyOn(Math, "random").mockReturnValue(0);
    expect(
      pollingInterval({
        ...goodPrev,
        data: { ...goodPrev.data, stale: true },
      })
    ).toBe(STALE_POLL_MS);
    expect(
      pollingInterval({
        ...goodPrev,
        data: { ...goodPrev.data, partial_campuses: ["04"] },
      })
    ).toBe(PARTIAL_POLL_MS);
    // Cross-day/expired cache revalidates promptly at the 1s floor.
    expect(
      pollingInterval({
        ...goodPrev,
        data: { ...goodPrev.data, stale_until: "2026-07-10T11:00:00+08:00" },
      })
    ).toBe(MIN_FRESH_DELAY_MS);
  });
});

describe("retryDelayFor", () => {
  const opts = { nowMs: now, random: () => 0 };

  it("maps SWR's 1-based retryCount straight onto the failure ladder", () => {
    expect(retryDelayFor(1, null, opts)).toBe(10_000);
    expect(retryDelayFor(2, null, opts)).toBe(20_000);
    expect(retryDelayFor(3, null, opts)).toBe(30_000);
    expect(retryDelayFor(4, null, opts)).toBe(60_000);
    expect(retryDelayFor(9, null, opts)).toBe(60_000);
  });

  it("clamps ladder delays to a displayable snapshot's stale_until deadline", () => {
    const nearDeadline = {
      ...goodPrev.data,
      stale_until: new Date(now + 3_000).toISOString(),
    };
    // Even with maximum jitter, the retry wakes at the hard deadline.
    expect(
      retryDelayFor(1, nearDeadline, { nowMs: now, random: () => 1 })
    ).toBe(3_000);
    expect(
      retryDelayFor(4, nearDeadline, { nowMs: now, random: () => 1 })
    ).toBe(3_000);
  });
});

describe("mergeFetchResult", () => {
  it("keeps last good campuses on background/hard fetch failure", () => {
    const merged = mergeFetchResult(goodPrev, hardError, now);

    expect(merged.code).toBe(0);
    expect(merged.data!.campuses).toEqual(goodPrev.data.campuses);
    expect(merged.data!.date).toBe(goodPrev.data.date);
    expect(merged.data!.stale).toBe(true);
    expect(merged.data!.error).toEqual({
      type: "client_refresh_failed",
      message: "网络中断",
    });
  });

  it("keeps last good data when the service returns a non-ok envelope", () => {
    const merged = mergeFetchResult(goodPrev, serviceError, now);

    expect(merged.code).toBe(0);
    expect(merged.data!.campuses).toHaveLength(1);
    expect((merged.data!.error as { message: string }).message).toBe(
      "教务系统暂时不可用"
    );
  });

  it("uses a hard empty error envelope when there was no prior good snapshot", () => {
    expect(
      mergeFetchResult({ code: 1, msg: "加载中", data: null }, hardError)
    ).toEqual(hardError);

    expect(mergeFetchResult(null, hardError)).toEqual(hardError);

    expect(
      mergeFetchResult(
        { code: 500, msg: "old", data: null },
        { code: 503, msg: "  ", data: null }
      )
    ).toEqual({
      code: 503,
      msg: fallbackErrorMessage,
      data: null,
    });
  });

  it("clears previous-day or expired snapshots after a fetch failure", () => {
    const previousDay = {
      ...goodPrev,
      data: { ...goodPrev.data, date: "2026-07-09" },
    };
    const expired = {
      ...goodPrev,
      data: {
        ...goodPrev.data,
        stale_until: "2026-07-10T12:00:00+08:00",
      },
    };

    expect(mergeFetchResult(previousDay, hardError, now)).toEqual(hardError);
    expect(mergeFetchResult(expired, hardError, now)).toEqual(hardError);
  });

  it("fails closed when a successful envelope has invalid cache metadata", () => {
    expect(
      mergeFetchResult(
        null,
        {
          code: 0,
          msg: "",
          data: { date: "2026-07-10", campuses: [] },
        },
        now
      )
    ).toEqual({
      code: 500,
      msg: fallbackErrorMessage,
      data: null,
    });
  });

  it("replaces prior state with a successful payload", () => {
    const next: {
      code: number;
      msg: string;
      data: Record<string, unknown>;
    } = {
      code: 0,
      msg: "fresh",
      data: {
        date: "2026-07-10",
        campuses: [{ id: "01", name: "西土城", buildings: [] }],
      },
    };

    next.data.stale_until = "2026-07-10T23:59:59.999+08:00";

    expect(mergeFetchResult(goodPrev, next, now)).toEqual(next);
    expect(
      mergeFetchResult(
        {
          ...goodPrev,
          data: {
            ...goodPrev.data,
            stale: true,
            error: { type: "client_refresh_failed", message: "old" },
          },
        },
        next,
        now
      )
    ).toEqual(next);
  });
});
