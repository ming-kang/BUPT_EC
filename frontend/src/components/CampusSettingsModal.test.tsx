/**
 * CampusSettingsModal: the data-updated timestamp renders in Asia/Shanghai
 * regardless of the visitor's local timezone (F-06).
 * @vitest-environment jsdom
 */
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import CampusSettingsModal from "./CampusSettingsModal";
import type { Envelope } from "../todayClassroomsResponse";
import { SelectionContext, type SelectionState } from "../selectionContext";

function makeState(): SelectionState {
  return {
    selectedCampus: "01",
    selectedBuildings: [],
    selectedClassTimes: [],
    showClassTime: false,
    canSelectAllDay: false,
  };
}

afterEach(() => {
  cleanup();
});

describe("CampusSettingsModal updated time (F-06)", () => {
  it("renders updated_at in Asia/Shanghai", () => {
    // 2026-08-15T16:00:00Z is 2026-08-16 00:00 in Asia/Shanghai (UTC+8).
    const todayData = {
      data: {
        updated_at: "2026-08-15T16:00:00Z",
        date: "2026-08-16",
      },
    };
    render(
      <SelectionContext.Provider value={{ state: makeState(), dispatch: () => {} }}>
        <CampusSettingsModal open todayData={todayData as Envelope} onClose={() => {}} />
      </SelectionContext.Provider>
    );
    expect(screen.getByText(/2026-08-16 00:00/)).not.toBeNull();
  });

  it("falls back to 未知 when updated_at is missing", () => {
    const todayData = { data: {} };
    render(
      <SelectionContext.Provider value={{ state: makeState(), dispatch: () => {} }}>
        <CampusSettingsModal open todayData={todayData as Envelope} onClose={() => {}} />
      </SelectionContext.Provider>
    );
    expect(screen.getByText(/未知/)).not.toBeNull();
  });
});
