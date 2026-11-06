/**
 * @vitest-environment jsdom
 *
 * BuildingPicker: aria-pressed reflects the selected buildings (R12) and the
 * toggle reports add/remove through the setBuildings setter.
 */
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  SelectionContext,
  type SelectionValue,
} from "../selectionContext";
import type { CampusInfo } from "../api/types";
import BuildingPicker from "./BuildingPicker";

const CAMPUS: CampusInfo = {
  id: "04",
  name: "沙河",
  buildings: [
    { name: "1", display_name: "教1", rooms: [] },
    { name: "未来学习大楼", display_name: "主楼", rooms: [] },
  ],
  nodes: [],
};

// antd inserts a space between two-CJK-character button labels.
const MAIN_BUILDING = /主\s*楼/;

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
      <BuildingPicker selectedCampusData={CAMPUS} />
    </SelectionContext>
  );
  return { ...view, value };
}

function pressedOf(name: string | RegExp) {
  return screen.getByRole("button", { name }).getAttribute("aria-pressed");
}

afterEach(() => {
  cleanup();
});

describe("BuildingPicker aria-pressed (R12)", () => {
  it("marks only the selected buildings as pressed", () => {
    renderPicker({ value: makeValue({ selectedBuildings: ["1"] }) });

    // "1" renders through the numeric alias as 教1, 未来学习大楼 as 主楼.
    expect(pressedOf("教1")).toBe("true");
    expect(pressedOf(MAIN_BUILDING)).toBe("false");
  });

  it("flips aria-pressed when the store selection changes", () => {
    const { rerender } = renderPicker();
    expect(pressedOf(MAIN_BUILDING)).toBe("false");

    rerender(
      <SelectionContext
        value={makeValue({ selectedBuildings: ["未来学习大楼"] })}
      >
        <BuildingPicker selectedCampusData={CAMPUS} />
      </SelectionContext>
    );
    expect(pressedOf(MAIN_BUILDING)).toBe("true");
  });

  it("toggles selection on click", () => {
    const { value } = renderPicker({
      value: makeValue({ selectedBuildings: ["1"] }),
    });

    fireEvent.click(screen.getByRole("button", { name: MAIN_BUILDING }));
    expect(value.setBuildings).toHaveBeenLastCalledWith([
      "1",
      "未来学习大楼",
    ]);

    fireEvent.click(screen.getByRole("button", { name: "教1" }));
    expect(value.setBuildings).toHaveBeenLastCalledWith([]);
  });
});
