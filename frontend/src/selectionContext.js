import { createContext, useContext } from "react";

export const SelectionContext = createContext(null);

export function initSelectionState() {
  return {
    selectedCampus: "",
    selectedBuildings: [],
    selectedClassTimes: [],
    showClassTime: localStorage.getItem("showClassTime") !== "false",
    canSelectAllDay: localStorage.getItem("canSelectAllDay") === "true",
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
      localStorage.setItem("showClassTime", action.value ? "true" : "false");
      return { ...state, showClassTime: action.value };
    case "SET_CAN_SELECT_ALL_DAY":
      localStorage.setItem("canSelectAllDay", action.value ? "true" : "false");
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
