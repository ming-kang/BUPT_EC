import { useCallback, useState } from "react";
import useSWR from "swr";
import { ApiError } from "./apiError";
import { isUsableBusinessDaySnapshot } from "./classroomDataValidity";
import { MIN_FRESH_DELAY_MS, nextReloadDelay } from "./reloadSchedule";
import {
  extractMessage,
  fallbackErrorMessage,
  loadingResponse,
  normalizeResponse,
  readJson,
  type Envelope,
  type PayloadView,
} from "./todayClassroomsResponse";

/**
 * Client fetch budget: above ClassroomRefreshLimit (30s), below Go WriteTimeout
 * (45s) and Nginx /api proxy_read_timeout (60s).
 */
export const CLIENT_FETCH_TIMEOUT_MS = 40_000;
export const CLIENT_FETCH_TIMEOUT_MESSAGE = "请求超时，请稍后重试";

export const TODAY_CLASSROOMS_KEY = "/api/get_data";

/**
 * SWR refetches on every visibilitychange/focus event. Throttle to the stale
 * poll cadence so multi-tab switching stays inside the Nginx 30 req/min budget.
 */
export const FOCUS_THROTTLE_MS = 15_000;

const EXPIRED_SNAPSHOT_MESSAGE = "当前缓存已失效，正在重新获取";

/** Hook-facing envelope: normalized shape plus the optional client log id. */
export type HookEnvelope = Envelope & { logId?: string };

/** Structural view for pure helpers that run before shapes are trusted. */
export interface EnvelopeView {
  code?: unknown;
  msg?: unknown;
  data?: unknown;
}

/** True when the response can still drive the classroom UI. */
export function hasUsableClassroomData(
  resp?: EnvelopeView | null,
  nowMs: number = Date.now()
): boolean {
  return (
    resp?.code === 0 &&
    isUsableBusinessDaySnapshot(resp?.data, nowMs)
  );
}

/**
 * Full-page Spin only for initial load / manual retry without usable data.
 * Background auto-reload never takes over the whole page.
 */
export function shouldFullPageSpin(isBackground: boolean, hasUsableData: boolean): boolean {
  if (isBackground) {
    return false;
  }
  return !hasUsableData;
}

/**
 * Merge a new fetch outcome into prior UI state.
 * On failure after a successful snapshot, keep campuses and attach a soft error
 * (reuses the existing stale/error Alert). Hard-empty only with no prior good data.
 */
/** Loose envelope shape returned by the merge (fields read by tests/UI). */
export interface MergedEnvelope {
  code: number;
  msg: string;
  data: Record<string, unknown> | null;
}

