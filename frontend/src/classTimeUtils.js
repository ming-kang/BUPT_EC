const SHANGHAI_TZ = "Asia/Shanghai";

/** YYYY-MM-DD in Asia/Shanghai (matches backend business day). */
export function formatShanghaiDate(date = new Date()) {
  return new Intl.DateTimeFormat("en-CA", {
    timeZone: SHANGHAI_TZ,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(date);
}

/** HH:mm in Asia/Shanghai for comparing against JW node end times. */
export function formatShanghaiTime(date = new Date()) {
  return new Intl.DateTimeFormat("en-GB", {
    timeZone: SHANGHAI_TZ,
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(date);
}

export function parseNodeEndTime(timeRange) {
  const [, endTime = ""] = String(timeRange || "").split("-");
  return endTime.trim();
}

export function isNodeEnded(timeRange, nowTime) {
  const endTime = parseNodeEndTime(timeRange);
  if (!endTime) {
    return false;
  }
  return endTime.localeCompare(nowTime) < 0;
}

/**
 * Drop selected nodes that have ended (when all-day selection is off and date is today).
 */
export function pruneEndedClassTimes(
  selectedClassTimes,
  nodes,
  { nowTime, isToday, canSelectAllDay }
) {
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
