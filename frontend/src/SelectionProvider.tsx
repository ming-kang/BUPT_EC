import { useCallback, useMemo, useState, type ReactNode } from "react";
import {
  CAN_SELECT_ALL_DAY_KEY,
  SelectionContext,
  SHOW_CLASS_TIME_KEY,
  initPreferences,
  writeLocalStorage,
  type SelectionValue,
} from "./selectionContext";

/**
 * Owns the selection state. Preferences persist to localStorage directly in
 * their setters; the campus setter clears the dependent selections.
 */
export default function SelectionProvider({
  children,
}: {
  children?: ReactNode;
}) {
  const [selectedCampus, setSelectedCampus] = useState("");
  const [selectedBuildings, setSelectedBuildings] = useState<string[]>([]);
  const [selectedClassTimes, setSelectedClassTimes] = useState<number[]>([]);
  const [initialPrefs] = useState(initPreferences);
  const [showClassTime, setShowClassTimeState] = useState(
    initialPrefs.showClassTime
  );
  const [canSelectAllDay, setCanSelectAllDayState] = useState(
    initialPrefs.canSelectAllDay
  );

  const setCampus = useCallback((id: string) => {
    setSelectedCampus(id);
    // Buildings and class times are campus-scoped; a campus change resets
    // both so no stale filters survive the switch.
    setSelectedBuildings([]);
    setSelectedClassTimes([]);
  }, []);

  const setShowClassTime = useCallback((value: boolean) => {
    setShowClassTimeState(value);
    writeLocalStorage(SHOW_CLASS_TIME_KEY, value ? "true" : "false");
  }, []);

  const setCanSelectAllDay = useCallback((value: boolean) => {
    setCanSelectAllDayState(value);
    writeLocalStorage(CAN_SELECT_ALL_DAY_KEY, value ? "true" : "false");
  }, []);

  const value = useMemo<SelectionValue>(
    () => ({
      selectedCampus,
      selectedBuildings,
      selectedClassTimes,
      showClassTime,
      canSelectAllDay,
      setCampus,
      setBuildings: setSelectedBuildings,
      setClassTimes: setSelectedClassTimes,
      setShowClassTime,
      setCanSelectAllDay,
    }),
    [
      selectedCampus,
      selectedBuildings,
      selectedClassTimes,
      showClassTime,
      canSelectAllDay,
      setCampus,
      setShowClassTime,
      setCanSelectAllDay,
    ]
  );

  return (
    <SelectionContext.Provider value={value}>
      {children}
    </SelectionContext.Provider>
  );
}
