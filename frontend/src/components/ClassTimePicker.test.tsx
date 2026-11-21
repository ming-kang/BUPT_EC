/**
 * @vitest-environment jsdom
 */
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  SelectionContext,
  type SelectionValue,
} from "../selectionContext";
import type { CampusInfo } from "../api/types";
import ClassTimePicker from "./ClassTimePicker";

// 2026-07-27 10:00 Asia/Shanghai.
const BASE_NOW = new Date("2026-07-27T02:00:00.000Z");
// 2026-07-27 18:00 Asia/Shanghai (same business day; node 2 has ended).
const LATER_SAME_DAY = new Date("2026-07-27T10:00:00.000Z");
const TODAY = "2026-07-27";

const CAMPUS: CampusInfo = {
  id: "04",
  name: "沙河",
  buildings: [],
  nodes: [
    { node: 1, time: "08:00-08:45", room_count: 0 },
    { node: 2, time: "15:00-15:45", room_count: 0 },
    { node: 3, time: "19:00-19:45", room_count: 0 },
  ],
};

function makeValue(
  overrides: Partial<SelectionValue> = {}
): SelectionValue {
  return {
    selectedCampus: "04",
    selectedBuildings: [],
    selectedClassTimes: [],
    showClassTime: false,
    canSelectAllDay: false,
    setCampus: vi.fn(),
    setBuildings: vi.fn(),
    setClassTimes: vi.fn(),
    setShowClassTime: vi.fn(),
    setCanSelectAllDay: vi.fn(),
    ...overrides,
  };
}

function renderPicker({ value = makeValue() }: { value?: SelectionValue } = {}) {
  const view = render(
    <SelectionContext value={value}>
      <ClassTimePicker selectedCampusData={CAMPUS} todayDate={TODAY} />
    </SelectionContext>
  );
  return { ...view, value };
}

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("ClassTimePicker derived pruning", () => {
  it("renders ended selections as unselected on the first frame", () => {
    vi.useFakeTimers();
    vi.setSystemTime(BASE_NOW);
    const { value } = renderPicker({
      value: makeValue({ selectedClassTimes: [1, 2] }),
    });

    const ended = screen.getByRole("button", { name: "01" });
    const active = screen.getByRole("button", { name: "02" });
    expect((ended as HTMLButtonElement).disabled).toBe(true);
    expect(ended.className).not.toContain("ant-btn-primary");
    expect(active.className).toContain("ant-btn-primary");
    expect(value.setClassTimes).toHaveBeenCalledWith([2]);
  });

  it("does not write back when pruning removes no selected nodes", () => {
    vi.useFakeTimers();
    vi.setSystemTime(BASE_NOW);
    const { value } = renderPicker({
      value: makeValue({ selectedClassTimes: [2, 3] }),
    });
    expect(value.setClassTimes).not.toHaveBeenCalled();
  });

  it("toggles from the pruned list, not the raw store list", () => {
    vi.useFakeTimers();
    vi.setSystemTime(BASE_NOW);
    const { value } = renderPicker({
      value: makeValue({ selectedClassTimes: [1, 2] }),
    });

    fireEvent.click(screen.getByRole("button", { name: "02" }));
    expect(value.setClassTimes).toHaveBeenLastCalledWith([]);
  });
});

describe("ClassTimePicker aria-pressed", () => {
  function pressedOf(name: string | RegExp) {
    return screen.getByRole("button", { name }).getAttribute("aria-pressed");
  }

  it("exposes the pruned selection through aria-pressed", () => {
    vi.useFakeTimers();
    vi.setSystemTime(BASE_NOW);
    renderPicker({ value: makeValue({ selectedClassTimes: [1, 2] }) });

    expect(pressedOf("01")).toBe("false");
    expect(pressedOf("02")).toBe("true");
    expect(pressedOf("03")).toBe("false");
  });

  it("flips aria-pressed when the selection changes", () => {
    vi.useFakeTimers();
    vi.setSystemTime(BASE_NOW);
    const { rerender } = renderPicker();
    expect(pressedOf("03")).toBe("false");

    rerender(
      <SelectionContext
        value={makeValue({ selectedClassTimes: [3] })}
      >
        <ClassTimePicker selectedCampusData={CAMPUS} todayDate={TODAY} />
      </SelectionContext>
    );
    expect(pressedOf("03")).toBe("true");
  });
});

describe("ClassTimePicker clock resync", () => {
  it("resyncs now when the tab becomes visible again", () => {
    vi.useFakeTimers();
    vi.setSystemTime(BASE_NOW);
    renderPicker();
    expect((screen.getByRole("button", { name: "02" }) as HTMLButtonElement).disabled).toBe(false);

    act(() => {
      vi.setSystemTime(LATER_SAME_DAY);
    });
    expect((screen.getByRole("button", { name: "02" }) as HTMLButtonElement).disabled).toBe(false);

    act(() => {
      document.dispatchEvent(new Event("visibilitychange"));
    });
    expect((screen.getByRole("button", { name: "02" }) as HTMLButtonElement).disabled).toBe(true);
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
      expect((screen.getByRole("button", { name: "02" }) as HTMLButtonElement).disabled).toBe(false);
    } finally {
      Reflect.deleteProperty(document, "visibilityState");
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
