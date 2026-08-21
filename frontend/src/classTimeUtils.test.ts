import { describe, expect, it } from "vitest";
import {
  isNodeEnded,
  pruneEndedClassTimes,
} from "./classTimeUtils";

describe("classTimeUtils", () => {
  it("does not treat empty end times as ended", () => {
    expect(isNodeEnded("", "12:00")).toBe(false);
    expect(isNodeEnded("bad", "12:00")).toBe(false);
  });

  it("detects ended nodes by end time", () => {
    expect(isNodeEnded("08:00-08:45", "09:00")).toBe(true);
    expect(isNodeEnded("08:00-08:45", "08:00")).toBe(false);
    expect(isNodeEnded("08:00-08:45", "08:45")).toBe(false);
  });

  it("prunes ended selections when not allowing all-day", () => {
    const nodes = [
      { node: 1, time: "08:00-08:45" },
      { node: 2, time: "09:00-09:45" },
      { node: 3, time: "10:00-10:45" },
    ];
    expect(
      pruneEndedClassTimes([1, 2, 3], nodes, {
        nowTime: "09:30",
        isToday: true,
        canSelectAllDay: false,
      })
    ).toEqual([2, 3]);
  });

  it("keeps ended selections when canSelectAllDay or not today", () => {
    const nodes = [{ node: 1, time: "08:00-08:45" }];
    expect(
      pruneEndedClassTimes([1], nodes, {
        nowTime: "12:00",
        isToday: true,
        canSelectAllDay: true,
      })
    ).toEqual([1]);
    expect(
      pruneEndedClassTimes([1], nodes, {
        nowTime: "12:00",
        isToday: false,
        canSelectAllDay: false,
      })
    ).toEqual([1]);
  });
});
