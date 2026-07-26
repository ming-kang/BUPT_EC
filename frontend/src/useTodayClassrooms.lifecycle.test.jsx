/**
 * @vitest-environment jsdom
 *
 * Real mount/unmount harness for the SWR-backed useTodayClassrooms. Pure
 * helper cases stay in useTodayClassrooms.test.js.
 *
 * Harness notes (spike-verified):
 * - Every case wraps the probe in SWRConfig with `provider: () => new Map()`
 *   so SWR's module-global cache never leaks data/error/dedupe state across
 *   cases.
 * - dedupingInterval stays at the production default (2s) on purpose: the
 *   always-alive polling chain fires a bootstrap tick ~1s after mount (the
 *   pre-first-data interval) and production deduping absorbs it. Setting it
 *   to 0 would turn that tick into a spurious request and make fetch counts
 *   flaky. Manual retry still works because bound mutate() bypasses deduping.
 * - AbortSignal.timeout is re-implemented on top of the vitest-patched global
 *   setTimeout where the test needs to drive the 40s budget (fake timers
 *   cannot advance jsdom's internal timeout).
 * - Node prints a harmless `TimeoutNaNWarning` on visibilitychange dispatch:
 *   SWR registers `setTimeout.bind(undefined, revalidateAllKeys)` as the
 *   focus listener, so the DOM event object lands in the delay slot (NaN →
 *   clamped to 1ms). Upstream artifact of swr@2.4.2, not a test bug.
 */
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SWRConfig } from "swr";
import useTodayClassrooms, {
  CLIENT_FETCH_TIMEOUT_MESSAGE,
  CLIENT_FETCH_TIMEOUT_MS,
} from "./useTodayClassrooms";

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
      <div data-testid="log-id">{resp.logId || ""}</div>
      <button type="button" onClick={retry}>
        retry
      </button>
    </div>
  );
}

