import PropTypes from "prop-types";
import { Button, Card } from "antd";
import { useEffect, useMemo, useState } from "react";
import {
  formatShanghaiDate,
  formatShanghaiTime,
  isNodeEnded,
  pruneEndedClassTimes,
} from "../classTimeUtils";
import { useSelection } from "../selectionContext";
import "./ClassTimePicker.css";

const FIVE_MINUTES_MS = 5 * 60 * 1000;
const EMPTY_CLASS_TIMES = [];

function ClassTimePicker({ selectedCampusData, todayDate }) {
  const { state, dispatch } = useSelection();
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    let timeoutID;

    function scheduleNextTick() {
      timeoutID = window.setTimeout(() => {
        setNow(new Date());
        scheduleNextTick();
      }, msUntilNextFiveMinuteTick(new Date()));
    }

    scheduleNextTick();
    return () => window.clearTimeout(timeoutID);
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
      selectable.every((item) => selectedClassTimes.includes(item.node))
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

  function renderTime(time, index) {
    const [start = "", end = ""] = String(time || "").split("-");
    return index === 0 ? start : end;
  }

  return (
    <Card className="class-time-picker responsive-card">
      {normalizedOptions.map((item) => (
        <Button
          key={item.node}
          type={selectedClassTimes.includes(item.node) ? "primary" : "default"}
          className={state.showClassTime ? "time-slot-show-time" : ""}
          onClick={() => {
            const times = selectedClassTimes.includes(item.node)
              ? selectedClassTimes.filter((node) => node !== item.node)
              : [...selectedClassTimes, item.node];
            dispatch({ type: "SET_CLASS_TIMES", times });
          }}
          disabled={item.disabled}
        >
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
        </Button>
      ))}
      <Button
        type={isAllChecked() ? "primary" : "default"}
        className={`select-all-btn ${state.showClassTime ? "time-slot-show-time" : ""}`}
        onClick={onCheckAllChange}
      >
        {isAllChecked() ? "取消" : "全选"}
      </Button>
    </Card>
  );
}

function msUntilNextFiveMinuteTick(date) {
  const next = new Date(date);
  next.setSeconds(0, 0);

  const tickMinutes = FIVE_MINUTES_MS / (60 * 1000);
  const remainder = next.getMinutes() % tickMinutes;
  const minutesToAdd = remainder === 0 ? tickMinutes : tickMinutes - remainder;
  next.setMinutes(next.getMinutes() + minutesToAdd);

  return Math.max(next.getTime() - date.getTime(), 1000);
}

ClassTimePicker.propTypes = {
  selectedCampusData: PropTypes.object,
  todayDate: PropTypes.string,
};

export default ClassTimePicker;
