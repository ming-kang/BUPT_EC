/**
 * Fetch boundary for `GET /api/get_data`: timeout composition (Safari 14 has
 * neither AbortSignal.timeout nor AbortSignal.any), wire-level log_id /
 * version extraction for failure envelopes, and the decode + usability gate.
 * Every failure mode exits as a typed `ApiError` with a user-safe message.
 */
import {
  ApiError,
  decodeGetData,
  fallbackErrorMessage,
  isUsableBusinessDaySnapshot,
  readJson,
  type Envelope,
} from "./decode";

const TODAY_CLASSROOMS_URL = "/api/get_data";

/**
 * Client fetch budget: below the Nginx /api proxy_read_timeout (60s). The
 * Go server itself answers well inside its 15s HTTP WriteTimeout (cold-cache
 * requests wait at most 5s for the background refresh), so a stall this long
 * is proxy-level and gets the safe timeout message.
 */
const CLIENT_FETCH_TIMEOUT_MS = 40_000;
const CLIENT_FETCH_TIMEOUT_MESSAGE = "请求超时，请稍后重试";

/**
 * Loose pre-decode reads for failure envelopes (decode itself may have
 * thrown before producing an envelope). Values stay unknown until checked.
 */
function wireRecord(payload: unknown): Record<string, unknown> | null {
  if (
    typeof payload !== "object" ||
    payload === null ||
    Array.isArray(payload)
  ) {
    return null;
  }
  return payload as Record<string, unknown>;
}

function wireString(
  record: Record<string, unknown> | null,
  key: string
): string {
  const value = record?.[key];
  return typeof value === "string" && value.trim() !== ""
    ? value.trim()
    : "";
}

function wireCode(record: Record<string, unknown> | null): number | undefined {
  const value = record?.code;
  return typeof value === "number" && Number.isFinite(value)
    ? value
    : undefined;
}

/**
 * fetch() with the timeout enforced through AbortController + setTimeout.
 * The parent signal (unmount abort) forwards into the child controller.
 */
async function fetchWithTimeout(
  url: string,
  parent: AbortSignal
): Promise<Response> {
  const controller = new AbortController();
  let timedOut = false;
  const timer = setTimeout(() => {
    timedOut = true;
    controller.abort();
  }, CLIENT_FETCH_TIMEOUT_MS);
  const onParentAbort = () => controller.abort();
  if (parent.aborted) {
    controller.abort();
  } else {
    parent.addEventListener("abort", onParentAbort, { once: true });
  }
  try {
    return await fetch(url, {
      headers: { Accept: "application/json" },
      signal: controller.signal,
    });
  } catch (error) {
    // A parent abort (unmount) rethrows untouched: the hook's disposed guard
    // discards it. Only the timeout converts into an ApiError here.
    if (timedOut) {
      throw new ApiError(CLIENT_FETCH_TIMEOUT_MESSAGE);
    }
    throw error;
  } finally {
    clearTimeout(timer);
    parent.removeEventListener("abort", onParentAbort);
  }
}

/**
 * Fetch + decode one `/api/get_data` response. The single error channel is a
 * thrown ApiError: transport failure, timeout, HTTP error envelope, service
 * error envelope over HTTP 2xx, malformed payload, or a success envelope
 * whose snapshot metadata is no longer a usable business-day snapshot (fails
 * closed as a client failure so the retry ladder, not the 1s poll floor,
 * paces the next attempt).
 */
export async function requestTodayClassrooms(
  signal: AbortSignal
): Promise<Envelope> {
  const response = await fetchWithTimeout(TODAY_CLASSROOMS_URL, signal);

  const payload = await readJson(response);
  const record = wireRecord(payload);
  // Request id: body log_id wins, else the X-Log-Id header. Rides along on
  // every throw so the error card can name the server-side log.
  const logId =
    wireString(record, "log_id") || response.headers?.get("X-Log-Id") || "";
  // Same rule as decodeGetData: absent rather than empty, so envelope shape
  // is unchanged when no version is known.
  const version = wireString(record, "version");

  if (!response.ok) {
    throw new ApiError(
      wireString(record, "msg") || `请求失败 (${response.status})`,
      { status: response.status, code: wireCode(record), logId, version }
    );
  }

  let envelope: Envelope;
  try {
    envelope = decodeGetData(payload);
  } catch (error) {
    if (error instanceof ApiError) {
      // Malformed payload under HTTP 2xx: fail closed as a client failure
      // (a status here would shadow the business code the UI never got).
      throw new ApiError(error.message, { status: response.status, logId, version });
    }
    throw error;
  }
  if (envelope.code !== 0) {
    // Legitimate service error envelope over HTTP 2xx: the business code is
    // the envelope code (no status attached — 2xx would shadow it).
    throw new ApiError(envelope.msg, {
      code: envelope.code,
      logId: envelope.logId || logId,
      version: envelope.version,
    });
  }
  if (!isUsableBusinessDaySnapshot(envelope.data)) {
    throw new ApiError(envelope.msg || fallbackErrorMessage, {
      logId: envelope.logId || logId,
      version,
    });
  }
  return {
    ...envelope,
    logId: envelope.logId || logId,
  };
}
