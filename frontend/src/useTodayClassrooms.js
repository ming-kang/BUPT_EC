import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError } from "./apiError";
import { isUsableBusinessDaySnapshot } from "./classroomDataValidity";
import { nextReloadDelay } from "./reloadSchedule";
import {
  extractMessage,
  fallbackErrorMessage,
  loadingResponse,
  normalizeResponse,
  readJson,
} from "./todayClassroomsResponse";

/**
 * Client fetch budget: above ClassroomRefreshLimit (30s), below Go WriteTimeout
 * (45s) and Nginx /api proxy_read_timeout (60s).
 */
export const CLIENT_FETCH_TIMEOUT_MS = 40_000;
export const CLIENT_FETCH_TIMEOUT_MESSAGE = "请求超时，请稍后重试";

/** True when the response can still drive the classroom UI. */
export function hasUsableClassroomData(resp, nowMs = Date.now()) {
  return (
    resp?.code === 0 &&
    isUsableBusinessDaySnapshot(resp?.data, nowMs)
  );
}

/**
 * Full-page Spin only for initial load / manual retry without usable data.
 * Background auto-reload never takes over the whole page.
 */
export function shouldFullPageSpin(isBackground, hasUsableData) {
  if (isBackground) {
    return false;
  }
  return !hasUsableData;
}

export function nextFailureCount(current, succeeded) {
  return succeeded ? 0 : current + 1;
}

/**
 * Merge a new fetch outcome into prior UI state.
 * On failure after a successful snapshot, keep campuses and attach a soft error
 * (reuses the existing stale/error Alert). Hard-empty only with no prior good data.
 */
export function mergeFetchResult(prev, next, nowMs = Date.now()) {
  const nextIsSuccessfulEnvelope = next?.code === 0 && next.data != null;
  if (hasUsableClassroomData(next, nowMs)) {
    return next;
  }

  const msg =
    (typeof next?.msg === "string" && next.msg.trim() !== ""
      ? next.msg.trim()
      : "") || fallbackErrorMessage;

  if (hasUsableClassroomData(prev, nowMs)) {
    return {
      code: 0,
      msg: typeof prev.msg === "string" ? prev.msg : "",
      data: {
        ...prev.data,
        stale: true,
        error: {
          type: "client_refresh_failed",
          message: msg,
        },
      },
    };
  }

  const code = nextIsSuccessfulEnvelope ? 500 : Number(next?.code);
  return {
    code: Number.isFinite(code) ? code : 500,
    msg,
    data: null,
  };
}

function errorEnvelope(message, code = 500, logId = "") {
  return {
    code,
    msg: message || fallbackErrorMessage,
    data: null,
    logId,
  };
}

function isPageVisible() {
  if (typeof document === "undefined") {
    return true;
  }
  return document.visibilityState !== "hidden";
}

export default function useTodayClassrooms() {
  const [reloadRequest, setReloadRequest] = useState({
    key: 0,
    background: false,
  });
  const [spinning, setSpinning] = useState(true);
  const [reloading, setReloading] = useState(false);
  const [failureCount, setFailureCount] = useState(0);
  const [resp, setResp] = useState(loadingResponse);
  const [pageVisible, setPageVisible] = useState(isPageVisible);
  const respRef = useRef(resp);
  respRef.current = resp;

  useEffect(() => {
    if (typeof document === "undefined") {
      return undefined;
    }
    const onVisibility = () => {
      setPageVisible(document.visibilityState !== "hidden");
    };
    document.addEventListener("visibilitychange", onVisibility);
    return () => document.removeEventListener("visibilitychange", onVisibility);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    let timedOut = false;
    const isBackground = reloadRequest.background;
    const timeoutId = setTimeout(() => {
      timedOut = true;
      controller.abort();
    }, CLIENT_FETCH_TIMEOUT_MS);

    async function loadData() {
      const usable = hasUsableClassroomData(respRef.current);
      const fullPageSpin = shouldFullPageSpin(isBackground, usable);

      setSpinning(fullPageSpin);
      // Subtle in-flight flag for background (or any non-full-page) reloads.
      setReloading(!fullPageSpin);

      if (fullPageSpin && !usable) {
        setResp(loadingResponse);
        respRef.current = loadingResponse;
      }

      try {
        const response = await fetch("/api/get_data", {
          signal: controller.signal,
          headers: { Accept: "application/json" },
        });
        const payload = await readJson(response);
        const logId =
          payload?.log_id || response.headers?.get("X-Log-Id") || "";
        const businessCode = Number(payload?.code);
        const apiErrorDetails = {
          status: response.status,
          code: Number.isFinite(businessCode) ? businessCode : undefined,
          logId,
        };

        if (!response.ok) {
          throw new ApiError(
            extractMessage(payload) || `请求失败 (${response.status})`,
            apiErrorDetails
          );
        }

        const normalized = normalizeResponse(payload);
        if (!normalized.ok) {
          throw new ApiError(normalized.reason, apiErrorDetails);
        }

        const nowMs = Date.now();
        const succeeded = hasUsableClassroomData(normalized.resp, nowMs);
        setFailureCount((current) => nextFailureCount(current, succeeded));
        setResp((current) => {
          const merged = mergeFetchResult(current, normalized.resp, nowMs);
          respRef.current = merged;
          return merged;
        });
      } catch (error) {
        if (controller.signal.aborted && !timedOut) {
          return;
        }
        // ApiError keeps the real HTTP status and log_id; anything else stays
        // on the existing safe-message path with the generic 500 code.
        const failed =
          !timedOut && error instanceof ApiError
            ? errorEnvelope(error.message, error.status, error.logId)
            : errorEnvelope(
                timedOut
                  ? CLIENT_FETCH_TIMEOUT_MESSAGE
                  : error instanceof Error
                    ? error.message
                    : fallbackErrorMessage
              );
        const nowMs = Date.now();
        setFailureCount((current) => nextFailureCount(current, false));
        setResp((current) => {
          const merged = mergeFetchResult(current, failed, nowMs);
          // mergeFetchResult rebuilds hard-empty envelopes without logId;
          // re-attach it so the error UI can surface it (stale-snapshot
          // merges keep code 0 and never show the hard error card).
          const next =
            merged.code !== 0 && failed.logId
              ? { ...merged, logId: failed.logId }
              : merged;
          respRef.current = next;
          return next;
        });
      } finally {
        clearTimeout(timeoutId);
        if (!controller.signal.aborted || timedOut) {
          setSpinning(false);
          setReloading(false);
        }
      }
    }

    loadData();
    return () => {
      clearTimeout(timeoutId);
      controller.abort();
    };
  }, [reloadRequest]);

  const retry = useCallback(() => {
    setReloadRequest((current) => ({
      key: current.key + 1,
      background: false,
    }));
  }, []);

  useEffect(() => {
    if (!pageVisible || spinning || reloading) {
      return undefined;
    }
    const delay = nextReloadDelay(resp.data, { failureCount });
    if (delay == null) {
      return undefined;
    }
    const timer = setTimeout(() => {
      if (resp.code === 0 && !hasUsableClassroomData(resp)) {
        const expired = errorEnvelope("当前缓存已失效，正在重新获取");
        respRef.current = expired;
        setResp(expired);
      }
      setReloadRequest((current) => ({
        key: current.key + 1,
        background: true,
      }));
    }, delay);
    return () => clearTimeout(timer);
  }, [failureCount, pageVisible, reloading, resp, spinning]);

  return {
    resp,
    spinning,
    reloading,
    isError: resp.code !== 0 && !spinning,
    retry,
  };
}
