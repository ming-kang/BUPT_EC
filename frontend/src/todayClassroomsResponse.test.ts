import { describe, expect, it } from "vitest";
import {
  classroomWarningMessage,
  fallbackErrorMessage,
  normalizeResponse,
} from "./todayClassroomsResponse";

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
      ok: true,
      resp: {
        code: 0,
        msg: "ok",
        data: {
          date: "2026-07-06",
          campuses: [{ id: "01", name: "西土城" }],
        },
      },
    });
  });

  it("normalizes non-zero service envelopes to safe null-data errors", () => {
    expect(normalizeResponse({ code: 503, msg: " 教务系统暂时不可用 " })).toEqual({
      ok: true,
      resp: {
        code: 503,
        msg: "教务系统暂时不可用",
        data: null,
      },
    });
    expect(normalizeResponse({ code: 500, msg: "   " })).toEqual({
      ok: true,
      resp: {
        code: 500,
        msg: fallbackErrorMessage,
        data: null,
      },
    });
  });

  it("rejects malformed success envelopes before UI code reads them", () => {
    expect(normalizeResponse(null)).toEqual({
      ok: false,
      reason: "服务返回格式异常",
    });
    expect(normalizeResponse({ code: "not-a-number" })).toEqual({
      ok: false,
      reason: "服务返回状态异常",
    });
    expect(normalizeResponse({ code: 0, data: null })).toEqual({
      ok: false,
      reason: "服务返回数据格式异常",
    });
    expect(normalizeResponse({ code: 0, data: { campuses: {} } })).toEqual({
      ok: false,
      reason: "服务返回校区数据异常",
    });
  });
});

describe("normalizeResponse envelope version", () => {
  it("passes the build version through on both success and failure envelopes", () => {
    expect(
      normalizeResponse({
        code: 0,
        msg: "ok",
        version: "v0.3.0",
        data: { date: "2026-07-06", campuses: [] },
      })
    ).toEqual({
      ok: true,
      resp: {
        code: 0,
        msg: "ok",
        version: "v0.3.0",
        data: { date: "2026-07-06", campuses: [] },
      },
    });

    expect(
      normalizeResponse({ code: 503, msg: "暂无数据", version: "v0.3.0" })
    ).toEqual({
      ok: true,
      resp: {
        code: 503,
        msg: "暂无数据",
        version: "v0.3.0",
        data: null,
      },
    });
  });

  it("omits the key entirely rather than emitting an empty version", () => {
    // A pre-0.3.0 backend sends no version; the envelope shape must be
    // unchanged so the settings row is simply absent.
    const result = normalizeResponse({
      code: 0,
      msg: "ok",
      data: { campuses: [] },
    });
    expect(result.ok && "version" in result.resp).toBe(false);
  });

  it("drops a non-string wire version instead of rendering it", () => {
    const result = normalizeResponse({
      code: 0,
      msg: "ok",
      version: 123,
      data: { campuses: [] },
    });
    expect(result.ok && "version" in result.resp).toBe(false);
  });
});

describe("classroomWarningMessage", () => {
  it("names affected campuses and falls back to IDs", () => {
    expect(
      classroomWarningMessage({
        campuses: [{ id: "04", name: "沙河" }],
        partial_campuses: ["04", "01"],
        error: { message: "部分数据刷新失败" },
      })
    ).toBe("受影响校区：沙河（04）、01。部分数据刷新失败");
  });

  it("keeps the generic warning when partial_campuses is absent", () => {
    expect(
      classroomWarningMessage({ error: { message: "刷新失败" } })
    ).toBe("刷新失败");
    expect(classroomWarningMessage({ stale: true })).toBe(
      "当前展示的是今天最后一次成功刷新数据"
    );
  });
});