function renderProbe(config = {}) {
  return render(
    <SWRConfig value={{ provider: () => new Map(), ...config }}>
      <HookProbe />
    </SWRConfig>
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

/** Rebuild AbortSignal.timeout on the patched setTimeout so fake timers drive it. */
function stubAbortSignalTimeout() {
  return vi.spyOn(AbortSignal, "timeout").mockImplementation((ms) => {
    const controller = new AbortController();
    setTimeout(() => {
      controller.abort(
        new DOMException("The operation timed out.", "TimeoutError")
      );
    }, ms);
    return controller.signal;
  });
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
    // Drop any per-case visibilityState override (falls back to the
    // prototype's "visible") so hidden-tab cases cannot leak forward.
    Reflect.deleteProperty(document, "visibilityState");
  });

  it("loads initial data on mount", async () => {
    renderProbe();
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

  it("ignores late responses after unmount without aborting the fetch", async () => {
    const pending = deferred();
    let seenSignal;
    fetch.mockImplementation((_url, init) => {
      seenSignal = init.signal;
      return pending.promise;
    });

    const view = renderProbe();
    await waitFor(() => {
      expect(fetch).toHaveBeenCalledTimes(1);
    });
    // The fetcher always attaches its 40s timeout budget signal.
    expect(seenSignal).toBeInstanceOf(AbortSignal);
    expect(seenSignal.aborted).toBe(false);

    view.unmount();
    // SWR does not abort in-flight requests on unmount (the timeout signal
    // still bounds them); it only ignores the late result.
    expect(seenSignal.aborted).toBe(false);

    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    await act(async () => {
      pending.resolve({
        ok: true,
        status: 200,
        json: async () => usablePayload(),
      });
      await Promise.resolve();
    });
    // No act warning, no unmounted-state update noise.
    expect(errorSpy).not.toHaveBeenCalled();
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

    renderProbe();
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

    // dedupingInterval 0 here: the next background revalidation must issue a
    // real second request instead of being absorbed by the dedupe window.
    renderProbe({ dedupingInterval: 0 });
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
    renderProbe();
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

  it("times out hanging fetches with a safe message", async () => {
    const timeoutSpy = stubAbortSignalTimeout();
    fetch.mockImplementation(
      (_url, init) =>
        new Promise((_resolve, reject) => {
          const signal = init?.signal;
          if (!signal) {
            return;
          }
          if (signal.aborted) {
            reject(signal.reason);
            return;
          }
          signal.addEventListener("abort", () => {
            reject(signal.reason);
          });
        })
    );

    renderProbe();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(CLIENT_FETCH_TIMEOUT_MS + 10);
    });
    await waitFor(() => {
      expect(screen.getByTestId("is-error").textContent).toBe("true");
    });
    expect(screen.getByTestId("msg").textContent).toBe(
      CLIENT_FETCH_TIMEOUT_MESSAGE
    );
    expect(timeoutSpy).toHaveBeenCalledWith(CLIENT_FETCH_TIMEOUT_MS);
  });

  it("keeps the real HTTP status and body log_id in the error envelope", async () => {
    fetch.mockImplementation(async () => ({
      ok: false,
      status: 404,
      headers: { get: (name) => (name === "X-Log-Id" ? "header-log-id" : null) },
      json: async () => ({
        code: 404,
        msg: "接口不存在",
        log_id: "body-log-id",
        data: null,
      }),
    }));

    renderProbe();
    await waitFor(() => {
      expect(screen.getByTestId("is-error").textContent).toBe("true");
    });
    // The derived envelope carries the real status, not a guessed 500.
    expect(screen.getByTestId("code").textContent).toBe("404");
    expect(screen.getByTestId("msg").textContent).toBe("接口不存在");
    expect(screen.getByTestId("log-id").textContent).toBe("body-log-id");
  });

  it("falls back to the X-Log-Id header when the error body has no log_id", async () => {
    fetch.mockImplementation(async () => ({
      ok: false,
      status: 502,
      headers: { get: (name) => (name === "X-Log-Id" ? "header-log-id" : null) },
      json: async () => null,
    }));

    renderProbe();
    await waitFor(() => {
      expect(screen.getByTestId("is-error").textContent).toBe("true");
    });
    expect(screen.getByTestId("code").textContent).toBe("502");
    expect(screen.getByTestId("msg").textContent).toBe("请求失败 (502)");
    expect(screen.getByTestId("log-id").textContent).toBe("header-log-id");
  });

  it("keeps background polling alive after the first load (no falsy chain death)", async () => {
    // Guards the SWR chain-death trap: a falsy refreshInterval return would
    // silently end all future polling; nextReloadDelay(undefined) is null.
    vi.spyOn(Math, "random").mockReturnValue(0);
    fetch.mockImplementation(async () => ({
      ok: true,
      status: 200,
      json: async () => usablePayload({ data: { stale: true } }),
    }));

    renderProbe();
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });
    const callsAfterLoad = fetch.mock.calls.length;

    // Stale payload → 15s base poll. The ~1s bootstrap tick is deduped; the
    // chain then re-arms from the real snapshot.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(17_000);
    });
    await waitFor(() => {
      expect(fetch.mock.calls.length).toBe(callsAfterLoad + 1);
    });
  });

  it("does not schedule background reloads while the page is hidden", async () => {
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => "hidden",
    });
    fetch.mockImplementation(async () => ({
      ok: true,
      status: 200,
      json: async () =>
        usablePayload({
          data: { expires_at: new Date(Date.now() + 1_000).toISOString() },
        }),
    }));

    renderProbe();
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });
    const callsAfterLoad = fetch.mock.calls.length;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30_000);
    });
    expect(fetch.mock.calls.length).toBe(callsAfterLoad);
  });

  it("abandons an armed failure retry while hidden and recovers on focus", async () => {
    // SWR fires already-armed error-retry timers even after the tab hides;
    // retryOnError's callback must give up when hidden and let
    // revalidateOnFocus reissue the request on return.
    vi.spyOn(Math, "random").mockReturnValue(0);
    let visibility = "visible";
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => visibility,
    });
    fetch
      .mockImplementationOnce(async () => {
        throw new Error("network down");
      })
      .mockImplementation(async () => ({
        ok: true,
        status: 200,
        json: async () => usablePayload(),
      }));

    renderProbe();
    await waitFor(() => {
      expect(screen.getByTestId("is-error").textContent).toBe("true");
    });
    const callsAfterError = fetch.mock.calls.length;

    // The 10s ladder timer was armed while visible; hide before it fires.
    await act(async () => {
      visibility = "hidden";
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30_000);
    });
    // Timer fired but gave up: hidden tabs never fetch.
    expect(fetch.mock.calls.length).toBe(callsAfterError);

    // Becoming visible again revalidates promptly and clears the error.
    await act(async () => {
      visibility = "visible";
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });
    expect(fetch.mock.calls.length).toBe(callsAfterError + 1);
  });

  it("clears expired snapshot and reloads once when becoming visible after stale_until", async () => {
    vi.spyOn(Math, "random").mockReturnValue(0);
    let visibility = "visible";
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => visibility,
    });

    const date = shanghaiToday();
    // Short remaining lifetime so we can cross the hard deadline while hidden.
    const staleUntil = new Date(Date.now() + 2_000).toISOString();
    let fetchCount = 0;
    fetch.mockImplementation(async () => {
      fetchCount += 1;
      // First response is near expiry; later reloads return a long-lived day.
      if (fetchCount === 1) {
        return {
          ok: true,
          status: 200,
          json: async () =>
            usablePayload({
              data: {
                date,
                expires_at: new Date(Date.now() + 1_000).toISOString(),
                stale_until: staleUntil,
              },
            }),
        };
      }
      return {
        ok: true,
        status: 200,
        json: async () => usablePayload(),
      };
    });

    renderProbe();
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
    });
    expect(screen.getByTestId("campus-count").textContent).toBe("1");
    const callsAfterLoad = fetch.mock.calls.length;

    // Hide: the polling chain keeps re-arming but never fetches while hidden.
    await act(async () => {
      visibility = "hidden";
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5_000);
    });
    expect(fetch.mock.calls.length).toBe(callsAfterLoad);
    // Stale snapshot may still be in state while hidden.
    expect(screen.getByTestId("campus-count").textContent).toBe("1");

    // Become visible after the hard deadline: revalidateOnFocus reloads
    // immediately; the pending chain tick is absorbed by the dedupe window.
    await act(async () => {
      visibility = "visible";
      document.dispatchEvent(new Event("visibilitychange"));
    });
    // Duplicate visible events are throttled (focusThrottleInterval).
    await act(async () => {
      document.dispatchEvent(new Event("visibilitychange"));
    });

    await waitFor(() => {
      expect(fetch.mock.calls.length).toBe(callsAfterLoad + 1);
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1_000);
    });
    // Exactly one reload after resume (no ordinary 15s stale poll thrash).
    expect(fetch.mock.calls.length).toBe(callsAfterLoad + 1);
    await waitFor(() => {
      expect(screen.getByTestId("code").textContent).toBe("0");
      expect(screen.getByTestId("campus-count").textContent).toBe("1");
    });
  });
});
