import type { TodayClassroomsData } from "./api/types";
import { isUsableBusinessDaySnapshot } from "./api/decode";

// Rate-aware schedule tuned for the Nginx 30 req/min /api limit with
// multi-tab users.
const STALE_POLL_MS = 15_000;
const PARTIAL_POLL_MS = 30_000;
const MIN_FRESH_DELAY_MS = 1_000;
const FAILURE_RETRY_DELAYS_MS = [10_000, 20_000, 30_000, 60_000];
const JITTER_RATIO = 0.1;
const JITTER_MAX_MS = 5_000;

/**
 * Delay until the next automatic /api/get_data reload, or null when nothing
 * should be scheduled yet (no snapshot and no failures — the initial request
 * is the mount effect's job).
 *
 * Pipeline: base delay by state → one-sample bounded positive jitter →
 * stale_until clamp (the business deadline wins over rate floors), so the
 * timer always wakes at stale_until even mid-backoff.
 */
export function nextReloadDelay(
  data: TodayClassroomsData | null,
  {
    failureCount = 0,
    nowMs = Date.now(),
    random = Math.random,
  }: { failureCount?: number; nowMs?: number; random?: () => number } = {},
): number | null {
  const usable = isUsableBusinessDaySnapshot(data, nowMs);
  let base: number;
  if (failureCount > 0) {
    base =
      FAILURE_RETRY_DELAYS_MS[
        Math.min(failureCount, FAILURE_RETRY_DELAYS_MS.length) - 1
      ];
  } else if (data === null) {
    return null;
  } else if (!usable) {
    base = MIN_FRESH_DELAY_MS;
  } else if (
    (data.partial_campuses?.length ?? 0) > 0 ||
    (data.error && !data.stale)
  ) {
    base = PARTIAL_POLL_MS;
  } else if (data.stale || data.error) {
    base = STALE_POLL_MS;
  } else {
    base =
      Math.max(Date.parse(data.expires_at) - nowMs, MIN_FRESH_DELAY_MS) ||
      STALE_POLL_MS;
  }

  const jittered = Math.round(
    base + random() * Math.min(base * JITTER_RATIO, JITTER_MAX_MS),
  );
  if (!usable || data === null) {
    return jittered;
  }
  return Math.min(jittered, Math.max(0, Date.parse(data.stale_until) - nowMs));
}
