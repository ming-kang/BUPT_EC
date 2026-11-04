import { useCallback, useEffect, useRef, useState } from "react";
import {
  ApiError,
  fallbackErrorMessage,
  isUsableBusinessDaySnapshot,
  loadingResponse,
  type Envelope,
} from "./api/decode";
import { requestTodayClassrooms } from "./api/getData";
import { nextReloadDelay } from "./reloadSchedule";

/**
 * Foreground revalidations (window focus / tab visible again) are throttled
 * to the stale-poll cadence so multi-tab switching stays inside the Nginx
 * 30 req/min budget.
 */
const FOCUS_THROTTLE_MS = 15_000;

const EXPIRED_SNAPSHOT_MESSAGE = "当前缓存已失效，正在重新获取";

/**
 * Safe message for transport-level failures; raw exception text stays out of
 * the UI (and out of any log line the user might paste into an issue).
 */
const TRANSPORT_FAILURE_MESSAGE = "网络连接异常，请检查网络后重试";

type RequestReason = "initial" | "poll" | "foreground" | "manual";

/**
 * Fetch + decode `/api/get_data` with the hook's lifecycle semantics:
 * - full-page Spin only when a foreground load has no usable data;
 * - auto-reload via `nextReloadDelay` (fresh → expires_at, stale ≥ 15s,
 *   partial ≥ 30s, failures 10s/20s/30s/60s, all clamped to stale_until);
 * - the last good snapshot survives failures and is dropped at render time
 *   once it stops being a usable business-day snapshot;
 * - hidden tabs issue no requests, one throttled reload on return;
 * - in-flight requests abort on unmount; requests never overlap.
 */
