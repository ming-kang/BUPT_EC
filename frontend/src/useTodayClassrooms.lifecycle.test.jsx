/**
 * @vitest-environment jsdom
 *
 * Real mount/unmount harness for useTodayClassrooms. Pure helper cases stay in
 * useTodayClassrooms.test.js so later timeout/visibility work can extend this file.
 */
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import useTodayClassrooms from "./useTodayClassrooms";

function shanghaiToday() {
  return new Intl.DateTimeFormat("en-CA", {
    timeZone: "Asia/Shanghai",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date());
}

function usablePayload(overrides = {}) {
  const date = shanghaiToday();
  const { data: dataOverrides = {}, ...topOverrides } = overrides;
  return {
    code: 0,
    msg: "ok",
    ...topOverrides,
    data: {
      date,
      expires_at: `${date}T23:50:00+08:00`,
      stale_until: `${date}T23:59:59.999+08:00`,
      campuses: [
        {
          id: "04",
          name: "沙河",
          buildings: [{ name: "S1", rooms: [] }],
        },
      ],
      ...dataOverrides,
    },
  };
}

function HookProbe() {
  const { resp, spinning, reloading, isError, retry } = useTodayClassrooms();
  return (
    <div>
      <div data-testid="code">{String(resp.code)}</div>
      <div data-testid="msg">{resp.msg || ""}</div>
      <div data-testid="spinning">{String(spinning)}</div>
      <div data-testid="reloading">{String(reloading)}</div>
      <div data-testid="is-error">{String(isError)}</div>
      <div data-testid="campus-count">
        {Array.isArray(resp.data?.campuses) ? resp.data.campuses.length : 0}
      </div>
      <div data-testid="stale">{String(Boolean(resp.data?.stale))}</div>
      <button type="button" onClick={retry}>
        retry
      </button>
    </div>
  );
}

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe("useTodayClassrooms lifecycle", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        Promise.resolve({
          ok: true,
          status: 200,
          json: async () => usablePayload(),
        })
      )
    );
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("loads initial data on mount", async () => {
    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });
    expect(fetch).toHaveBeenCalledWith(
      "/api/get_data",
      expect.objectContaining({
        headers: { Accept: "application/json" },
      })
    );
    expect(screen.getByTestId("campus-count").textContent).toBe("1");
    expect(screen.getByTestId("spinning").textContent).toBe("false");
  });

  it("aborts in-flight fetch on unmount", async () => {
    const pending = deferred();
    let seenSignal;
    fetch.mockImplementation((_url, init) => {
      seenSignal = init.signal;
      return pending.promise;
    });

    const view = render(<HookProbe />);
    await waitFor(() => {
      expect(fetch).toHaveBeenCalledTimes(1);
    });
    expect(seenSignal).toBeInstanceOf(AbortSignal);
    expect(seenSignal.aborted).toBe(false);

    view.unmount();
    expect(seenSignal.aborted).toBe(true);

    // Late resolve must not crash or update unmounted state.
    await act(async () => {
      pending.resolve({
        ok: true,
        status: 200,
        json: async () => usablePayload(),
      });
      await Promise.resolve();
    });
  });

  it("manual retry issues a second request and clears full-page error", async () => {
    fetch
      .mockImplementationOnce(async () => ({
        ok: false,
        status: 503,
        json: async () => ({ code: 503, msg: "upstream down", data: null }),
      }))
      .mockImplementationOnce(async () => ({
        ok: true,
        status: 200,
        json: async () => usablePayload(),
      }));

    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("is-error").textContent).toBe("true");
    });
    expect(screen.getByTestId("msg").textContent).toContain("upstream down");

    await act(async () => {
      screen.getByRole("button", { name: "retry" }).click();
    });
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });
    expect(fetch).toHaveBeenCalledTimes(2);
    expect(screen.getByTestId("is-error").textContent).toBe("false");
  });

  it("keeps last good data when a later background reload fails", async () => {
    const soon = new Date(Date.now() + 1_500).toISOString();
    fetch
      .mockImplementationOnce(async () => ({
        ok: true,
        status: 200,
        json: async () =>
          usablePayload({
            data: {
              expires_at: soon,
            },
          }),
      }))
      .mockImplementationOnce(async () => {
        throw new Error("network down");
      });

    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });

    // Advance past the short fresh-cache delay from expires_at.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3_000);
    });

    await waitFor(() => {
      expect(fetch).toHaveBeenCalledTimes(2);
    });
    await waitFor(() => {
      expect(screen.getByTestId("stale").textContent).toBe("true");
    });
    expect(screen.getByTestId("code").textContent).toBe("0");
    expect(screen.getByTestId("campus-count").textContent).toBe("1");
    expect(screen.getByTestId("msg").textContent).not.toBe("");
  });

  it("clears the reload timer on unmount", async () => {
    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });
    const callsAfterLoad = fetch.mock.calls.length;
    cleanup();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    expect(fetch.mock.calls.length).toBe(callsAfterLoad);
  });
});
