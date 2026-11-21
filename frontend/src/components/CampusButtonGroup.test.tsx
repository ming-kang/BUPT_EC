/**
 * @vitest-environment jsdom
 *
 * CampusButtonGroup: aria-checked radios track the render-derived
 * activeCampusId; the settings trigger stays a plain button.
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
    <SelectionContext value={value}>
      <CampusButtonGroup
        campuses={CAMPUSES}
        todayData={TODAY_DATA}
        activeCampusId={activeCampusId}
      />
    </SelectionContext>
  );
  return { ...view, value };
}

function checkedOf(name: string | RegExp) {
  return screen.getByRole("radio", { name }).getAttribute("aria-checked");
}

afterEach(() => {
  cleanup();
});

describe("CampusButtonGroup radio group", () => {
  it("checks only the active campus radio", () => {
    renderGroup({ activeCampusId: "04" });

    expect(screen.getByRole("radiogroup")).not.toBeNull();
    expect(checkedOf(SHAHE)).toBe("true");
    expect(checkedOf(XITUCHENG)).toBe("false");
    const settingsTrigger = screen.getByRole("button", { name: "设置" });
    expect(settingsTrigger.hasAttribute("aria-pressed")).toBe(false);
    expect(settingsTrigger.hasAttribute("aria-checked")).toBe(false);
  });

  it("follows activeCampusId when it changes", () => {
    const { rerender } = renderGroup({ activeCampusId: "04" });

    rerender(
      <SelectionContext value={makeValue()}>
        <CampusButtonGroup
          campuses={CAMPUSES}
          todayData={TODAY_DATA}
          activeCampusId="02"
        />
      </SelectionContext>
    );
    expect(checkedOf(SHAHE)).toBe("false");
    expect(checkedOf(XITUCHENG)).toBe("true");
  });

  it("selects the campus on click", () => {
    const { value } = renderGroup({ activeCampusId: "04" });
    fireEvent.click(screen.getByRole("radio", { name: XITUCHENG }));
    expect(value.setCampus).toHaveBeenLastCalledWith("02");
  });
});
