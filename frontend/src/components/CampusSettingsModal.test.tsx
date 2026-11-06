/**
 * CampusSettingsModal: the data-updated timestamp renders in Asia/Shanghai
 * regardless of the visitor's local timezone (F-06), the running-version row
 * follows the envelope, and the preference switches report through the
 * useSelection() setters.
 * @vitest-environment jsdom
 */
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import CampusSettingsModal from "./CampusSettingsModal";
import type { Envelope } from "../api/decode";
import { SelectionContext, type SelectionValue } from "../selectionContext";

function makeValue(
  overrides: Partial<SelectionValue> = {}
): SelectionValue {
  return {
    selectedCampus: "01",
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

function envelope(overrides: Partial<Envelope> = {}): Envelope {
  return { code: 0, msg: "", data: null, ...overrides };
}

function snapshotWith(
  overrides: Partial<NonNullable<Envelope["data"]>> = {}
): NonNullable<Envelope["data"]> {
  return {
    date: "2026-08-16",
    updated_at: "2026-08-15T16:00:00Z",
    expires_at: "2026-08-16T12:00:00+08:00",
    stale_until: "2026-08-16T23:59:59+08:00",
    stale: false,
    campuses: [],
    error: null,
    ...overrides,
  };
}

function renderModal(todayData: Envelope, value = makeValue()) {
  return {
    value,
    ...render(
      <SelectionContext value={value}>
        <CampusSettingsModal open todayData={todayData} onClose={() => {}} />
      </SelectionContext>
    ),
  };
}

afterEach(() => {
  cleanup();
});

describe("CampusSettingsModal updated time (F-06)", () => {
  it("renders updated_at in Asia/Shanghai", () => {
    // 2026-08-15T16:00:00Z is 2026-08-16 00:00 in Asia/Shanghai (UTC+8).
    renderModal(envelope({ data: snapshotWith() }));
    expect(screen.getByText(/2026-08-16 00:00/)).not.toBeNull();
  });

  it("falls back to 未知 when there is no snapshot", () => {
    renderModal(envelope());
    expect(screen.getByText(/未知/)).not.toBeNull();
  });
});

describe("CampusSettingsModal running version", () => {
  it("renders the running build directly above the repository link", () => {
    renderModal(envelope({ version: "v0.3.0" }));

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
    renderModal(envelope());
    expect(screen.queryByText(/当前运行版本/)).toBeNull();
  });
});

describe("CampusSettingsModal preference switches", () => {
  it("reflects the current preferences and reports toggles", () => {
    const { value } = renderModal(
      envelope(),
      makeValue({ showClassTime: true, canSelectAllDay: true })
    );

    const switches = screen.getAllByRole("switch");
    expect(switches.every((item) => item.getAttribute("aria-checked") === "true")).toBe(
      true
    );

    fireEvent.click(switches[0]);
    fireEvent.click(switches[1]);
    expect(value.setShowClassTime).toHaveBeenLastCalledWith(false);
    expect(value.setCanSelectAllDay).toHaveBeenLastCalledWith(false);
  });
});
