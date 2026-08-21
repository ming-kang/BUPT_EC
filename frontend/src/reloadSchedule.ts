import { isUsableBusinessDaySnapshot } from "./classroomDataValidity";
import type { TodayClassroomsData } from "./api/types";

// Rate-aware schedule tuned for Nginx 30 req/min /api limit with multi-tab users.
export const STALE_POLL_MS = 15_000;
export const PARTIAL_POLL_MS = 30_000;
export const MIN_FRESH_DELAY_MS = 1_000;
export const FAILURE_RETRY_DELAYS_MS = [10_000, 20_000, 30_000, 60_000];
/** Maximum positive relative jitter applied to a base delay. */
export const JITTER_RATIO = 0.1;
/** Absolute cap on positive jitter so long fresh delays do not drift too far. */
export const JITTER_MAX_MS = 5_000;

/** Structural view of the fields this module reads off a payload. */
type SnapshotFields = Partial<TodayClassroomsData>;

export function failureRetryDelay(failureCount: unknown): number {
  const parsed = Number(failureCount);
  const count = Number.isFinite(parsed) ? Math.max(1, Math.floor(parsed)) : 1;
  return FAILURE_RETRY_DELAYS_MS[
    Math.min(count - 1, FAILURE_RETRY_DELAYS_MS.length - 1)
  ];
}

/**
 * Normalize one random unit sample. Invalid / missing / throwing sources fall
 * back to 0.5 so callers never see NaN delays.
 */
export function normalizeRandomSample(
  random: (() => number) | null = Math.random
): number {
  let sample;
  try {
    sample = typeof random === "function" ? random() : 0.5;
  } catch {
    return 0.5;
  }
  if (!Number.isFinite(sample)) {
    return 0.5;
  }
  if (sample < 0) {
    return 0;
  }
  if (sample > 1) {
    return 1;
  }
  return sample;
}

/**
 * Apply bounded **positive** jitter so multi-tab reloads desync without
 * shortening the documented minimum intervals.
 * Reads the random source at most once per call.
 */
export function withJitter(
  delayMs: number | null | undefined,
  random?: (() => number) | null
): number | null | undefined {
  if (delayMs == null || !Number.isFinite(delayMs) || delayMs <= 0) {
    return delayMs;
  }
  const sample = normalizeRandomSample(random);
  const spread = Math.min(delayMs * JITTER_RATIO, JITTER_MAX_MS);
  return Math.round(delayMs + sample * spread);
}

/**
 * Hard display deadline for a still-usable same-day snapshot, or null when the
 * payload is not displayable (no clamp applies).
 */
function usableHardDeadlineMs(data: unknown, nowMs: number): number | null {
  if (!isUsableBusinessDaySnapshot(data, nowMs)) {
    return null;
  }
  const staleUntil = Date.parse((data as SnapshotFields).stale_until ?? "");
  return Math.max(0, staleUntil - nowMs);
}

/**
 * Delay until the next automatic /api/get_data reload.
 *
 * Pipeline: base delay by state → one-sample positive jitter → absolute
 * stale_until clamp (business deadline wins over rate-limit floors).
 */
export function nextReloadDelay(
  data: unknown,
  {
    failureCount = 0,
    nowMs = Date.now(),
    random = Math.random,
  }: {
    failureCount?: number;
    nowMs?: number;
    random?: (() => number) | null;
  } = {}
): number | null | undefined {
  let base: number | null = null;

  if (failureCount > 0) {
    // Failure backoff always starts from the ladder; hard deadline clamps later
    // when a same-day snapshot is still displayable.
    base = failureRetryDelay(failureCount);
  } else if (data == null) {
    base = null;
  } else if (!isUsableBusinessDaySnapshot(data, nowMs)) {
    // Cross-day, expired, or malformed snapshots must be revalidated promptly.
    base = MIN_FRESH_DELAY_MS;
  } else {
    const snapshot = data as SnapshotFields;
    const partialCampuses = Array.isArray(snapshot.partial_campuses)
      ? snapshot.partial_campuses
      : [];
    if (
      partialCampuses.length > 0 ||
      (snapshot.error && !snapshot.stale && snapshot.error.type !== "client_refresh_failed")
    ) {
      base = PARTIAL_POLL_MS;
    } else if (snapshot.stale || snapshot.error) {
      base = STALE_POLL_MS;
    } else {
      const expiresAt = Date.parse(snapshot.expires_at ?? "");
      if (!Number.isFinite(expiresAt)) {
        base = STALE_POLL_MS;
      } else {
        base = Math.max(expiresAt - nowMs, MIN_FRESH_DELAY_MS);
      }
    }
  }

  const jittered = withJitter(base, random);
  if (jittered == null || !Number.isFinite(jittered)) {
    return jittered;
  }

  const hardDelay = usableHardDeadlineMs(data, nowMs);
  if (hardDelay == null) {
    return jittered;
  }
  // Clamp after jitter so random=1 cannot schedule past stale_until.
  // When hardDelay is earlier than the rate-limit floor, the business deadline wins.
  return Math.min(jittered, hardDelay);
}
