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

export function isNodeEnded(timeRange: string, nowTime: string): boolean {
  const endTime = (timeRange.split("-")[1] ?? "").trim();
  return endTime !== "" && endTime.localeCompare(nowTime) < 0;
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
  selectedClassTimes: number[],
  nodes: ClassTimeOption[],
  { nowTime, isToday, canSelectAllDay }: PruneOptions
): number[] {
  if (canSelectAllDay || !isToday || selectedClassTimes.length === 0) {
    return selectedClassTimes;
  }
  const ended = new Set(
    nodes
      .filter((item) => isNodeEnded(item.time, nowTime))
      .map((item) => item.node)
  );
  if (ended.size === 0) {
    return selectedClassTimes;
  }
  return selectedClassTimes.filter((node) => !ended.has(node));
}
