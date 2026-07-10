import { isUsableBusinessDaySnapshot } from "./classroomDataValidity";

// Rate-aware schedule tuned for Nginx 30 req/min /api limit with multi-tab users.
export const STALE_POLL_MS = 15_000;
export const PARTIAL_POLL_MS = 30_000;
export const MIN_FRESH_DELAY_MS = 1_000;
export const FAILURE_RETRY_DELAYS_MS = [10_000, 20_000, 30_000, 60_000];
export const JITTER_RATIO = 0.2;

export function failureRetryDelay(failureCount) {
  const parsed = Number(failureCount);
  const count = Number.isFinite(parsed) ? Math.max(1, Math.floor(parsed)) : 1;
  return FAILURE_RETRY_DELAYS_MS[
    Math.min(count - 1, FAILURE_RETRY_DELAYS_MS.length - 1)
  ];
}

/**
 * Apply bounded symmetric jitter for auto-reload spacing.
 * @param {number|null} delayMs
 * @param {() => number} [random]
 */
export function withJitter(delayMs, random = Math.random) {
  if (delayMs == null || !Number.isFinite(delayMs) || delayMs <= 0) {
    return delayMs;
  }
  const spread = delayMs * JITTER_RATIO;
  const sample =
    typeof random === "function" && Number.isFinite(random())
      ? Math.min(1, Math.max(0, random()))
      : 0.5;
  return Math.max(0, Math.round(delayMs - spread / 2 + sample * spread));
}

/**
 * Delay until the next automatic /api/get_data reload.
 * Transport failures use bounded backoff. Partial payloads follow the backend's
 * refresh backoff, while stale payloads poll without thrashing shared rate limits.
 */
export function nextReloadDelay(
  data,
  { failureCount = 0, nowMs = Date.now(), random = Math.random } = {}
) {
  let base = null;

  if (failureCount > 0) {
    const retryDelay = failureRetryDelay(failureCount);
    if (!isUsableBusinessDaySnapshot(data, nowMs)) {
      base = retryDelay;
    } else {
      const staleUntil = Date.parse(data.stale_until);
      base = Math.min(retryDelay, staleUntil - nowMs);
    }
  } else if (data == null) {
    base = null;
  } else if (!isUsableBusinessDaySnapshot(data, nowMs)) {
    // Cross-day, expired, or malformed snapshots must be revalidated promptly.
    base = MIN_FRESH_DELAY_MS;
  } else {
    const staleUntilDelay = Date.parse(data.stale_until) - nowMs;
    const partialCampuses = Array.isArray(data.partial_campuses)
      ? data.partial_campuses
      : [];
    if (
      partialCampuses.length > 0 ||
      (data.error && !data.stale && data.error.type !== "client_refresh_failed")
    ) {
      base = Math.min(PARTIAL_POLL_MS, staleUntilDelay);
    } else if (data.stale || data.error) {
      base = Math.min(STALE_POLL_MS, staleUntilDelay);
    } else {
      const expiresAt = Date.parse(data.expires_at);
      if (!Number.isFinite(expiresAt)) {
        base = STALE_POLL_MS;
      } else {
        base = Math.min(
          Math.max(expiresAt - nowMs, MIN_FRESH_DELAY_MS),
          staleUntilDelay
        );
      }
    }
  }

  return withJitter(base, random);
}
