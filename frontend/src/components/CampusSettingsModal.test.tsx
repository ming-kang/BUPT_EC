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

describe("CampusSettingsModal running version", () => {
  it("renders the running build directly above the repository link", () => {
    const todayData = { version: "v0.3.0", data: {} };
    render(
      <SelectionContext.Provider value={{ state: makeState(), dispatch: () => {} }}>
        <CampusSettingsModal open todayData={todayData as Envelope} onClose={() => {}} />
      </SelectionContext.Provider>
    );

    const versionRow = screen.getByText(/当前运行版本：v0\.3\.0/);
    const repositoryRow = screen.getByText(/项目已开源/);
    // Order only, not structure: the row must precede the repository link.
    expect(
      versionRow.compareDocumentPosition(repositoryRow) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();
  });

  it("omits the row when the server sent no version", () => {
    // Pre-0.3.0 backend: an empty "未知" would tell the user nothing, and the
    // row appears on its own once the server is upgraded.
    const todayData = { data: {} };
    render(
      <SelectionContext.Provider value={{ state: makeState(), dispatch: () => {} }}>
        <CampusSettingsModal open todayData={todayData as Envelope} onClose={() => {}} />
      </SelectionContext.Provider>
    );
    expect(screen.queryByText(/当前运行版本/)).toBeNull();
  });
});
