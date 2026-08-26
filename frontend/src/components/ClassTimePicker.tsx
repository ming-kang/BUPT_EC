import type { CampusInfo } from "../api/types";
import { useEffect, useMemo, useState } from "react";
import {
  formatShanghaiDate,
  formatShanghaiTime,
  isNodeEnded,
  pruneEndedClassTimes,
} from "../classTimeUtils";
import { useSelection } from "../selectionContext";
import { ToggleButton, ToggleButtonGroup } from "./ToggleButtonGroup";
import Panel from "./Panel";
import "./ClassTimePicker.css";

const FIVE_MINUTES_MS = 5 * 60 * 1000;
const EMPTY_CLASS_TIMES: number[] = [];

interface ClassTimePickerProps {
  selectedCampusData?: CampusInfo | null;
  todayDate?: string;
}

function ClassTimePicker({ selectedCampusData, todayDate }: ClassTimePickerProps) {
  const { state, dispatch } = useSelection();
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    let timeoutID: number | undefined;

    function scheduleNextTick() {
      timeoutID = window.setTimeout(() => {
        setNow(new Date());
        scheduleNextTick();
      }, msUntilNextFiveMinuteTick(new Date()));
    }

    // R11: browsers throttle background-tab timers, so `now` can lag tens of
    // minutes after the tab was hidden. When it becomes visible again, drop
    // the stale alarm, resync immediately, and re-align to the 5-minute grid.
    function onVisibilityChange() {
      if (document.visibilityState !== "hidden") {
        window.clearTimeout(timeoutID);
        setNow(new Date());
        scheduleNextTick();
      }
    }

    scheduleNextTick();
    document.addEventListener("visibilitychange", onVisibilityChange);
    return () => {
      window.clearTimeout(timeoutID);
      document.removeEventListener("visibilitychange", onVisibilityChange);
    };
  }, []);

  const selectedClassTimes = useMemo(
    () =>
      Array.isArray(state.selectedClassTimes)
        ? state.selectedClassTimes
        : EMPTY_CLASS_TIMES,
    [state.selectedClassTimes]
  );
  const options = Array.isArray(selectedCampusData?.nodes)
    ? selectedCampusData.nodes
    : [];
  const nowTime = formatShanghaiTime(now);
  const isToday = todayDate === formatShanghaiDate(now);

  // R10: prune during render so ended nodes never flash as selected while
  // waiting for the convergence effect below. The store still converges via
  // that effect (guards unchanged), keeping TodayClassroomTable consistent.
  const prunedSelected = pruneEndedClassTimes(selectedClassTimes, options, {
    nowTime,
    isToday,
    canSelectAllDay: state.canSelectAllDay,
  });

  useEffect(() => {
    if (!selectedCampusData) {
      return;
    }
    const nodes = Array.isArray(selectedCampusData.nodes)
      ? selectedCampusData.nodes
      : [];
    const next = pruneEndedClassTimes(selectedClassTimes, nodes, {
      nowTime,
      isToday,
      canSelectAllDay: state.canSelectAllDay,
    });
    if (
      next.length !== selectedClassTimes.length ||
      next.some((node, index) => node !== selectedClassTimes[index])
    ) {
      dispatch({ type: "SET_CLASS_TIMES", times: next });
    }
  }, [
    selectedCampusData,
    selectedClassTimes,
    nowTime,
    isToday,
    state.canSelectAllDay,
    dispatch,
  ]);

  if (!selectedCampusData) {
    return null;
  }

  const normalizedOptions = options.map((item) => ({
    ...item,
    disabled:
      isNodeEnded(item.time, nowTime) && !state.canSelectAllDay && isToday,
  }));

  function isAllChecked() {
    const selectable = normalizedOptions.filter((item) => !item.disabled);
    return (
      selectable.length > 0 &&
      selectable.every((item) => prunedSelected.includes(item.node))
    );
  }

  function onCheckAllChange() {
    if (isAllChecked()) {
      dispatch({ type: "SET_CLASS_TIMES", times: [] });
      return;
    }
    dispatch({
      type: "SET_CLASS_TIMES",
      times: normalizedOptions
        .filter((item) => !item.disabled)
        .map((item) => item.node),
    });
  }

  function renderTime(time: unknown, index: number) {
    const [start = "", end = ""] = String(time || "").split("-");
    return index === 0 ? start : end;
  }

  return (
    <Panel className="class-time-picker responsive-card">
      <ToggleButtonGroup
        className="class-time-buttons"
        options={normalizedOptions.map((item) => ({
          value: item.node,
          disabled: item.disabled,
          className: state.showClassTime ? "time-slot-show-time" : "",
          content: (
            <div>
              {state.showClassTime ? (
                <div
                  style={{
                    fontSize: "0.7em",
                    marginBottom: "-0.5em",
                  }}
                >
                  {renderTime(item.time, 0)}
                </div>
              ) : null}
              {String(item.node).padStart(2, "0")}
              {state.showClassTime ? (
                <div
                  style={{
                    fontSize: "0.7em",
                    marginTop: "-0.5em",
                  }}
                >
                  {renderTime(item.time, 1)}
                </div>
              ) : null}
            </div>
          ),
        }))}
        selectedValues={prunedSelected}
        onToggle={(node) => {
          const times = prunedSelected.includes(node)
            ? prunedSelected.filter((x) => x !== node)
            : [...prunedSelected, node];
          dispatch({ type: "SET_CLASS_TIMES", times });
        }}
      />
      <ToggleButton
        className={`select-all-btn ${state.showClassTime ? "time-slot-show-time" : ""}`}
        onClick={onCheckAllChange}
      >
        {isAllChecked() ? "取消" : "全选"}
      </ToggleButton>
    </Panel>
  );
}

function msUntilNextFiveMinuteTick(date: Date): number {
  const next = new Date(date);
  next.setSeconds(0, 0);

  const tickMinutes = FIVE_MINUTES_MS / (60 * 1000);
  const remainder = next.getMinutes() % tickMinutes;
  const minutesToAdd = remainder === 0 ? tickMinutes : tickMinutes - remainder;
  next.setMinutes(next.getMinutes() + minutesToAdd);

  return Math.max(next.getTime() - date.getTime(), 1000);
}

export default ClassTimePicker;
