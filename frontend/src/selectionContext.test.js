import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { initSelectionState, selectionReducer } from "./selectionContext";

function createMemoryLocalStorage(initialValues = {}) {
  const storage = new Map(Object.entries(initialValues));
  return {
    getItem(key) {
      return storage.has(key) ? storage.get(key) : null;
    },
    setItem(key, value) {
      storage.set(key, String(value));
    },
    removeItem(key) {
      storage.delete(key);
    },
    clear() {
      storage.clear();
    },
  };
}

describe("selection state", () => {
  beforeEach(() => {
    vi.stubGlobal("localStorage", createMemoryLocalStorage());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("initializes persisted preferences from localStorage", () => {
    vi.stubGlobal(
      "localStorage",
      createMemoryLocalStorage({ showClassTime: "false", canSelectAllDay: "true" })
    );

    expect(initSelectionState()).toMatchObject({
      showClassTime: false,
      canSelectAllDay: true,
    });
  });

  it("clears dependent selections when campus changes", () => {
    const initialState = {
      selectedCampus: "01",
      selectedBuildings: ["教学实验综合楼"],
      selectedClassTimes: [1, 2],
      showClassTime: true,
      canSelectAllDay: false,
    };

    expect(selectionReducer(initialState, { type: "SET_CAMPUS", id: "04" })).toEqual({
      selectedCampus: "04",
      selectedBuildings: [],
      selectedClassTimes: [],
      showClassTime: true,
      canSelectAllDay: false,
    });
  });

  it("updates list selections without changing unrelated preferences", () => {
    const initialState = initSelectionState();
    const withBuildings = selectionReducer(initialState, {
      type: "SET_BUILDINGS",
      buildings: ["教学实验综合楼"],
    });
    const withClassTimes = selectionReducer(withBuildings, {
      type: "SET_CLASS_TIMES",
      times: [1, 3],
    });

    expect(withClassTimes).toEqual({
      selectedCampus: "",
      selectedBuildings: ["教学实验综合楼"],
      selectedClassTimes: [1, 3],
      showClassTime: true,
      canSelectAllDay: false,
    });
  });

  it("persists display preference actions to localStorage", () => {
    const initialState = initSelectionState();

    const hiddenTimeState = selectionReducer(initialState, {
      type: "SET_SHOW_CLASS_TIME",
      value: false,
    });
    const allDayState = selectionReducer(hiddenTimeState, {
      type: "SET_CAN_SELECT_ALL_DAY",
      value: true,
    });

    expect(hiddenTimeState.showClassTime).toBe(false);
    expect(allDayState.canSelectAllDay).toBe(true);
    expect(localStorage.getItem("showClassTime")).toBe("false");
    expect(localStorage.getItem("canSelectAllDay")).toBe("true");
  });
});
