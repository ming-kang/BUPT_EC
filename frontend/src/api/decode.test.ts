import { describe, expect, it } from "vitest";
import {
  ApiError,
  classroomWarningMessage,
  decodeGetData,
  fallbackErrorMessage,
  isUsableBusinessDaySnapshot,
} from "./decode";
import type { TodayClassroomsData } from "./types";

describe("ApiError", () => {
  it("keeps the real HTTP status, business code and logId structured", () => {
    const err = new ApiError("教务系统暂时不可用", {
      status: 503,
      code: 503,
      logId: "log-abc",
    });

    expect(err).toBeInstanceOf(Error);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.name).toBe("ApiError");
    expect(err.message).toBe("教务系统暂时不可用");
    expect(err.status).toBe(503);
    expect(err.code).toBe(503);
    expect(err.logId).toBe("log-abc");
  });

  it("defaults logId to an empty string and tolerates missing details", () => {
    expect(new ApiError("boom", { status: 404 }).logId).toBe("");
    expect(new ApiError("boom").status).toBeUndefined();
    expect(new ApiError("boom").code).toBeUndefined();
    expect(new ApiError("boom").logId).toBe("");
  });
});

/* --------------------------- fixtures --------------------------- */

function snapshot(overrides: Record<string, unknown> = {}) {
  return {
    date: "2026-07-10",
    updated_at: "2026-07-10T08:00:00+08:00",
    expires_at: "2026-07-10T12:00:00+08:00",
    stale_until: "2026-07-10T23:59:59.999+08:00",
    stale: false,
    campuses: [
      {
        id: "01",
        name: "西土城",
        buildings: [
          {
            name: "主楼",
            display_name: "主楼",
            rooms: [
              {
                name: "主楼-101",
                display_name: "101",
                capacity: 60,
                free_nodes: [1, 2],
                free_times: [{ node: 1, time: "08:00-08:45" }],
              },
            ],
          },
        ],
        nodes: [{ node: 1, time: "08:00-08:45", room_count: 1 }],
      },
    ],
    error: null,
    ...overrides,
  };
}

const now = Date.parse("2026-07-10T12:00:00+08:00");

/* --------------------------- decodeGetData --------------------------- */

