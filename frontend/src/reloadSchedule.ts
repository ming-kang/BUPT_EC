import { isUsableBusinessDaySnapshot } from "./api/decode";

// Rate-aware schedule tuned for the Nginx 30 req/min /api limit with
// multi-tab users.
const STALE_POLL_MS = 15_000;
const PARTIAL_POLL_MS = 30_000;
const MIN_FRESH_DELAY_MS = 1_000;
const FAILURE_RETRY_DELAYS_MS = [10_000, 20_000, 30_000, 60_000];
/** Maximum positive relative jitter applied to a base delay. */
const JITTER_RATIO = 0.1;
/** Absolute cap on positive jitter so long fresh delays do not drift too far. */
const JITTER_MAX_MS = 5_000;

function recordView(data: unknown): Record<string, unknown> | null {
  if (typeof data !== "object" || data === null || Array.isArray(data)) {
    return null;
  }
  return data as Record<string, unknown>;
}

/**
 * Delay until the next automatic /api/get_data reload, or null when nothing
 * should be scheduled yet (no snapshot and no failures — the initial request
 * is the mount effect's job).
 *
 * Pipeline: base delay by state → one-sample bounded positive jitter →
 * absolute stale_until clamp (the business deadline wins over rate floors).
 * A still-displayable snapshot's hard deadline clamps failure backoff too,
 * so the timer always wakes at stale_until even mid-backoff.
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
): number | null {
  let base: number;

  if (failureCount > 0) {
    const parsed = Number(failureCount);
    const count = Number.isFinite(parsed) ? Math.max(1, Math.floor(parsed)) : 1;
    const rung = Math.min(count, FAILURE_RETRY_DELAYS_MS.length) - 1;
    base = FAILURE_RETRY_DELAYS_MS[rung];
  } else if (data == null) {
    return null;
  } else if (!isUsableBusinessDaySnapshot(data, nowMs)) {
    // Cross-day, expired, or malformed snapshots must be revalidated promptly.
    base = MIN_FRESH_DELAY_MS;
  } else {
    const fields = recordView(data) ?? {};
    const partialCampuses = Array.isArray(fields.partial_campuses)
      ? fields.partial_campuses
      : [];
    const error = recordView(fields.error);
    const errorType = typeof error?.type === "string" ? error.type : "";
    if (
      partialCampuses.length > 0 ||
      (error && !fields.stale && errorType !== "client_refresh_failed")
    ) {
      base = PARTIAL_POLL_MS;
    } else if (fields.stale || error) {
      base = STALE_POLL_MS;
    } else {
      const expiresAt = Date.parse(
        typeof fields.expires_at === "string" ? fields.expires_at : ""
      );
      base = Number.isFinite(expiresAt)
        ? Math.max(expiresAt - nowMs, MIN_FRESH_DELAY_MS)
        : STALE_POLL_MS;
    }
  }

  // One bounded positive jitter sample desyncs multi-tab reloads without
  // ever shortening the documented minimum intervals.
  let sample = typeof random === "function" ? random() : 0.5;
  if (typeof sample !== "number" || !Number.isFinite(sample)) {
    sample = 0.5;
  }
  sample = Math.min(Math.max(sample, 0), 1);
  const jittered = Math.round(
    base + sample * Math.min(base * JITTER_RATIO, JITTER_MAX_MS)
  );

  if (!isUsableBusinessDaySnapshot(data, nowMs)) {
    return jittered;
  }
  const staleUntilRaw = (recordView(data) ?? {}).stale_until;
  const staleUntil = Date.parse(
    typeof staleUntilRaw === "string" ? staleUntilRaw : ""
  );
  // Clamp after jitter so a sample of 1 can never schedule past stale_until.
  // When the deadline is earlier than a rate-limit floor, it wins.
  return Math.min(jittered, Math.max(0, staleUntil - nowMs));
}
