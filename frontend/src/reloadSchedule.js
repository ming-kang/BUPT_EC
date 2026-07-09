export const STALE_POLL_MS = 5_000;
export const MIN_FRESH_DELAY_MS = 1_000;

/**
 * Delay until the next automatic /api/get_data reload.
 * Stale or partial-error payloads poll quickly so background JW recovery shows up soon.
 */
export function nextReloadDelay(data, nowMs = Date.now()) {
  if (!data?.expires_at) {
    return null;
  }
  const expiresAt = new Date(data.expires_at).getTime();
  if (!Number.isFinite(expiresAt)) {
    return null;
  }
  if (data.stale || data.error) {
    return STALE_POLL_MS;
  }
  return Math.max(expiresAt - nowMs, MIN_FRESH_DELAY_MS);
}