describe("decodeGetData", () => {
  it("decodes a fully valid success envelope", () => {
    const payload = {
      code: 0,
      msg: " ok ",
      log_id: "log-1",
      version: "v0.3.1",
      data: snapshot(),
    };

    expect(decodeGetData(payload)).toEqual({
      code: 0,
      msg: "ok",
      logId: "log-1",
      version: "v0.3.1",
      data: snapshot(),
    });
  });

  it("normalizes non-zero service envelopes to safe null-data errors", () => {
    expect(decodeGetData({ code: 503, msg: " 教务系统暂时不可用 " })).toEqual({
      code: 503,
      msg: "教务系统暂时不可用",
      data: null,
    });
    expect(decodeGetData({ code: 500, msg: "   " })).toEqual({
      code: 500,
      msg: fallbackErrorMessage,
      data: null,
    });
  });

  it("rejects malformed envelopes before UI code reads them", () => {
    expect(() => decodeGetData(null)).toThrowError("服务返回格式异常");
    expect(() => decodeGetData({ code: "not-a-number" })).toThrowError(
      "服务返回状态异常"
    );
    expect(() => decodeGetData({ code: 0, data: null })).toThrowError(
      "服务返回数据格式异常"
    );
    expect(() =>
      decodeGetData({ code: 0, data: { campuses: {} } })
    ).toThrowError("服务返回校区数据异常");
  });

  it("rejects shallow-valid containers with broken nested items", () => {
    const brokenCases: unknown[] = [
      { ...snapshot(), campuses: [null] },
      { ...snapshot(), campuses: [{ id: "01" }] }, // missing name/buildings/nodes
      { ...snapshot(), campuses: [{ id: "01", name: "x", buildings: [{}], nodes: [] }] },
      { ...snapshot(), campuses: [{ id: "01", name: "x", buildings: [], nodes: [{}] }] },
      {
        ...snapshot(),
        campuses: [
          {
            id: "01",
            name: "x",
            buildings: [
              {
                name: "b",
                rooms: [
                  {
                    name: "r",
                    capacity: "60", // capacity must be a number
                    free_nodes: [1],
                    free_times: [],
                  },
                ],
              },
            ],
            nodes: [],
          },
        ],
      },
      {
        ...snapshot(),
        campuses: [
          {
            id: "01",
            name: "x",
            buildings: [
              {
                name: "b",
                rooms: [
                  {
                    name: "r",
                    capacity: 60,
                    free_nodes: ["1"], // free_nodes must be numbers
                    free_times: [],
                  },
                ],
              },
            ],
            nodes: [],
          },
        ],
      },
      {
        ...snapshot(),
        campuses: [
          {
            id: "01",
            name: "x",
            buildings: [
              { name: "b", rooms: [{ name: "r", capacity: 60, free_nodes: [], free_times: [{ node: "1", time: "t" }] }] },
            ],
            nodes: [],
          },
        ],
      },
    ];

    for (const data of brokenCases) {
      expect(
        () => decodeGetData({ code: 0, msg: "", data }),
        JSON.stringify(data)
      ).toThrowError(ApiError);
    }
  });

  it("rejects unusable or malformed snapshot metadata", () => {
    const metadataCases: unknown[] = [
      { ...snapshot(), date: 20260710 },
      { ...snapshot(), date: "" },
      { ...snapshot(), updated_at: null },
      { ...snapshot(), expires_at: null },
      { ...snapshot(), stale_until: "not-a-time" },
      { ...snapshot(), stale: "false" },
      { ...snapshot(), error: { type: "jw_query_failed" } }, // missing message
      { ...snapshot(), error: "boom" },
      { ...snapshot(), partial_campuses: [null] },
      { ...snapshot(), partial_campuses: "04" },
    ];

    for (const data of metadataCases) {
      expect(
        () => decodeGetData({ code: 0, msg: "", data }),
        JSON.stringify(data)
      ).toThrowError(ApiError);
    }
  });

  it("accepts degraded-but-valid shapes the backend can emit", () => {
    // Capacity 0 (unknown), empty rooms/free lists, absent optional fields.
    const data = {
      ...snapshot(),
      campuses: [
        {
          id: "04",
          name: "沙河",
          buildings: [
            { name: "S1", rooms: [] },
            // Pre-0.3.x cache without display_name falls back to name.
            {
              name: "S2",
              rooms: [
                {
                  name: "S2-101",
                  capacity: 0,
                  free_nodes: [],
                  free_times: [],
                },
              ],
            },
          ],
          nodes: [],
        },
      ],
      error: null,
    };

    expect(decodeGetData({ code: 0, msg: "", data })).toEqual({
      code: 0,
      msg: "",
      data: {
        ...data,
        campuses: [
          {
            id: "04",
            name: "沙河",
            buildings: [
              { name: "S1", display_name: "S1", rooms: [] },
              {
                name: "S2",
                display_name: "S2",
                rooms: [
                  {
                    name: "S2-101",
                    display_name: "S2-101",
                    capacity: 0,
                    free_nodes: [],
                    free_times: [],
                  },
                ],
              },
            ],
            nodes: [],
          },
        ],
      },
    });
  });

  it("normalizes partial_campuses ids to trimmed strings", () => {
    const data = decodeGetData({
      code: 0,
      msg: "",
      data: {
        ...snapshot(),
        partial_campuses: ["04", 1, " ", "01"],
      },
    });

    expect(data.data?.partial_campuses).toEqual(["04", "1", "01"]);
  });

  it("passes the build version through on both success and failure envelopes", () => {
    expect(
      decodeGetData({
        code: 0,
        msg: "ok",
        version: "v0.3.0",
        data: snapshot({ campuses: [] }),
      })
    ).toEqual({
      code: 0,
      msg: "ok",
      version: "v0.3.0",
      data: snapshot({ campuses: [] }),
    });

    expect(
      decodeGetData({ code: 503, msg: "暂无数据", version: "v0.3.0" })
    ).toEqual({
      code: 503,
      msg: "暂无数据",
      version: "v0.3.0",
      data: null,
    });
  });

  it("omits version and logId entirely rather than emitting empty values", () => {
    // Pre-0.3.0 backends send neither; the envelope shape must be unchanged
    // so the settings row and error card simply stay hidden.
    const result = decodeGetData({
      code: 0,
      msg: "ok",
      data: snapshot({ campuses: [] }),
    });
    expect("version" in result).toBe(false);
    expect("logId" in result).toBe(false);
  });

  it("drops non-string wire version/log_id instead of rendering them", () => {
    const result = decodeGetData({
      code: 0,
      msg: "ok",
      version: 123,
      log_id: {},
      data: snapshot({ campuses: [] }),
    });
    expect("version" in result).toBe(false);
    expect("logId" in result).toBe(false);
  });
});

/* -------------------- isUsableBusinessDaySnapshot -------------------- */

describe("isUsableBusinessDaySnapshot", () => {
  it("accepts a same-day snapshot inside its stale window", () => {
    expect(
      isUsableBusinessDaySnapshot(snapshot({ campuses: [] }), now)
    ).toBe(true);
  });

  it("rejects previous-day and expired snapshots", () => {
    expect(
      isUsableBusinessDaySnapshot(
        snapshot({ date: "2026-07-09", campuses: [] }),
        now
      )
    ).toBe(false);
    expect(
      isUsableBusinessDaySnapshot(
        snapshot({ stale_until: "2026-07-10T12:00:00+08:00", campuses: [] }),
        now
      )
    ).toBe(false);
  });

  it("fails closed without a snapshot", () => {
    expect(isUsableBusinessDaySnapshot(null, now)).toBe(false);
    expect(isUsableBusinessDaySnapshot(undefined, now)).toBe(false);
  });
});

/* ---------------------- classroomWarningMessage ---------------------- */

describe("classroomWarningMessage", () => {
  it("names affected campuses and falls back to ids", () => {
    const data = {
      ...snapshot({ campuses: [], partial_campuses: undefined }),
      campuses: [
        {
          id: "04",
          name: "沙河",
          buildings: [],
          nodes: [],
        },
      ],
      partial_campuses: ["04", "01"],
      error: { type: "jw_query_failed", message: "部分数据刷新失败" },
    } as TodayClassroomsData;

    expect(classroomWarningMessage(data)).toBe(
      "受影响校区：沙河（04）、01。部分数据刷新失败"
    );
  });

  it("keeps the generic warning when partial_campuses is absent", () => {
    const withError = {
      ...snapshot({ campuses: [] }),
      error: { type: "jw_query_failed", message: "刷新失败" },
    } as TodayClassroomsData;
    expect(classroomWarningMessage(withError)).toBe("刷新失败");
    expect(
      classroomWarningMessage(snapshot({ campuses: [], stale: true }))
    ).toBe("当前展示的是今天最后一次成功刷新数据");
  });
});
