/**
 * @vitest-environment jsdom
 *
 * GlobalEmpty rendering: hidden on success, loading text, and error state
 * with the optional log_id line for support diagnostics.
 */
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import GlobalEmpty from "./GlobalEmpty";

describe("GlobalEmpty", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders nothing for a successful envelope", () => {
    const { container } = render(
      <GlobalEmpty
        todayData={{ code: 0, msg: "", data: { campuses: [] } }}
        isError={false}
      />
    );
    expect(container.firstChild).toBeNull();
  });

  it("shows the loading description without a retry button", () => {
    render(
      <GlobalEmpty
        todayData={{ code: 1, msg: "加载中", data: null }}
        isError={false}
      />
    );
    expect(screen.getByText("加载中")).toBeTruthy();
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.queryByText(/log_id/)).toBeNull();
  });

  it("shows the error message with the log_id line when present", () => {
    render(
      <GlobalEmpty
        todayData={{
          code: 503,
          msg: "教务系统暂时不可用",
          data: null,
          logId: "test-log-id",
        }}
        isError
        onRetry={() => {}}
      />
    );
    expect(screen.getByText("教务系统暂时不可用")).toBeTruthy();
    const logLine = screen.getByText(/log_id: test-log-id/);
    expect(logLine.className).toBe("global-empty__log-id");
    // antd inserts a space between two-CJK-character button labels.
    expect(screen.getByRole("button", { name: /重\s*试/ })).toBeTruthy();
  });

  it("omits the log_id line when logId is empty or missing", () => {
    render(
      <GlobalEmpty
        todayData={{ code: 500, msg: "数据获取失败", data: null, logId: "" }}
        isError
        onRetry={() => {}}
      />
    );
    expect(screen.getByText("数据获取失败")).toBeTruthy();
    expect(screen.queryByText(/log_id/)).toBeNull();
  });
});
