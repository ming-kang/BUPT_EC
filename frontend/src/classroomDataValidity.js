import { formatShanghaiDate } from "./classTimeUtils";

/**
 * True when a classroom snapshot is still safe to display for the current
 * Asia/Shanghai business day.
 */
export function isUsableBusinessDaySnapshot(data, nowMs = Date.now()) {
  if (
    !Number.isFinite(nowMs) ||
    !data ||
    typeof data !== "object" ||
    !Array.isArray(data.campuses)
  ) {
    return false;
  }

  if (
    typeof data.date !== "string" ||
    data.date !== formatShanghaiDate(new Date(nowMs))
  ) {
    return false;
  }

  if (
    typeof data.stale_until !== "string" ||
    data.stale_until.trim() === ""
  ) {
    return false;
  }
  const staleUntil = Date.parse(data.stale_until);
  return Number.isFinite(staleUntil) && nowMs < staleUntil;
}
