/**
 * @vitest-environment jsdom
 *
 * CampusButtonGroup: aria-pressed tracks the render-derived activeCampusId
 * (R12/R10); the settings trigger stays outside the toggle semantics.
 */
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SelectionContext } from "../selectionContext";
import CampusButtonGroup from "./CampusButtonGroup";

const CAMPUSES = [
  { id: "04", name: "沙河" },
  { id: "02", name: "西土城" },
];

// antd inserts a space between two-CJK-character button labels.
const SHAHE = /沙\s*河/;
const XITUCHENG = "西土城";

const TODAY_DATA = { code: 0, msg: "", data: { campuses: CAMPUSES } };

function renderGroup({ activeCampusId = "04", dispatch = vi.fn() } = {}) {
  const view = render(
    <SelectionContext.Provider value={{ state: {}, dispatch }}>
      <CampusButtonGroup
        campuses={CAMPUSES}
        todayData={TODAY_DATA}
        activeCampusId={activeCampusId}
      />
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

describe("CampusButtonGroup aria-pressed (R12)", () => {
  it("presses only the active campus button", () => {
    renderGroup({ activeCampusId: "04" });

    expect(pressedOf(SHAHE)).toBe("true");
    expect(pressedOf(XITUCHENG)).toBe("false");
    // The settings trigger is not a toggle: no aria-pressed at all.
    expect(pressedOf("设置")).toBeNull();
  });

  it("follows activeCampusId when it changes", () => {
    const { rerender } = renderGroup({ activeCampusId: "04" });

    rerender(
      <SelectionContext.Provider value={{ state: {}, dispatch: vi.fn() }}>
        <CampusButtonGroup
          campuses={CAMPUSES}
          todayData={TODAY_DATA}
          activeCampusId="02"
        />
      </SelectionContext.Provider>
    );
    expect(pressedOf(SHAHE)).toBe("false");
    expect(pressedOf(XITUCHENG)).toBe("true");
  });

  it("dispatches the campus selection on click", () => {
    const { dispatch } = renderGroup({ activeCampusId: "04" });
    fireEvent.click(screen.getByRole("button", { name: XITUCHENG }));
    expect(dispatch).toHaveBeenLastCalledWith({ type: "SET_CAMPUS", id: "02" });
  });
});
