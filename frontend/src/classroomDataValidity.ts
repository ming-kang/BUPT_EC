import { formatShanghaiDate } from "./classTimeUtils";

interface SnapshotView {
  campuses?: unknown;
  date?: unknown;
  stale_until?: unknown;
}

/**
 * True when a classroom snapshot is still safe to display for the current
 * Asia/Shanghai business day.
 */
export function isUsableBusinessDaySnapshot(
  data: unknown,
  nowMs: number = Date.now()
): boolean {
  // The snapshot arrives from the network unvalidated; every field access is
  // guarded below (types describe trust, this function enforces it).
  const view = data as SnapshotView | null | undefined;

  if (
    !Number.isFinite(nowMs) ||
    !view ||
    typeof view !== "object" ||
    !Array.isArray(view.campuses)
  ) {
    return false;
  }

  if (
    typeof view.date !== "string" ||
    view.date !== formatShanghaiDate(new Date(nowMs))
  ) {
    return false;
  }

  if (
    typeof view.stale_until !== "string" ||
    view.stale_until.trim() === ""
  ) {
    return false;
  }
  const staleUntil = Date.parse(view.stale_until);
  return Number.isFinite(staleUntil) && nowMs < staleUntil;
}
