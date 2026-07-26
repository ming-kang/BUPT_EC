import { describe, expect, it } from "vitest";
import { ApiError } from "./apiError";

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
