import { formatShanghaiDate } from "./classTimeUtils";

export const STALE_POLL_MS = 5_000;
export const MIN_FRESH_DELAY_MS = 1_000;

/**
 * Delay until the next automatic /api/get_data reload.
 * Stale or partial-error payloads poll quickly so background JW recovery shows up soon.
 * Cross-day payloads (or past stale_until) reload ASAP so yesterday's "fresh" cache
 * is not held until expires_at after Shanghai midnight.
 */
export function nextReloadDelay(data, nowMs = Date.now()) {
  if (!data?.expires_at) {
    return null;
  }
  const expiresAt = new Date(data.expires_at).getTime();
  if (!Number.isFinite(expiresAt)) {
    return null;
  }

  // Payload is for a previous Shanghai business day — do not wait on expires_at.
  if (data.date) {
    const today = formatShanghaiDate(new Date(nowMs));
    if (data.date !== today) {
      return MIN_FRESH_DELAY_MS;
    }
  }

  // Past end-of-day stale window — cache is no longer usable same-day.
  if (data.stale_until) {
    const staleUntil = new Date(data.stale_until).getTime();
    if (Number.isFinite(staleUntil) && nowMs >= staleUntil) {
      return MIN_FRESH_DELAY_MS;
    }
  }

  if (data.stale || data.error) {
    return STALE_POLL_MS;
  }
  return Math.max(expiresAt - nowMs, MIN_FRESH_DELAY_MS);
}
