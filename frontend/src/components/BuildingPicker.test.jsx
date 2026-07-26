/**
 * @vitest-environment jsdom
 *
 * BuildingPicker: aria-pressed reflects the selected buildings (R12) and the
 * toggle still dispatches add/remove correctly.
 */
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SelectionContext } from "../selectionContext";
import BuildingPicker from "./BuildingPicker";

const CAMPUS = {
  id: "04",
  name: "沙河",
  buildings: [{ name: "1" }, { name: "未来学习大楼" }],
};

// antd inserts a space between two-CJK-character button labels.
const MAIN_BUILDING = /主\s*楼/;

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
      <BuildingPicker selectedCampusData={CAMPUS} />
    </SelectionContext.Provider>
  );
  return { ...view, dispatch };
}

function pressedOf(name) {
  return screen.getByRole("button", { name }).getAttribute("aria-pressed");
}

afterEach(() => {
  cleanup();
});

describe("BuildingPicker aria-pressed (R12)", () => {
  it("marks only the selected buildings as pressed", () => {
    renderPicker({ state: makeState({ selectedBuildings: ["1"] }) });

    // "1" renders through the numeric alias as 教1, 未来学习大楼 as 主楼.
    expect(pressedOf("教1")).toBe("true");
    expect(pressedOf(MAIN_BUILDING)).toBe("false");
  });

  it("flips aria-pressed when the store selection changes", () => {
    const { rerender } = renderPicker();
    expect(pressedOf(MAIN_BUILDING)).toBe("false");

    rerender(
      <SelectionContext.Provider
        value={{
          state: makeState({ selectedBuildings: ["未来学习大楼"] }),
          dispatch: vi.fn(),
        }}
      >
        <BuildingPicker selectedCampusData={CAMPUS} />
      </SelectionContext.Provider>
    );
    expect(pressedOf(MAIN_BUILDING)).toBe("true");
  });

  it("toggles selection on click", () => {
    const { dispatch } = renderPicker({
      state: makeState({ selectedBuildings: ["1"] }),
    });

    fireEvent.click(screen.getByRole("button", { name: MAIN_BUILDING }));
    expect(dispatch).toHaveBeenLastCalledWith({
      type: "SET_BUILDINGS",
      buildings: ["1", "未来学习大楼"],
    });

    fireEvent.click(screen.getByRole("button", { name: "教1" }));
    expect(dispatch).toHaveBeenLastCalledWith({
      type: "SET_BUILDINGS",
      buildings: [],
    });
  });
});
