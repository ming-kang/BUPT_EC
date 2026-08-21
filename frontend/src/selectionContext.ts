import { createContext, useContext, type Dispatch } from "react";

export interface SelectionState {
  selectedCampus: string;
  selectedBuildings: string[];
  selectedClassTimes: number[];
  showClassTime: boolean;
  canSelectAllDay: boolean;
}

export type SelectionAction =
  | { type: "SET_CAMPUS"; id: string }
  | { type: "SET_BUILDINGS"; buildings: string[] }
  | { type: "SET_CLASS_TIMES"; times: number[] }
  | { type: "SET_SHOW_CLASS_TIME"; value: boolean }
  | { type: "SET_CAN_SELECT_ALL_DAY"; value: boolean };

export interface SelectionContextValue {
  state: SelectionState;
  dispatch: Dispatch<SelectionAction>;
}

export const SelectionContext = createContext<SelectionContextValue | null>(
  null
);

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

export function initSelectionState(): SelectionState {
  return {
    selectedCampus: "",
    selectedBuildings: [],
    selectedClassTimes: [],
    showClassTime: readLocalStorage(SHOW_CLASS_TIME_KEY) !== "false",
    canSelectAllDay: readLocalStorage(CAN_SELECT_ALL_DAY_KEY) === "true",
  };
}

/** Pure reducer — no I/O. Persistence lives in SelectionProvider. */
export function selectionReducer(
  state: SelectionState,
  action: SelectionAction
): SelectionState {
  switch (action.type) {
    case "SET_CAMPUS":
      return {
        ...state,
        selectedCampus: action.id,
        selectedBuildings: [],
        selectedClassTimes: [],
      };
    case "SET_BUILDINGS":
      return { ...state, selectedBuildings: action.buildings };
    case "SET_CLASS_TIMES":
      return { ...state, selectedClassTimes: action.times };
    case "SET_SHOW_CLASS_TIME":
      return { ...state, showClassTime: action.value };
    case "SET_CAN_SELECT_ALL_DAY":
      return { ...state, canSelectAllDay: action.value };
    default:
      return state;
  }
}

export function useSelection(): SelectionContextValue {
  const ctx = useContext(SelectionContext);
  if (!ctx) {
    throw new Error("useSelection must be used within SelectionProvider");
  }
  return ctx;
}
