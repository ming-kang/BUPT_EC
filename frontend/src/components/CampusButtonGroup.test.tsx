/**
 * @vitest-environment jsdom
 *
 * CampusButtonGroup: aria-pressed tracks the render-derived activeCampusId
 * (R12/R10); the settings trigger stays outside the toggle semantics.
 */
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  SelectionContext,
  type SelectionValue,
} from "../selectionContext";
import type { CampusInfo } from "../api/types";
import type { Envelope } from "../api/decode";
import CampusButtonGroup from "./CampusButtonGroup";

const CAMPUSES: CampusInfo[] = [
  { id: "04", name: "沙河", buildings: [], nodes: [] },
  { id: "02", name: "西土城", buildings: [], nodes: [] },
];

// antd inserts a space between two-CJK-character button labels.
const SHAHE = /沙\s*河/;
const XITUCHENG = "西土城";

const TODAY_DATA: Envelope = { code: 0, msg: "", data: null };

function makeValue(
  overrides: Partial<SelectionValue> = {}
): SelectionValue {
  return {
    selectedCampus: "",
    selectedBuildings: [],
    selectedClassTimes: [],
    showClassTime: true,
    canSelectAllDay: false,
    setCampus: vi.fn(),
    setBuildings: vi.fn(),
    setClassTimes: vi.fn(),
    setShowClassTime: vi.fn(),
    setCanSelectAllDay: vi.fn(),
    ...overrides,
  };
}

function renderGroup({
  activeCampusId = "04",
  value = makeValue(),
}: {
  activeCampusId?: string;
  value?: SelectionValue;
} = {}) {
  const view = render(
    <SelectionContext.Provider value={value}>
      <CampusButtonGroup
        campuses={CAMPUSES}
        todayData={TODAY_DATA}
        activeCampusId={activeCampusId}
      />
    </SelectionContext.Provider>
  );
  return { ...view, value };
}

function pressedOf(name: string | RegExp) {
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
      <SelectionContext.Provider value={makeValue()}>
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

  it("selects the campus on click", () => {
    const { value } = renderGroup({ activeCampusId: "04" });
    fireEvent.click(screen.getByRole("button", { name: XITUCHENG }));
    expect(value.setCampus).toHaveBeenLastCalledWith("02");
  });
});
