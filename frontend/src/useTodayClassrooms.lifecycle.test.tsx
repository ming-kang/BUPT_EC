/**
 * @vitest-environment jsdom
 *
 * Real mount/unmount harness for useTodayClassrooms: scheduling (fresh /
 * stale / failure ladder / stale_until clamp), visibility and focus
 * revalidation, snapshot retention, timeout + transport mapping, and the
 * abort-on-unmount contract. Pure schedule cases stay in
 * reloadSchedule.test.ts; decode cases stay in api/decode.test.ts.
 */
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import useTodayClassrooms from "./useTodayClassrooms";

const TIMEOUT_MESSAGE = "请求超时，请稍后重试";
const TRANSPORT_MESSAGE = "网络连接异常，请检查网络后重试";
const EXPIRED_MESSAGE = "当前缓存已失效，正在重新获取";

function shanghaiToday() {
  return new Intl.DateTimeFormat("en-CA", {
    timeZone: "Asia/Shanghai",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date());
}

/** Deep-valid fixture (api/decode rejects anything less). */
function usablePayload(
  overrides: { data?: Record<string, unknown>; [key: string]: unknown } = {}
) {
  const date = shanghaiToday();
  const { data: dataOverrides = {}, ...topOverrides } = overrides;
  return {
    code: 0,
    msg: "ok",
    ...topOverrides,
    data: {
      date,
      updated_at: `${date}T08:00:00+08:00`,
      expires_at: `${date}T23:50:00+08:00`,
      stale_until: `${date}T23:59:59.999+08:00`,
      stale: false,
      campuses: [
        {
          id: "04",
          name: "沙河",
          buildings: [{ name: "S1", display_name: "S1", rooms: [] }],
          nodes: [],
        },
      ],
      error: null,
      ...dataOverrides,
    },
  };
}

function jsonResponse(payload: unknown, init: { ok?: boolean; status?: number; logId?: string } = {}) {
  return {
    ok: init.ok ?? true,
    status: init.status ?? 200,
    headers: { get: (name: string) => (name === "X-Log-Id" ? init.logId ?? null : null) },
    json: async () => payload,
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
      <div data-testid="campus-count">{resp.data?.campuses.length ?? 0}</div>
      <div data-testid="stale">{String(Boolean(resp.data?.stale))}</div>
      <div data-testid="error-message">{resp.data?.error?.message ?? ""}</div>
      <div data-testid="log-id">{resp.logId || ""}</div>
      <div data-testid="version">{resp.version || ""}</div>
      <button type="button" onClick={retry}>
        retry
      </button>
    </div>
  );
}

function deferred() {
  let resolve: (value?: unknown) => void = () => {};
  let reject: (reason?: unknown) => void = () => {};
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe("useTodayClassrooms lifecycle", () => {
  let fetchMock: Mock;

  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    // Deterministic schedules: positive jitter samples of 0 keep base delays.
    vi.spyOn(Math, "random").mockReturnValue(0);
    fetchMock = vi.fn(() => Promise.resolve(jsonResponse(usablePayload())));
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    // Drop any per-case visibilityState override (falls back to the
    // prototype's "visible") so hidden-tab cases cannot leak forward.
    Reflect.deleteProperty(document, "visibilityState");
  });

  it("loads initial data on mount after a full-page spin", async () => {
    const pending = deferred();
    fetchMock.mockImplementation(() => pending.promise);

    render(<HookProbe />);
    // Initial load with no usable data: full-page spinner + loading envelope.
    expect(screen.getByTestId("spinning").textContent).toBe("true");
    expect(screen.getByTestId("code").textContent).toBe("1");
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/get_data",
      expect.objectContaining({
        headers: { Accept: "application/json" },
      })
    );

    await act(async () => {
      pending.resolve(jsonResponse(usablePayload()));
    });
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });
    expect(screen.getByTestId("campus-count").textContent).toBe("1");
    expect(screen.getByTestId("spinning").textContent).toBe("false");
  });

  it("auto-reloads near expires_at with a 1s floor", async () => {
    const soon = Date.now() + 1_500;
    fetchMock.mockImplementation(async () =>
      jsonResponse(
        usablePayload({
          data: { expires_at: new Date(soon).toISOString() },
        })
      )
    );

    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });
    const callsAfterLoad = fetchMock.mock.calls.length;

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1_600);
    });
    await waitFor(() => {
      expect(fetchMock.mock.calls.length).toBe(callsAfterLoad + 1);
    });
  });

  it("polls stale snapshots at the 15s cadence", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(usablePayload({ data: { stale: true } }))
    );

    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });
    const callsAfterLoad = fetchMock.mock.calls.length;

    // 14s: quiet. 15s: the stale poll fires.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(14_000);
    });
    expect(fetchMock.mock.calls.length).toBe(callsAfterLoad);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1_500);
    });
    expect(fetchMock.mock.calls.length).toBe(callsAfterLoad + 1);
  });

  it("backs off client failures 10s/20s/30s/60s and resets the ladder after a success", async () => {
    const shortLived = () =>
      jsonResponse(
        usablePayload({
          data: { expires_at: new Date(Date.now() + 3_000).toISOString() },
        })
      );
    fetchMock
      .mockImplementationOnce(async () => shortLived())
      .mockImplementationOnce(async () => {
        throw new TypeError("Failed to fetch");
      })
      .mockImplementationOnce(async () => {
        throw new TypeError("Failed to fetch");
      })
      .mockImplementationOnce(async () => shortLived())
      .mockImplementationOnce(async () => {
        throw new TypeError("Failed to fetch");
      });

    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });

    // ~3s: poll hits failure #1 → ladder rung 1 (10s). Snapshot stays visible.
    // (3.5s crosses the 3s poll delay; failure #1 lands ≈0.5s into it.)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3_500);
    });
    await waitFor(() => {
      expect(screen.getByTestId("stale").textContent).toBe("true");
    });
    expect(screen.getByTestId("error-message").textContent).toBe(
      TRANSPORT_MESSAGE
    );
    expect(screen.getByTestId("campus-count").textContent).toBe("1");
    let calls = fetchMock.mock.calls.length;
    expect(calls).toBe(2);

    // 8.5s into the 10s backoff: quiet.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(8_000);
    });
    expect(fetchMock.mock.calls.length).toBe(calls);

    // Rung 1 fires and fails again → rung 2 (20s).
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_500);
    });
    expect(fetchMock.mock.calls.length).toBe(calls + 1);

    // 18.5s into the 20s backoff: quiet.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(18_000);
    });
    expect(fetchMock.mock.calls.length).toBe(calls + 1);

    // Rung 2 fires and succeeds → the ladder resets.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1_500);
    });
    await waitFor(() => {
      expect(screen.getByTestId("stale").textContent).toBe("false");
    });
    expect(fetchMock.mock.calls.length).toBe(calls + 2);

    // ~3s later the next poll fails again: a brand-new ladder starts at rung
    // 1 (10s), never at the old rung 3 (30s).
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3_500);
    });
    await waitFor(() => {
      expect(screen.getByTestId("stale").textContent).toBe("true");
    });
    calls = fetchMock.mock.calls.length;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(11_000);
    });
    expect(fetchMock.mock.calls.length).toBe(calls + 1);
  });

  it("never overlaps requests and skips polls while one is in flight", async () => {
    const pending = deferred();
    let seenSignal: AbortSignal | undefined;
    fetchMock.mockImplementation((_url: string, init?: { signal?: AbortSignal }) => {
      seenSignal = init?.signal;
      return pending.promise;
    });

    render(<HookProbe />);
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });

    // No timer is armed while a request is in flight: long advances stay quiet.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(120_000);
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);

    // A manual retry during the in-flight request is absorbed (never overlap).
    await act(async () => {
      screen.getByRole("button", { name: "retry" }).click();
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      pending.resolve(jsonResponse(usablePayload()));
    });
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });
    expect(seenSignal).toBeInstanceOf(AbortSignal);
  });

  it("aborts the in-flight request on unmount and ignores the late result", async () => {
    const pending = deferred();
    let seenSignal: AbortSignal | undefined;
    fetchMock.mockImplementation((_url: string, init?: { signal?: AbortSignal }) => {
      seenSignal = init?.signal;
      return pending.promise;
    });

    const view = render(<HookProbe />);
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });
    expect(seenSignal!.aborted).toBe(false);

    view.unmount();
    // New contract: the in-flight request dies with the component.
    expect(seenSignal!.aborted).toBe(true);

    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    await act(async () => {
      pending.resolve(jsonResponse(usablePayload()));
      await Promise.resolve();
    });
    expect(errorSpy).not.toHaveBeenCalled();
  });

  it("clears the reload timer on unmount", async () => {
    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });
    const callsAfterLoad = fetchMock.mock.calls.length;
    cleanup();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(120_000);
    });
    expect(fetchMock.mock.calls.length).toBe(callsAfterLoad);
  });

  it("times out hanging fetches with a safe message (AbortController + setTimeout)", async () => {
    // Hangs until the signal aborts, like a real fetch.
    fetchMock.mockImplementation(
      (_url: string, init?: { signal?: AbortSignal }) =>
        new Promise((_resolve, reject) => {
          const signal = init?.signal;
          if (!signal) {
            return;
          }
          if (signal.aborted) {
            reject(new DOMException("Aborted", "AbortError"));
            return;
          }
          signal.addEventListener("abort", () => {
            reject(new DOMException("Aborted", "AbortError"));
          });
        })
    );

    render(<HookProbe />);
    // Just short of the 40s budget: still waiting, no error yet.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(39_000);
    });
    expect(screen.getByTestId("is-error").textContent).toBe("false");

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_000);
    });
    await waitFor(() => {
      expect(screen.getByTestId("is-error").textContent).toBe("true");
    });
    expect(screen.getByTestId("msg").textContent).toBe(TIMEOUT_MESSAGE);
  });

  it("maps generic transport errors to a typed ApiError with a safe message", async () => {
    fetchMock.mockImplementation(async () => {
      throw new TypeError("Failed to fetch");
    });

    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("is-error").textContent).toBe("true");
    });
    // Raw exception text ("Failed to fetch") never reaches the UI.
    expect(screen.getByTestId("msg").textContent).toBe(TRANSPORT_MESSAGE);
  });

  it("keeps the real HTTP status and body log_id on the hard error envelope", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(
        { code: 404, msg: "接口不存在", log_id: "body-log-id", data: null },
        { ok: false, status: 404 }
      )
    );

    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("is-error").textContent).toBe("true");
    });
    expect(screen.getByTestId("code").textContent).toBe("404");
    expect(screen.getByTestId("msg").textContent).toBe("接口不存在");
    expect(screen.getByTestId("log-id").textContent).toBe("body-log-id");
  });

  it("falls back to the X-Log-Id header when the error body carries none", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(null, { ok: false, status: 502, logId: "header-log-id" })
    );

    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("is-error").textContent).toBe("true");
    });
    expect(screen.getByTestId("code").textContent).toBe("502");
    expect(screen.getByTestId("msg").textContent).toBe("请求失败 (502)");
    expect(screen.getByTestId("log-id").textContent).toBe("header-log-id");
  });

  it("surfaces the build version on the hard error envelope", async () => {
    // The settings dialog is reachable from the empty state, so a failure
    // must still name the running build.
    fetchMock.mockImplementation(async () =>
      jsonResponse(
        { code: 503, msg: "暂无数据", version: "v0.3.0", data: null },
        { ok: false, status: 503 }
      )
    );

    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("is-error").textContent).toBe("true");
    });
    expect(screen.getByTestId("version").textContent).toBe("v0.3.0");
  });

  it("manual retry reloads and clears the full-page error", async () => {
    fetchMock
      .mockImplementationOnce(async () =>
        jsonResponse(
          { code: 503, msg: "upstream down", data: null },
          { ok: false, status: 503 }
        )
      )
      .mockImplementationOnce(async () => jsonResponse(usablePayload()));

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
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(screen.getByTestId("is-error").textContent).toBe("false");
  });

  it("keeps the build version across a stale-snapshot merge", async () => {
    fetchMock
      .mockImplementationOnce(async () =>
        jsonResponse(
          usablePayload({
            version: "v0.3.0",
            data: { expires_at: new Date(Date.now() + 1_500).toISOString() },
          })
        )
      )
      .mockImplementationOnce(async () => {
        throw new TypeError("Failed to fetch");
      });

    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("version").textContent).toBe("v0.3.0");
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_000);
    });
    await waitFor(() => {
      expect(screen.getByTestId("stale").textContent).toBe("true");
    });
    // The transport failure carried no version of its own, so this can only
    // come from the retained snapshot.
    expect(screen.getByTestId("code").textContent).toBe("0");
    expect(screen.getByTestId("version").textContent).toBe("v0.3.0");
  });

  it("background reloads never show the full-page spinner", async () => {
    const pending = deferred();
    fetchMock
      .mockImplementationOnce(async () =>
        jsonResponse(
          usablePayload({
            data: { expires_at: new Date(Date.now() + 1_000).toISOString() },
          })
        )
      )
      .mockImplementationOnce(() => pending.promise);

    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1_500);
    });
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(2);
    });
    // The poll revalidation is background: indicator yes, full-page spin no,
    // and the campuses stay on screen.
    expect(screen.getByTestId("reloading").textContent).toBe("true");
    expect(screen.getByTestId("spinning").textContent).toBe("false");
    expect(screen.getByTestId("campus-count").textContent).toBe("1");

    await act(async () => {
      pending.resolve(jsonResponse(usablePayload()));
    });
    await waitFor(() => {
      expect(screen.getByTestId("reloading").textContent).toBe("false");
    });
  });

  it("does not poll while hidden and fires a single reload on becoming visible", async () => {
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => "hidden",
    });
    // First payload is near expiry; the reload after becoming visible gets a
    // long-lived day (no further rapid polls inside this window).
    let fetchCount = 0;
    fetchMock.mockImplementation(async () => {
      fetchCount += 1;
      return jsonResponse(
        usablePayload({
          data:
            fetchCount === 1
              ? { expires_at: new Date(Date.now() + 1_000).toISOString() }
              : {},
        })
      );
    });

    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });
    // The initial mount load still happened (background-opened tab).
    const callsAfterLoad = fetchMock.mock.calls.length;
    expect(callsAfterLoad).toBe(1);

    // Hidden: timers re-arm but never fetch.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    expect(fetchMock.mock.calls.length).toBe(callsAfterLoad);

    // Visible again: exactly one reload.
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => "visible",
    });
    await act(async () => {
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await waitFor(() => {
      expect(fetchMock.mock.calls.length).toBe(callsAfterLoad + 1);
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5_000);
    });
    expect(fetchMock.mock.calls.length).toBe(callsAfterLoad + 1);
  });

  it("drops the expired snapshot before the visible-again reload and restores data after", async () => {
    let visibility = "visible";
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => visibility,
    });

    const date = shanghaiToday();
    const staleUntil = new Date(Date.now() + 2_000).toISOString();
    let fetchCount = 0;
    let secondResponse: (value: unknown) => void = () => {};
    fetchMock.mockImplementation(
      () =>
        new Promise((resolve) => {
          fetchCount += 1;
          if (fetchCount === 1) {
            // Near expiry: expires_at in 1s, hard deadline in 2s.
            resolve(
              jsonResponse(
                usablePayload({
                  data: {
                    expires_at: new Date(Date.now() + 1_000).toISOString(),
                    stale_until: staleUntil,
                  },
                })
              )
            );
            return;
          }
          secondResponse = resolve;
        })
    );

    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });
    expect(screen.getByTestId("campus-count").textContent).toBe("1");
    const callsAfterLoad = fetchMock.mock.calls.length;

    // Hide and let the hard deadline pass while hidden.
    await act(async () => {
      visibility = "hidden";
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5_000);
    });
    expect(fetchMock.mock.calls.length).toBe(callsAfterLoad);

    // Become visible after stale_until: a single background reload fires and
    // the expired campuses are already gone from the render.
    await act(async () => {
      visibility = "visible";
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await waitFor(() => {
      expect(fetchMock.mock.calls.length).toBe(callsAfterLoad + 1);
    });
    expect(screen.getByTestId("campus-count").textContent).toBe("0");
    expect(screen.getByTestId("msg").textContent).toBe(EXPIRED_MESSAGE);
    // Background reload: no full-page spin over the expired-state card.
    expect(screen.getByTestId("spinning").textContent).toBe("false");

    await act(async () => {
      secondResponse(jsonResponse(usablePayload({ data: { date } })));
    });
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
      expect(screen.getByTestId("campus-count").textContent).toBe("1");
    });
  });

  it("drops an expired snapshot when the visible-again reload fails", async () => {
    let visibility = "visible";
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => visibility,
    });

    fetchMock
      .mockImplementationOnce(async () =>
        jsonResponse(
          usablePayload({
            data: { stale_until: new Date(Date.now() + 1_500).toISOString() },
          })
        )
      )
      .mockImplementationOnce(async () => {
        throw new TypeError("Failed to fetch");
      });

    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("campus-count").textContent).toBe("1");
    });

    await act(async () => {
      visibility = "hidden";
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3_000);
    });
    await act(async () => {
      visibility = "visible";
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await waitFor(() => {
      expect(screen.getByTestId("is-error").textContent).toBe("true");
    });
    // No usable snapshot left to keep: hard error, campuses cleared.
    expect(screen.getByTestId("campus-count").textContent).toBe("0");
    expect(screen.getByTestId("code").textContent).toBe("500");
    expect(screen.getByTestId("msg").textContent).toBe(TRANSPORT_MESSAGE);
  });

  it("throttles focus revalidation to one request per 15s", async () => {
    render(<HookProbe />);
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });
    const callsAfterLoad = fetchMock.mock.calls.length;

    // First focus revalidates immediately (nothing foreground since mount).
    await act(async () => {
      window.dispatchEvent(new Event("focus"));
    });
    await waitFor(() => {
      expect(fetchMock.mock.calls.length).toBe(callsAfterLoad + 1);
    });

    // Rapid refocuses inside the throttle window are absorbed.
    await act(async () => {
      window.dispatchEvent(new Event("focus"));
      document.dispatchEvent(new Event("visibilitychange"));
    });
    expect(fetchMock.mock.calls.length).toBe(callsAfterLoad + 1);

    // Past the 15s window a new focus revalidates — and a paired
    // visibilitychange still cannot double-fire it.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(15_500);
    });
    await act(async () => {
      window.dispatchEvent(new Event("focus"));
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await waitFor(() => {
      expect(fetchMock.mock.calls.length).toBe(callsAfterLoad + 2);
    });
    expect(fetchMock.mock.calls.length).toBe(callsAfterLoad + 2);
  });
});
