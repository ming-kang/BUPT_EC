/**
 * @vitest-environment jsdom
 *
 * ClassTimePicker render-time pruning (R10) and clock resync on
 * visibilitychange (R11). The selection store is mocked through
 * SelectionContext so state is fully controlled and the convergence
 * dispatch can be asserted without reducer feedback.
 */
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SelectionContext } from "../selectionContext";
import ClassTimePicker from "./ClassTimePicker";

// 2026-07-27 10:00 Asia/Shanghai.
const BASE_NOW = new Date("2026-07-27T02:00:00.000Z");
// 2026-07-27 18:00 Asia/Shanghai (same business day; node 2 has ended).
const LATER_SAME_DAY = new Date("2026-07-27T10:00:00.000Z");
const TODAY = "2026-07-27";

const CAMPUS = {
  id: "04",
  name: "沙河",
  nodes: [
    { node: 1, time: "08:00-08:45" }, // already ended at BASE_NOW
    { node: 2, time: "15:00-15:45" }, // ends between BASE_NOW and LATER_SAME_DAY
    { node: 3, time: "19:00-19:45" }, // never ends in these tests
  ],
};

function makeState(overrides = {}) {
  return {
    selectedCampus: "04",
    selectedBuildings: [],
    selectedClassTimes: [],
    showClassTime: false,
    canSelectAllDay: false,
    ...overrides,
  };
}

function renderPicker({ state = makeState(), dispatch = vi.fn() } = {}) {
  const view = render(
    <SelectionContext.Provider value={{ state, dispatch }}>
      <ClassTimePicker selectedCampusData={CAMPUS} todayDate={TODAY} />
    </SelectionContext.Provider>
  );
  return { ...view, dispatch };
}

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("ClassTimePicker derived pruning (R10)", () => {
  it("renders ended selections as unselected on the first frame", () => {
    vi.useFakeTimers();
    vi.setSystemTime(BASE_NOW);
    const { dispatch } = renderPicker({
      state: makeState({ selectedClassTimes: [1, 2] }),
    });

    const ended = screen.getByRole("button", { name: "01" });
    const active = screen.getByRole("button", { name: "02" });
    expect(ended.disabled).toBe(true);
    // Pruned during render: never flashes as selected while waiting for the
    // convergence effect.
    expect(ended.className).not.toContain("ant-btn-primary");
    expect(active.className).toContain("ant-btn-primary");
    // The thin convergence effect still reconciles the store.
    expect(dispatch).toHaveBeenCalledWith({
      type: "SET_CLASS_TIMES",
      times: [2],
    });
  });

  it("toggles from the pruned list, not the raw store list", () => {
    vi.useFakeTimers();
    vi.setSystemTime(BASE_NOW);
    const { dispatch } = renderPicker({
      state: makeState({ selectedClassTimes: [1, 2] }),
    });

    fireEvent.click(screen.getByRole("button", { name: "02" }));
    // Deselecting node 2 must not resurrect the ended node 1.
    expect(dispatch).toHaveBeenLastCalledWith({
      type: "SET_CLASS_TIMES",
      times: [],
    });
  });
});

describe("ClassTimePicker clock resync (R11)", () => {
  it("resyncs now when the tab becomes visible again", () => {
    vi.useFakeTimers();
    vi.setSystemTime(BASE_NOW);
    renderPicker();
    expect(screen.getByRole("button", { name: "02" }).disabled).toBe(false);

    // Background throttling: the wall clock jumps 8h but no timer fires.
    act(() => {
      vi.setSystemTime(LATER_SAME_DAY);
    });
    expect(screen.getByRole("button", { name: "02" }).disabled).toBe(false);

    // Tab becomes visible → clock resyncs immediately, node 2 is now ended.
    act(() => {
      document.dispatchEvent(new Event("visibilitychange"));
    });
    expect(screen.getByRole("button", { name: "02" }).disabled).toBe(true);
  });

  it("ignores visibilitychange while the tab stays hidden", () => {
    vi.useFakeTimers();
    vi.setSystemTime(BASE_NOW);
    renderPicker();
    act(() => {
      vi.setSystemTime(LATER_SAME_DAY);
    });

    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => "hidden",
    });
    try {
      act(() => {
        document.dispatchEvent(new Event("visibilitychange"));
      });
      // No resync while hidden; the stale clock keeps node 2 enabled.
      expect(screen.getByRole("button", { name: "02" }).disabled).toBe(false);
    } finally {
      delete document.visibilityState;
    }
  });

  it("clears the timer and the visibility listener on unmount", () => {
    vi.useFakeTimers();
    vi.setSystemTime(BASE_NOW);
    const removeSpy = vi.spyOn(document, "removeEventListener");
    const { unmount } = renderPicker();
    expect(vi.getTimerCount()).toBeGreaterThan(0);

    unmount();
    expect(vi.getTimerCount()).toBe(0);
    expect(
      removeSpy.mock.calls.some(([type]) => type === "visibilitychange")
    ).toBe(true);
    removeSpy.mockRestore();
  });
});
