const SHANGHAI_TZ = "Asia/Shanghai";

/** YYYY-MM-DD in Asia/Shanghai (matches backend business day). */
export function formatShanghaiDate(date: Date = new Date()): string {
  return new Intl.DateTimeFormat("en-CA", {
    timeZone: SHANGHAI_TZ,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(date);
}

/** HH:mm in Asia/Shanghai for comparing against JW node end times. */
export function formatShanghaiTime(date: Date = new Date()): string {
  return new Intl.DateTimeFormat("en-GB", {
    timeZone: SHANGHAI_TZ,
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(date);
}

/** YYYY-MM-DD HH:mm in Asia/Shanghai for user-facing timestamps (F-06). */
export function formatShanghaiDateTime(date: Date = new Date()): string {
  return new Intl.DateTimeFormat("sv-SE", {
    timeZone: SHANGHAI_TZ,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(date);
}

export function parseNodeEndTime(timeRange: unknown): string {
  const [, endTime = ""] = String(timeRange || "").split("-");
  return endTime.trim();
}

export function isNodeEnded(timeRange: unknown, nowTime: string): boolean {
  const endTime = parseNodeEndTime(timeRange);
  if (!endTime) {
    return false;
  }
  return endTime.localeCompare(nowTime) < 0;
}

interface ClassTimeOption {
  node: number;
  time: string;
}

interface PruneOptions {
  nowTime: string;
  isToday: boolean;
  canSelectAllDay: boolean;
}

/**
 * Drop selected nodes that have ended (when all-day selection is off and date is today).
 */
export function pruneEndedClassTimes(
  selectedClassTimes: number[] | null | undefined,
  nodes: ClassTimeOption[] | null | undefined,
  { nowTime, isToday, canSelectAllDay }: PruneOptions
): number[] {
  const selected = Array.isArray(selectedClassTimes) ? selectedClassTimes : [];
  if (canSelectAllDay || !isToday || selected.length === 0) {
    return selected;
  }
  const options = Array.isArray(nodes) ? nodes : [];
  const ended = new Set(
    options
      .filter((item) => isNodeEnded(item.time, nowTime))
      .map((item) => item.node)
  );
  if (ended.size === 0) {
    return selected;
  }
  return selected.filter((node) => !ended.has(node));
}
