import PropTypes from "prop-types";
import { Button, Card } from "antd";
import "./ClassTimePicker.css";

function ClassTimePicker(props) {
  if (!props.selectedCampusData) {
    return null;
  }

  const selectedClassTimes = props.selectedClassTimes || [];
  const options = Array.isArray(props.selectedCampusData.nodes)
    ? props.selectedCampusData.nodes
    : [];
  const nowTime = new Date().toTimeString().slice(0, 5);
  const isToday = props.todayDate === formatLocalDate(new Date());

  const normalizedOptions = options.map((item) => {
    const [, endTime = ""] = String(item.time || "").split("-");
    return {
      ...item,
      disabled:
        endTime.localeCompare(nowTime) < 0 &&
        !props.canSelectAllDay &&
        isToday,
    };
  });

  function isAllChecked() {
    const selectable = normalizedOptions.filter((item) => !item.disabled);
    return (
      selectable.length > 0 &&
      selectable.every((item) => selectedClassTimes.includes(item.node))
    );
  }

  function onCheckAllChange() {
    if (isAllChecked()) {
      props.setSelectedClassTimes([]);
      return;
    }
    props.setSelectedClassTimes(
      normalizedOptions.filter((item) => !item.disabled).map((item) => item.node)
    );
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
          type={selectedClassTimes.includes(item.node) ? "primary" : "outline"}
          className={props.showClassTime ? "time-slot-show-time" : ""}
          onClick={() => {
            if (selectedClassTimes.includes(item.node)) {
              props.setSelectedClassTimes(
                selectedClassTimes.filter((node) => node !== item.node)
              );
            } else {
              props.setSelectedClassTimes([...selectedClassTimes, item.node]);
            }
          }}
          style={{
            color: item.disabled
              ? props.isDark
                ? "rgba(255,255,255,0.45)"
                : "rgba(0,0,0,0.45)"
              : undefined,
          }}
          disabled={item.disabled}
        >
          <div>
            {props.showClassTime ? (
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
            {props.showClassTime ? (
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
        type={isAllChecked() ? "primary" : "outline"}
        className={`select-all-btn ${props.showClassTime ? "time-slot-show-time" : ""}`}
        onClick={onCheckAllChange}
      >
        {isAllChecked() ? "全不选" : "全选"}
      </Button>
    </Card>
  );
}

function formatLocalDate(date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

ClassTimePicker.propTypes = {
  selectedCampusData: PropTypes.object,
  todayDate: PropTypes.string,
  selectedClassTimes: PropTypes.array,
  setSelectedClassTimes: PropTypes.func,
  showClassTime: PropTypes.bool,
  canSelectAllDay: PropTypes.bool,
  isDark: PropTypes.bool,
};

export default ClassTimePicker;
