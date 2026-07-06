import { describe, expect, it } from "vitest";
import { fallbackErrorMessage, normalizeResponse } from "./todayClassroomsResponse";

describe("normalizeResponse", () => {
  it("preserves a successful classroom payload with campuses", () => {
    const payload = {
      code: 0,
      msg: " ok ",
      data: {
        date: "2026-07-06",
        campuses: [{ id: "01", name: "西土城" }],
      },
    };

    expect(normalizeResponse(payload)).toEqual({
      code: 0,
      msg: "ok",
      data: {
        date: "2026-07-06",
        campuses: [{ id: "01", name: "西土城" }],
      },
    });
  });

  it("normalizes non-zero service envelopes to safe null-data errors", () => {
    expect(normalizeResponse({ code: 503, msg: " 教务系统暂时不可用 " })).toEqual({
      code: 503,
      msg: "教务系统暂时不可用",
      data: null,
    });
    expect(normalizeResponse({ code: 500, msg: "   " })).toEqual({
      code: 500,
      msg: fallbackErrorMessage,
      data: null,
    });
  });

  it("rejects malformed success envelopes before UI code reads them", () => {
    expect(() => normalizeResponse(null)).toThrow("服务返回格式异常");
    expect(() => normalizeResponse({ code: "not-a-number" })).toThrow(
      "服务返回状态异常"
    );
    expect(() => normalizeResponse({ code: 0, data: null })).toThrow(
      "服务返回数据格式异常"
    );
    expect(() => normalizeResponse({ code: 0, data: { campuses: {} } })).toThrow(
      "服务返回校区数据异常"
    );
  });
});
