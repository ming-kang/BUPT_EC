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
  const [showClassTime, setShowClassTime] = useState(
    initialPrefs.showClassTime
  );
  const [canSelectAllDay, setCanSelectAllDay] = useState(
    initialPrefs.canSelectAllDay
  );

  const setCampus = useCallback((id: string) => {
    setSelectedCampus(id);
    // Buildings and class times are campus-scoped; a campus change resets
    // both so no stale filters survive the switch.
    setSelectedBuildings([]);
    setSelectedClassTimes([]);
  }, []);

  const persistShowClassTime = useCallback((value: boolean) => {
    setShowClassTime(value);
    writeLocalStorage(SHOW_CLASS_TIME_KEY, value ? "true" : "false");
  }, []);

  const persistCanSelectAllDay = useCallback((value: boolean) => {
    setCanSelectAllDay(value);
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
      setShowClassTime: persistShowClassTime,
      setCanSelectAllDay: persistCanSelectAllDay,
    }),
    [
      selectedCampus,
      selectedBuildings,
      selectedClassTimes,
      showClassTime,
      canSelectAllDay,
      setCampus,
      persistShowClassTime,
      persistCanSelectAllDay,
    ]
  );

  return (
    <SelectionContext value={value}>{children}</SelectionContext>
  );
}
