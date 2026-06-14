import "./App.css";
import { Alert, ConfigProvider, Spin, Typography, theme } from "antd";
import { useEffect, useMemo, useState } from "react";
import CampusButtonGroup from "./components/CampusButtonGroup";
import BuildingPicker from "./components/BuildingPicker";
import ClassTimePicker from "./components/ClassTimePicker";
import TodayClassroomTable from "./components/TodayClassroomTable";
import GlobalEmpty from "./components/GlobalEmpty";
import Footer from "./components/Footer";
import useTodayClassrooms from "./useTodayClassrooms";

const WEEKDAY_LABELS = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];

function formatDateWithWeekday(dateStr) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(dateStr);
  if (!match) return dateStr;
  const [, y, m, d] = match;
  const date = new Date(Number(y), Number(m) - 1, Number(d));
  return `${dateStr} · ${WEEKDAY_LABELS[date.getDay()]}`;
}

function App() {
  const { resp, spinning, isError, retry } = useTodayClassrooms();
  const [selectedCampus, setSelectedCampus] = useState("");
  const [selectedBuildings, setSelectedBuildings] = useState([]);
  const [selectedClassTimes, setSelectedClassTimes] = useState([]);
  const [showClassTime, setShowClassTime] = useState(false);
  const [canSelectAllDay, setCanSelectAllDay] = useState(false);
  const [isDark, setIsDark] = useState(false);

  const { Title } = Typography;

  const campuses = useMemo(
    () =>
      resp.code === 0 && Array.isArray(resp.data?.campuses)
        ? resp.data.campuses
        : [],
    [resp]
  );
  const selectedCampusData = useMemo(
    () => campuses.find((campus) => campus.id === selectedCampus) || null,
    [campuses, selectedCampus]
  );

  useEffect(() => {
    const mql = window.matchMedia("(prefers-color-scheme: dark)");

    function matchMode(e) {
      const body = document.body;
      if (e.matches) {
        body.classList.add("dark");
        setIsDark(true);
        localStorage.setItem("darkMode", "true");
      } else {
        body.classList.remove("dark");
        setIsDark(false);
        localStorage.setItem("darkMode", "false");
      }
    }

    mql.addEventListener("change", matchMode);
    matchMode(mql);

    setShowClassTime(localStorage.getItem("showClassTime") !== "false");
    setCanSelectAllDay(localStorage.getItem("canSelectAllDay") === "true");

    return () => {
      mql.removeEventListener("change", matchMode);
    };
  }, []);

  useEffect(() => {
    if (campuses.length === 0) {
      if (selectedCampus !== "") {
        setSelectedCampus("");
        setSelectedBuildings([]);
        setSelectedClassTimes([]);
      }
      return;
    }

    if (!campuses.some((campus) => campus.id === selectedCampus)) {
      const preferred =
        campuses.find((campus) => campus.name === "沙河") || campuses[0];
      setSelectedCampus(preferred.id);
      setSelectedBuildings([]);
      setSelectedClassTimes([]);
    }
  }, [campuses, selectedCampus]);

  return (
    <ConfigProvider
      theme={{
        algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
      }}
    >
      <Spin spinning={spinning}>
        <div className="App">
          <div className="app-header">
            <Title level={3} className="app-title">
              BUPT 今日空教室
            </Title>
          </div>
          {resp.code === 0 && resp.data?.date ? (
            <div className="today-caption">
              {formatDateWithWeekday(resp.data.date)}
            </div>
          ) : null}
          {resp.code === 0 && resp.data?.stale ? (
            <Alert
              className="stale-alert"
              type="warning"
              showIcon
              message={
                resp.data.error?.message ||
                "当前展示的是今天最后一次成功刷新数据"
              }
            />
          ) : null}
          <CampusButtonGroup
            campuses={campuses}
            todayData={resp}
            selectedCampus={selectedCampus}
            setSelectedCampus={setSelectedCampus}
            setSelectedBuildings={setSelectedBuildings}
            setSelectedClassTimes={setSelectedClassTimes}
            showClassTime={showClassTime}
            setShowClassTime={setShowClassTime}
            canSelectAllDay={canSelectAllDay}
            setCanSelectAllDay={setCanSelectAllDay}
          />
          <BuildingPicker
            selectedCampusData={selectedCampusData}
            selectedBuildings={selectedBuildings}
            setSelectedBuildings={setSelectedBuildings}
          />
          <ClassTimePicker
            selectedCampusData={selectedCampusData}
            todayDate={resp.data?.date}
            selectedClassTimes={selectedClassTimes}
            setSelectedClassTimes={setSelectedClassTimes}
            showClassTime={showClassTime}
            canSelectAllDay={canSelectAllDay}
            isDark={isDark}
          />
          <TodayClassroomTable
            selectedCampusData={selectedCampusData}
            selectedBuildings={selectedBuildings}
            selectedClassTimes={selectedClassTimes}
          />
          <GlobalEmpty todayData={resp} isError={isError} onRetry={retry} />
          <Footer />
        </div>
      </Spin>
    </ConfigProvider>
  );
}

export default App;
