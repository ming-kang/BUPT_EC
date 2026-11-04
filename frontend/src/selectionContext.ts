import { createContext, useContext } from "react";

/**
 * Selection state + setters shared by the pickers. The provider owns the
 * state; components read it (and mutate it) only through useSelection().
 */
export interface SelectionValue {
  selectedCampus: string;
  selectedBuildings: string[];
  selectedClassTimes: number[];
  showClassTime: boolean;
  canSelectAllDay: boolean;
  /** Changing the campus clears the selected buildings and class times. */
  setCampus(id: string): void;
  setBuildings(buildings: string[]): void;
  setClassTimes(times: number[]): void;
  /** Display preferences persist to localStorage. */
  setShowClassTime(value: boolean): void;
  /** Display preferences persist to localStorage. */
  setCanSelectAllDay(value: boolean): void;
}

export const SelectionContext = createContext<SelectionValue | null>(null);

export const SHOW_CLASS_TIME_KEY = "showClassTime";
export const CAN_SELECT_ALL_DAY_KEY = "canSelectAllDay";

function readLocalStorage(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

export function writeLocalStorage(key: string, value: string): void {
  try {
    localStorage.setItem(key, value);
  } catch {
    // Ignore quota / privacy-mode failures; in-memory state still updates.
  }
}

/**
 * Initial display preferences from localStorage. Defaults: class times are
 * shown unless explicitly disabled; all-day selection is opt-in.
 */
export function initPreferences(): Pick<
  SelectionValue,
  "showClassTime" | "canSelectAllDay"
> {
  return {
    showClassTime: readLocalStorage(SHOW_CLASS_TIME_KEY) !== "false",
    canSelectAllDay: readLocalStorage(CAN_SELECT_ALL_DAY_KEY) === "true",
  };
}

export function useSelection(): SelectionValue {
  const ctx = useContext(SelectionContext);
  if (!ctx) {
    throw new Error("useSelection must be used within SelectionProvider");
  }
  return ctx;
}
