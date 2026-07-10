import { isUsableBusinessDaySnapshot } from "./classroomDataValidity";

export const STALE_POLL_MS = 5_000;
export const PARTIAL_POLL_MS = 30_000;
export const MIN_FRESH_DELAY_MS = 1_000;
export const FAILURE_RETRY_DELAYS_MS = [5_000, 10_000, 20_000, 30_000];

export function failureRetryDelay(failureCount) {
  const parsed = Number(failureCount);
  const count = Number.isFinite(parsed) ? Math.max(1, Math.floor(parsed)) : 1;
  return FAILURE_RETRY_DELAYS_MS[
    Math.min(count - 1, FAILURE_RETRY_DELAYS_MS.length - 1)
  ];
}

/**
 * Delay until the next automatic /api/get_data reload.
 * Transport failures use bounded backoff. Partial payloads follow the backend's
 * refresh backoff, while stale payloads can poll briefly for an in-flight result.
 */
export function nextReloadDelay(
  data,
  { failureCount = 0, nowMs = Date.now() } = {}
) {
  if (failureCount > 0) {
    const retryDelay = failureRetryDelay(failureCount);
    if (!isUsableBusinessDaySnapshot(data, nowMs)) {
      return retryDelay;
    }
    const staleUntil = Date.parse(data.stale_until);
    return Math.min(retryDelay, staleUntil - nowMs);
  }

  if (data == null) {
    return null;
  }

  // Cross-day, expired, or malformed snapshots must be revalidated promptly.
  if (!isUsableBusinessDaySnapshot(data, nowMs)) {
    return MIN_FRESH_DELAY_MS;
  }
  const staleUntilDelay = Date.parse(data.stale_until) - nowMs;

  const partialCampuses = Array.isArray(data.partial_campuses)
    ? data.partial_campuses
    : [];
  if (
    partialCampuses.length > 0 ||
    (data.error && !data.stale && data.error.type !== "client_refresh_failed")
  ) {
    return Math.min(PARTIAL_POLL_MS, staleUntilDelay);
  }

  if (data.stale || data.error) {
    return Math.min(STALE_POLL_MS, staleUntilDelay);
  }

  const expiresAt = Date.parse(data.expires_at);
  if (!Number.isFinite(expiresAt)) {
    return STALE_POLL_MS;
  }
  return Math.min(
    Math.max(expiresAt - nowMs, MIN_FRESH_DELAY_MS),
    staleUntilDelay
  );
}
