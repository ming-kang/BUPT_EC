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

// Keep focus / visibility reloads inside the Nginx 30 req/min budget.
const FOCUS_THROTTLE_MS = 15_000;
const EXPIRED_SNAPSHOT_MESSAGE = "当前缓存已失效，正在重新获取";
const TRANSPORT_FAILURE_MESSAGE = "网络连接异常，请检查网络后重试";

type RequestReason = "initial" | "poll" | "foreground" | "manual";

export default function useTodayClassrooms() {
  const [data, setData] = useState<Envelope | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [requestReason, setRequestReason] = useState<RequestReason | null>(null);
  const retryRef = useRef<() => void>(() => {});

  useEffect(() => {
    let snapshot: Envelope | null = null;
    let failureCount = 0;
    let timer: number | undefined;
    let controller: AbortController | null = null;
    let disposed = false;
    let lastForeground = 0;

    function armTimer() {
      const delay = nextReloadDelay(snapshot?.data ?? null, { failureCount });
      if (delay === null) {
        return;
      }
      timer = window.setTimeout(() => {
        if (document.visibilityState === "hidden") {
          armTimer();
        } else {
          request("poll");
        }
      }, delay);
    }

    function canStart(reason: RequestReason) {
      return (
        controller === null &&
        (reason === "initial" || document.visibilityState !== "hidden")
      );
    }

    async function request(reason: RequestReason) {
      if (!canStart(reason)) {
        return;
      }
      window.clearTimeout(timer);
      controller = new AbortController();
      setRequestReason(reason);

      try {
        const envelope = await requestTodayClassrooms(controller.signal);
        if (disposed) {
          return;
        }
        failureCount = 0;
        snapshot = envelope;
        setData(envelope);
        setError(null);
      } catch (rawError) {
        if (disposed) {
          return;
        }
        failureCount += 1;
        setError(
          rawError instanceof ApiError
            ? rawError
            : new ApiError(TRANSPORT_FAILURE_MESSAGE)
        );
      } finally {
        if (!disposed) {
          controller = null;
          setRequestReason(null);
          armTimer();
        }
      }
    }

    function onForeground() {
      const nowMs = Date.now();
      if (nowMs - lastForeground < FOCUS_THROTTLE_MS) {
        return;
      }
      lastForeground = nowMs;
      request("foreground");
    }

    function onVisibilityChange() {
      if (document.visibilityState !== "hidden") {
        onForeground();
      }
    }

    retryRef.current = () => request("manual");
    request("initial");
    window.addEventListener("focus", onForeground);
    document.addEventListener("visibilitychange", onVisibilityChange);

    return () => {
      disposed = true;
      window.removeEventListener("focus", onForeground);
      document.removeEventListener("visibilitychange", onVisibilityChange);
      window.clearTimeout(timer);
      controller?.abort();
    };
  }, []);

  const retry = useCallback(() => retryRef.current(), []);

  const usableData =
    data !== null && isUsableBusinessDaySnapshot(data.data) ? data : null;
  const neverResolved = data === null && error === null;
  const inflight = requestReason !== null;
  const spinning =
    inflight &&
    (neverResolved || requestReason === "manual") &&
    usableData === null;
  const cachedVersion = data?.version || "";

  let resp: Envelope;
  if (spinning || neverResolved) {
    resp = loadingResponse;
  } else if (error && usableData && usableData.data) {
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
