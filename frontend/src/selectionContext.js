import { createContext, useContext } from "react";

export const SelectionContext = createContext(null);

function readLocalStorage(key) {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function writeLocalStorage(key, value) {
  try {
    localStorage.setItem(key, value);
  } catch {
    // Ignore quota / privacy-mode failures; in-memory state still updates.
  }
}

export function initSelectionState() {
  return {
    selectedCampus: "",
    selectedBuildings: [],
    selectedClassTimes: [],
    showClassTime: readLocalStorage("showClassTime") !== "false",
    canSelectAllDay: readLocalStorage("canSelectAllDay") === "true",
  };
}

export function selectionReducer(state, action) {
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
      writeLocalStorage("showClassTime", action.value ? "true" : "false");
      return { ...state, showClassTime: action.value };
    case "SET_CAN_SELECT_ALL_DAY":
      writeLocalStorage("canSelectAllDay", action.value ? "true" : "false");
      return { ...state, canSelectAllDay: action.value };
    default:
      return state;
  }
}

export function useSelection() {
  const ctx = useContext(SelectionContext);
  if (!ctx) {
    throw new Error("useSelection must be used within SelectionProvider");
  }
  return ctx;
}
