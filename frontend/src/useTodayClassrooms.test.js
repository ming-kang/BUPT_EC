import { describe, expect, it } from "vitest";
import {
  hasUsableClassroomData,
  mergeFetchResult,
  shouldFullPageSpin,
} from "./useTodayClassrooms";
import { fallbackErrorMessage } from "./todayClassroomsResponse";

const goodPrev = {
  code: 0,
  msg: "ok",
  data: {
    date: "2026-07-10",
    expires_at: "2026-07-10T12:05:00+08:00",
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
    expect(hasUsableClassroomData(goodPrev)).toBe(true);
  });

  it("rejects loading, hard errors, and missing campuses", () => {
    expect(hasUsableClassroomData({ code: 1, msg: "加载中", data: null })).toBe(
      false
    );
    expect(hasUsableClassroomData(hardError)).toBe(false);
    expect(
      hasUsableClassroomData({ code: 0, msg: "", data: { campuses: null } })
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

describe("mergeFetchResult", () => {
  it("keeps last good campuses on background/hard fetch failure", () => {
    const merged = mergeFetchResult(goodPrev, hardError);

    expect(merged.code).toBe(0);
    expect(merged.data.campuses).toEqual(goodPrev.data.campuses);
    expect(merged.data.date).toBe(goodPrev.data.date);
    expect(merged.data.stale).toBe(true);
    expect(merged.data.error).toEqual({
      type: "client_refresh_failed",
      message: "网络中断",
    });
  });

  it("keeps last good data when the service returns a non-ok envelope", () => {
    const merged = mergeFetchResult(goodPrev, serviceError);

    expect(merged.code).toBe(0);
    expect(merged.data.campuses).toHaveLength(1);
    expect(merged.data.error.message).toBe("教务系统暂时不可用");
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

  it("replaces prior state with a successful payload", () => {
    const next = {
      code: 0,
      msg: "fresh",
      data: {
        date: "2026-07-10",
        campuses: [{ id: "01", name: "西土城", buildings: [] }],
      },
    };

    expect(mergeFetchResult(goodPrev, next)).toEqual(next);
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
        next
      )
    ).toEqual(next);
  });
});