export function mergeFetchResult(
  prev?: EnvelopeView | null,
  next?: EnvelopeView | null,
  nowMs: number = Date.now()
): MergedEnvelope {
  const nextIsSuccessfulEnvelope = next?.code === 0 && next.data != null;
  if (hasUsableClassroomData(next, nowMs)) {
    return next as unknown as MergedEnvelope;
  }

  const msg =
    (typeof next?.msg === "string" && next.msg.trim() !== ""
      ? next.msg.trim()
      : "") || fallbackErrorMessage;

  if (hasUsableClassroomData(prev, nowMs)) {
    return {
      code: 0,
      msg: typeof prev!.msg === "string" ? prev!.msg : "",
      data: {
        ...(prev!.data as object),
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

function errorEnvelope(message: string, code = 500, logId = "") {
  return {
    code,
    msg: message || fallbackErrorMessage,
    data: null,
    logId,
  };
}

/**
 * Envelope code precedence mirrors the pre-SWR behavior: the real HTTP status
 * when the transport failed, otherwise the non-zero business code carried by
 * an HTTP-2xx service error envelope, and 500 only as the last resort.
 */
function errorAsEnvelope(error: unknown): HookEnvelope {
  if (error instanceof ApiError) {
    const status = Number.isFinite(error.status) ? error.status : null;
    const businessCode =
      Number.isFinite(error.code) && error.code !== 0 ? error.code : null;
    return errorEnvelope(
      error.message,
      status ?? businessCode ?? 500,
      error.logId
    );
  }
  return errorEnvelope(
    error instanceof Error && error.message ? error.message : ""
  );
}

/**
 * SWR fetcher. Throwing is the single error channel: any outcome that must
 * not replace the cached snapshot (transport failure, malformed payload,
 * service error envelope, unusable business-day metadata) throws an ApiError;
 * SWR then keeps the previous data on its data track ("failure preserves the
 * last snapshot") and routes scheduling through onErrorRetry.
 */
export async function fetchTodayClassrooms(url: string): Promise<Envelope> {
  let response;
  try {
    response = await fetch(url, {
      headers: { Accept: "application/json" },
      signal: AbortSignal.timeout(CLIENT_FETCH_TIMEOUT_MS),
    });
  } catch (error) {
    if ((error as { name?: string } | undefined)?.name === "TimeoutError") {
      throw new ApiError(CLIENT_FETCH_TIMEOUT_MESSAGE, {});
    }
    throw error;
  }

  const payload = await readJson(response);
  const record = payload as PayloadView | null;
  const logId = record?.log_id || response.headers?.get("X-Log-Id") || "";
  const businessCode = Number(record?.code);
  const code = Number.isFinite(businessCode) ? businessCode : undefined;

  if (!response.ok) {
    throw new ApiError(
      extractMessage(record) || `请求失败 (${response.status})`,
      { status: response.status, code, logId }
    );
  }

  const normalized = normalizeResponse(payload);
  if (!normalized.ok) {
    throw new ApiError(normalized.reason, {
      status: response.status,
      code,
      logId,
    });
  }
  if (normalized.resp.code !== 0) {
    // Legitimate service error envelope over HTTP 2xx: the business code is
    // the envelope code (no status attached, 2xx would shadow it).
    throw new ApiError(normalized.resp.msg, { code: normalized.resp.code, logId });
  }
  if (!hasUsableClassroomData(normalized.resp)) {
    // Success envelope with cross-day/expired/malformed cache metadata fails
    // closed as a client failure (code 500) so the retry ladder, not the 1s
    // poll floor, paces the next attempt (Nginx 30 req/min budget).
    throw new ApiError(extractMessage(record) || fallbackErrorMessage, {
      logId,
    });
  }
  return normalized.resp;
}

/**
 * refreshInterval for useSWR. Module-level stable identity is required: the
 * polling effect only re-arms via its own `execute → then(next)` chain, and
 * SWR terminates that chain permanently on a falsy interval — while
 * `nextReloadDelay(undefined)` (pre-first-data) is null. Hence the never-falsy
 * wrapper. SWR passes the cached fetcher value (the envelope), the schedule
 * pipeline wants the inner snapshot.
 */
export function pollingInterval(latest?: EnvelopeView | null): number {
  const delay = nextReloadDelay(latest?.data, { failureCount: 0 });
  return Math.max(1, delay ?? MIN_FRESH_DELAY_MS);
}

export interface RetryScheduleOptions {
  nowMs?: number;
  random?: (() => number) | null;
}

/**
 * Failure backoff for onErrorRetry. SWR's retryCount starts at 1, aligning
 * 1:1 with the 10/20/30/60s ladder. Never use failureRetryDelay directly:
 * nextReloadDelay also clamps the ladder to a still-displayable snapshot's
 * stale_until hard deadline (contract: wake at stale_until even mid-backoff).
 */
export function retryDelayFor(
  retryCount: number,
  latestData: unknown,
  options: RetryScheduleOptions = {}
): number | null | undefined {
  return nextReloadDelay(latestData, { ...options, failureCount: retryCount });
}

interface RetryCacheEntry {
  data?: { data?: unknown };
}

interface RetryConfigView {
  cache?: { get(key: string): RetryCacheEntry | undefined };
}

type RevalidateFn = (opts?: object) => void | Promise<unknown>;

/**
 * Custom onErrorRetry: ladder + stale_until clamp via retryDelayFor, and a
 * visibility self-check inside the timer callback — SWR fires already-armed
 * retry timers even after the tab hides (no isActive gate on
 * ERROR_REVALIDATE_EVENT), so give up when hidden; revalidateOnFocus reissues
 * the request when the tab returns. New retries are not chained while hidden
 * (SWR's own isActive precondition), which pauses the ladder as the old
 * scheduler did.
 */
export function retryOnError(
  _error: unknown,
  key: string,
  config: unknown,
  revalidate?: RevalidateFn,
  opts?: { retryCount: number }
): void {
  const cached = (config as RetryConfigView | undefined)?.cache?.get?.(key);
  const delay = retryDelayFor(opts!.retryCount, cached?.data?.data);
  if (delay == null || !Number.isFinite(delay)) {
    return;
  }
  setTimeout(() => {
    if (
      typeof document !== "undefined" &&
      document.visibilityState === "hidden"
    ) {
      return;
    }
    revalidate!(opts);
  }, delay);
}

export default function useTodayClassrooms() {
  // Only survivor of the old 6-useState machine: marks a user-initiated
  // reload so the spin policy can tell it apart from background revalidation.
  const [manualReload, setManualReload] = useState(false);

  const { data, error, isValidating, mutate } = useSWR<
    Envelope,
    unknown,
    string
  >(
    TODAY_CLASSROOMS_KEY,
    fetchTodayClassrooms,
    {
      refreshInterval: pollingInterval,
      onErrorRetry: retryOnError,
      // Red line: keep true. Besides "revalidate promptly after stale_until
      // when becoming visible", SWR only pauses the error-retry chain on
      // hidden tabs while revalidateOnFocus is on.
      revalidateOnFocus: true,
      focusThrottleInterval: FOCUS_THROTTLE_MS,
      // Everything else stays default: refreshWhenHidden false (hidden tabs
      // never fetch), shouldRetryOnError true, dedupingInterval 2s,
      // errorRetryCount unset (the ladder itself caps at 60s).
    }
  );

  const retry = useCallback(() => {
    setManualReload(true);
    Promise.resolve(mutate())
      .catch(() => {
        // Failures surface through the hook's error track.
      })
      .finally(() => setManualReload(false));
  }, [mutate]);

  // Render-time derivation replaces the old setState-time merging. SWR keeps
  // data and error on separate tracks (a throwing fetcher never clears data),
  // so mergeFetchResult sees the same (prev, next) pairs as before.
  const nowMs = Date.now();
  const hasUsable = hasUsableClassroomData(data, nowMs);
  const neverResolved = data === undefined && error === undefined;
  const isBackground = !neverResolved && !manualReload;
  const spinning = isValidating && shouldFullPageSpin(isBackground, hasUsable);

  let resp: HookEnvelope;
  if (spinning || neverResolved) {
    // Full-page spin implies no usable data: reset to the loading envelope,
    // exactly like the old hook did before a foreground request.
    resp = loadingResponse;
  } else if (error) {
    const failed = errorAsEnvelope(error);
    const merged = mergeFetchResult(data ?? null, failed, nowMs) as HookEnvelope;
    // mergeFetchResult rebuilds hard-empty envelopes without logId; re-attach
    // it so the error UI can surface it (stale-snapshot merges keep code 0
    // and never show the hard error card).
    resp =
      merged.code !== 0 && failed.logId
        ? { ...merged, logId: failed.logId }
        : merged;
  } else if (hasUsable) {
    resp = data as Envelope;
  } else {
    // code-0 cache crossed midnight or stale_until between revalidations:
    // clear the campuses while the clamped reload is in flight.
    resp = errorEnvelope(EXPIRED_SNAPSHOT_MESSAGE);
  }

  return {
    resp,
    spinning,
    reloading: isValidating && !spinning,
    isError: resp.code !== 0 && !spinning,
    retry,
  };
}
