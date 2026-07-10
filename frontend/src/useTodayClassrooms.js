import { useCallback, useEffect, useRef, useState } from "react";
import { isUsableBusinessDaySnapshot } from "./classroomDataValidity";
import { nextReloadDelay } from "./reloadSchedule";
import {
  extractMessage,
  fallbackErrorMessage,
  loadingResponse,
  normalizeResponse,
  readJson,
} from "./todayClassroomsResponse";

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

function errorEnvelope(message, code = 500) {
  return {
    code,
    msg: message || fallbackErrorMessage,
    data: null,
  };
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
  const respRef = useRef(resp);
  respRef.current = resp;

  useEffect(() => {
    const controller = new AbortController();
    const isBackground = reloadRequest.background;

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

        if (!response.ok) {
          throw new Error(
            extractMessage(payload) || `请求失败 (${response.status})`
          );
        }

        const normalized = normalizeResponse(payload);
        const nowMs = Date.now();
        const succeeded = hasUsableClassroomData(normalized, nowMs);
        setFailureCount((current) => nextFailureCount(current, succeeded));
        setResp((current) => {
          const merged = mergeFetchResult(current, normalized, nowMs);
          respRef.current = merged;
          return merged;
        });
      } catch (error) {
        if (controller.signal.aborted) {
          return;
        }
        const failed = errorEnvelope(
          error instanceof Error ? error.message : fallbackErrorMessage
        );
        const nowMs = Date.now();
        setFailureCount((current) => nextFailureCount(current, false));
        setResp((current) => {
          const merged = mergeFetchResult(current, failed, nowMs);
          respRef.current = merged;
          return merged;
        });
      } finally {
        if (!controller.signal.aborted) {
          setSpinning(false);
          setReloading(false);
        }
      }
    }

    loadData();
    return () => controller.abort();
  }, [reloadRequest]);

  const retry = useCallback(() => {
    setReloadRequest((current) => ({
      key: current.key + 1,
      background: false,
    }));
  }, []);

  useEffect(() => {
    if (spinning || reloading) {
      return;
    }
    const delay = nextReloadDelay(resp.data, { failureCount });
    if (delay == null) {
      return;
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
  }, [failureCount, reloading, resp, spinning]);

  return {
    resp,
    spinning,
    reloading,
    isError: resp.code !== 0 && !spinning,
    retry,
  };
}