export default function useTodayClassrooms() {
  const [data, setData] = useState<Envelope | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [inflight, setInflight] = useState(false);
  const [manualReload, setManualReload] = useState(false);

  // Refs mirror the state the scheduler reads outside React's render flow.
  // armTimerRef breaks the request <-> armTimer cycle for React's lint.
  const dataRef = useRef<Envelope | null>(null);
  const failureCountRef = useRef(0);
  const timerRef = useRef<number | null>(null);
  const inflightRef = useRef(false);
  const abortRef = useRef<AbortController | null>(null);
  const disposedRef = useRef(false);
  const lastForegroundRef = useRef(0);
  const armTimerRef = useRef<() => void>(() => {});

  const request = useCallback((reason: RequestReason) => {
    // Never overlap: an in-flight request already refreshes every track.
    if (inflightRef.current) {
      return;
    }
    // Hidden tabs issue no requests. The initial mount load is the one
    // exception: a background-opened tab still needs its first payload.
    if (
      reason !== "initial" &&
      typeof document !== "undefined" &&
      document.visibilityState === "hidden"
    ) {
      return;
    }

    const controller = new AbortController();
    abortRef.current = controller;
    inflightRef.current = true;
    if (reason === "manual") {
      setManualReload(true);
    }
    setInflight(true);

    requestTodayClassrooms(controller.signal)
      .then((envelope) => {
        if (disposedRef.current) {
          return;
        }
        failureCountRef.current = 0;
        dataRef.current = envelope;
        setData(envelope);
        setError(null);
      })
      .catch((rawError) => {
        if (disposedRef.current) {
          return;
        }
        failureCountRef.current += 1;
        // Generic transport failures become a typed ApiError with a safe
        // message; raw exception text never reaches the UI.
        setError(
          rawError instanceof ApiError
            ? rawError
            : new ApiError(TRANSPORT_FAILURE_MESSAGE)
        );
      })
      .finally(() => {
        if (disposedRef.current) {
          return;
        }
        inflightRef.current = false;
        abortRef.current = null;
        setInflight(false);
        setManualReload(false);
        armTimerRef.current();
      });
  }, []);

  const armTimer = useCallback(() => {
    if (timerRef.current !== null) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    if (disposedRef.current || inflightRef.current) {
      return;
    }
    const delay = nextReloadDelay(dataRef.current?.data ?? null, {
      failureCount: failureCountRef.current,
    });
    if (delay === null) {
      return;
    }
    timerRef.current = window.setTimeout(() => {
      timerRef.current = null;
      if (disposedRef.current) {
        return;
      }
      if (
        typeof document !== "undefined" &&
        document.visibilityState === "hidden"
      ) {
        // No requests while hidden: re-arm from the current state. Once the
        // cached snapshot stops being displayable this becomes the 1s floor,
        // so becoming visible again reloads promptly.
        armTimer();
        return;
      }
      request("poll");
    }, delay);
  }, [request]);

  const onForeground = useCallback(() => {
    const nowMs = Date.now();
    if (nowMs - lastForegroundRef.current < FOCUS_THROTTLE_MS) {
      return;
    }
    lastForegroundRef.current = nowMs;
    request("foreground");
  }, [request]);

  useEffect(() => {
    disposedRef.current = false;
    armTimerRef.current = armTimer;

    function onVisibilityChange() {
      if (document.visibilityState !== "hidden") {
        onForeground();
      }
    }

    request("initial");
    window.addEventListener("focus", onForeground);
    document.addEventListener("visibilitychange", onVisibilityChange);

    return () => {
      disposedRef.current = true;
      window.removeEventListener("focus", onForeground);
      document.removeEventListener("visibilitychange", onVisibilityChange);
      if (timerRef.current !== null) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
      // The in-flight request dies with the component.
      abortRef.current?.abort();
      abortRef.current = null;
      inflightRef.current = false;
    };
  }, [request, onForeground, armTimer]);

  const retry = useCallback(() => {
    // User-initiated: the spin policy distinguishes this from background
    // revalidation, but an in-flight request still wins (never overlap).
    request("manual");
  }, [request]);

  /* ------------------------- render-time derivation ------------------------- */

  const nowMs = Date.now();
  // A code-0 snapshot survives only while it stays a usable business-day
  // snapshot; midnight / stale_until crossings drop it here, at render time,
  // so expired campuses never stay on screen while the prompt reload runs.
  const usableData =
    data !== null && isUsableBusinessDaySnapshot(data.data, nowMs)
      ? data
      : null;
  const hasUsableData = usableData !== null;
  const neverResolved = data === null && error === null;
  const isBackground = !neverResolved && !manualReload;
  // Full-page Spin only for initial load / manual retry without usable data.
  // Background auto-reload never takes over the whole page.
  const spinning = inflight && !isBackground && !hasUsableData;

  // Envelope-level fields do not survive merges; fall back to the last
  // snapshot that carried one (the running build cannot change between polls).
  const cachedVersion = data?.version || "";

  let resp: Envelope;
  if (spinning || neverResolved) {
    resp = loadingResponse;
  } else if (error && usableData && usableData.data) {
    // Soft failure: keep the last good snapshot visible and flag it stale
    // (reuses the existing stale/error Alert).
    resp = {
      ...usableData,
      data: {
        ...usableData.data,
        stale: true,
        error: {
          type: "client_refresh_failed",
          message: error.message || fallbackErrorMessage,
        },
      },
    };
  } else if (error) {
    // Hard failure with no displayable snapshot. Code precedence mirrors the
    // pre-SWR rule: the real HTTP status when the transport failed, else
    // the non-zero business code of a service error envelope, 500 last.
    const status = Number.isFinite(error.status) ? error.status : null;
    const businessCode =
      Number.isFinite(error.code) && error.code !== 0 ? error.code : null;
    resp = {
      code: status ?? businessCode ?? 500,
      msg: error.message || fallbackErrorMessage,
      data: null,
      ...(error.version || cachedVersion
        ? { version: error.version || cachedVersion }
        : {}),
      ...(error.logId ? { logId: error.logId } : {}),
    };
  } else if (usableData) {
    resp = usableData;
  } else {
    // Code-0 snapshot crossed midnight or stale_until between reloads; the
    // clamped prompt reload (1s floor) is already scheduled or in flight.
    resp = {
      code: 500,
      msg: EXPIRED_SNAPSHOT_MESSAGE,
      data: null,
      ...(cachedVersion ? { version: cachedVersion } : {}),
    };
  }

  return {
    resp,
    spinning,
    reloading: inflight && !spinning,
    isError: resp.code !== 0 && !spinning,
    retry,
  };
}
