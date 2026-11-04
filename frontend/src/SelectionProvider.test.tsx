/**
 * @vitest-environment jsdom
 *
 * SelectionProvider behavior through the public useSelection() API:
 * defaults and persistence of the display preferences, the campus-change
 * rule, and resilience to localStorage failures.
 */
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import SelectionProvider from "./SelectionProvider";
import { useSelection } from "./selectionContext";

function createMemoryLocalStorage(initial: Record<string, string> = {}) {
  const storage = new Map(Object.entries(initial));
  return {
    getItem(key: string) {
      return storage.has(key) ? storage.get(key)! : null;
    },
    setItem(key: string, value: string) {
      storage.set(key, String(value));
    },
    removeItem(key: string) {
      storage.delete(key);
    },
    clear() {
      storage.clear();
    },
  };
}

function Probe() {
  const selection = useSelection();
  return (
    <div>
      <div data-testid="campus">{selection.selectedCampus || "(none)"}</div>
      <div data-testid="buildings">{selection.selectedBuildings.join(",")}</div>
      <div data-testid="times">{selection.selectedClassTimes.join(",")}</div>
      <div data-testid="show-time">{String(selection.showClassTime)}</div>
      <div data-testid="all-day">{String(selection.canSelectAllDay)}</div>
      <button type="button" onClick={() => selection.setCampus("04")}>
        set-campus
      </button>
      <button
        type="button"
        onClick={() => selection.setBuildings(["S1", "S2"])}
      >
        set-buildings
      </button>
      <button type="button" onClick={() => selection.setClassTimes([1, 2])}>
        set-times
      </button>
      <button type="button" onClick={() => selection.setShowClassTime(false)}>
        hide-time
      </button>
      <button
        type="button"
        onClick={() => selection.setCanSelectAllDay(true)}
      >
        allow-all-day
      </button>
    </div>
  );
}

function renderProbe() {
  return render(
    <SelectionProvider>
      <Probe />
    </SelectionProvider>
  );
}

beforeEach(() => {
  vi.stubGlobal("localStorage", createMemoryLocalStorage());
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("SelectionProvider", () => {
  it("initializes defaults with empty localStorage", () => {
    renderProbe();
    expect(screen.getByTestId("campus").textContent).toBe("(none)");
    expect(screen.getByTestId("buildings").textContent).toBe("");
    expect(screen.getByTestId("times").textContent).toBe("");
    expect(screen.getByTestId("show-time").textContent).toBe("true");
    expect(screen.getByTestId("all-day").textContent).toBe("false");
  });

  it("restores persisted display preferences", () => {
    cleanup();
    vi.stubGlobal(
      "localStorage",
      createMemoryLocalStorage({
        showClassTime: "false",
        canSelectAllDay: "true",
      })
    );
    renderProbe();
    expect(screen.getByTestId("show-time").textContent).toBe("false");
    expect(screen.getByTestId("all-day").textContent).toBe("true");
  });

  it("persists preference changes directly from the setters", () => {
    renderProbe();
    fireEvent.click(screen.getByRole("button", { name: "hide-time" }));
    fireEvent.click(screen.getByRole("button", { name: "allow-all-day" }));

    expect(screen.getByTestId("show-time").textContent).toBe("false");
    expect(screen.getByTestId("all-day").textContent).toBe("true");
    expect(localStorage.getItem("showClassTime")).toBe("false");
    expect(localStorage.getItem("canSelectAllDay")).toBe("true");
  });

  it("clears buildings and class times when the campus changes", () => {
    renderProbe();
    fireEvent.click(screen.getByRole("button", { name: "set-buildings" }));
    fireEvent.click(screen.getByRole("button", { name: "set-times" }));
    expect(screen.getByTestId("buildings").textContent).toBe("S1,S2");
    expect(screen.getByTestId("times").textContent).toBe("1,2");

    fireEvent.click(screen.getByRole("button", { name: "set-campus" }));
    expect(screen.getByTestId("campus").textContent).toBe("04");
    expect(screen.getByTestId("buildings").textContent).toBe("");
    expect(screen.getByTestId("times").textContent).toBe("");
  });

  it("keeps selection changes out of localStorage", () => {
    renderProbe();
    fireEvent.click(screen.getByRole("button", { name: "set-campus" }));
    fireEvent.click(screen.getByRole("button", { name: "set-buildings" }));
    expect(localStorage.getItem("showClassTime")).toBeNull();
    expect(localStorage.getItem("canSelectAllDay")).toBeNull();
  });

  it("survives localStorage failures", () => {
    cleanup();
    vi.stubGlobal("localStorage", {
      getItem() {
        throw new Error("blocked");
      },
      setItem() {
        throw new Error("blocked");
      },
      removeItem() {
        throw new Error("blocked");
      },
      clear() {
        throw new Error("blocked");
      },
    });
    renderProbe();
    // Defaults apply when reads are blocked.
    expect(screen.getByTestId("show-time").textContent).toBe("true");
    // In-memory state still updates when writes are blocked.
    fireEvent.click(screen.getByRole("button", { name: "hide-time" }));
    expect(screen.getByTestId("show-time").textContent).toBe("false");
  });

  it("throws when useSelection runs outside the provider", () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => render(<Probe />)).toThrowError(
      /useSelection must be used within SelectionProvider/
    );
    errorSpy.mockRestore();
  });
});
