import PropTypes from "prop-types";
import { useReducer } from "react";
import {
  SelectionContext,
  initSelectionState,
  selectionReducer,
} from "./selectionContext";

export default function SelectionProvider({ children }) {
  const [state, dispatch] = useReducer(
    selectionReducer,
    undefined,
    initSelectionState
  );
  return (
    <SelectionContext.Provider value={{ state, dispatch }}>
      {children}
    </SelectionContext.Provider>
  );
}

SelectionProvider.propTypes = {
  children: PropTypes.node,
};
