import PropTypes from "prop-types";
import { useEffect, useReducer, useRef } from "react";
import {
  CAN_SELECT_ALL_DAY_KEY,
  SelectionContext,
  SHOW_CLASS_TIME_KEY,
  initSelectionState,
  selectionReducer,
  writeLocalStorage,
} from "./selectionContext";

export default function SelectionProvider({ children }) {
  const [state, dispatch] = useReducer(
    selectionReducer,
    undefined,
    initSelectionState
  );
  const previousPrefs = useRef({
    showClassTime: state.showClassTime,
    canSelectAllDay: state.canSelectAllDay,
  });

  useEffect(() => {
    const previous = previousPrefs.current;
    if (previous.showClassTime !== state.showClassTime) {
      writeLocalStorage(
        SHOW_CLASS_TIME_KEY,
        state.showClassTime ? "true" : "false"
      );
    }
    if (previous.canSelectAllDay !== state.canSelectAllDay) {
      writeLocalStorage(
        CAN_SELECT_ALL_DAY_KEY,
        state.canSelectAllDay ? "true" : "false"
      );
    }
    previousPrefs.current = {
      showClassTime: state.showClassTime,
      canSelectAllDay: state.canSelectAllDay,
    };
  }, [state.showClassTime, state.canSelectAllDay]);

  return (
    <SelectionContext.Provider value={{ state, dispatch }}>
      {children}
    </SelectionContext.Provider>
  );
}

SelectionProvider.propTypes = {
  children: PropTypes.node,
};
