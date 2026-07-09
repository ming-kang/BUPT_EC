import "./App.css";
import { Alert, ConfigProvider, Spin, Typography, theme } from "antd";
import { Suspense, lazy, useEffect, useMemo, useState } from "react";
import CampusButtonGroup from "./components/CampusButtonGroup";
import BuildingPicker from "./components/BuildingPicker";
import ClassTimePicker from "./components/ClassTimePicker";
import ErrorBoundary from "./components/ErrorBoundary";
import GlobalEmpty from "./components/GlobalEmpty";
import Footer from "./components/Footer";
import useTodayClassrooms from "./useTodayClassrooms";
import SelectionProvider from "./SelectionProvider";
import { useSelection } from "./selectionContext";
import {
  applyDarkClass,
  getSystemPrefersDark,
  resolveDarkMode,
} from "./darkMode";

const TodayClassroomTable = lazy(
  () => import("./components/TodayClassroomTable")
);

const WEEKDAY_LABELS = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];

function formatDateWithWeekday(dateStr) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(dateStr);
  if (!match) return dateStr;
  const [, y, m, d] = match;
  const date = new Date(Number(y), Number(m) - 1, Number(d));
  return `${dateStr} · ${WEEKDAY_LABELS[date.getDay()]}`;
}

function AppContent() {
  const { resp, spinning, isError, retry } = useTodayClassrooms();
  const { state, dispatch } = useSelection();
  const { selectedCampus, selectedBuildings, selectedClassTimes } = state;
  const [isDark, setIsDark] = useState(() =>
    resolveDarkMode(getSystemPrefersDark())
  );

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
      const dark = resolveDarkMode(e.matches);
      applyDarkClass(dark);
      setIsDark(dark);
    }

    mql.addEventListener("change", matchMode);
    matchMode(mql);

    return () => {
      mql.removeEventListener("change", matchMode);
    };
  }, []);

  useEffect(() => {
    if (campuses.length === 0) {
      if (selectedCampus !== "") {
        dispatch({ type: "SET_CAMPUS", id: "" });
      }
      return;
    }

    if (!campuses.some((campus) => campus.id === selectedCampus)) {
      const preferred =
        campuses.find((campus) => campus.name === "沙河") || campuses[0];
      dispatch({ type: "SET_CAMPUS", id: preferred.id });
    }
  }, [campuses, selectedCampus, dispatch]);

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
          {resp.code === 0 && (resp.data?.stale || resp.data?.error) ? (
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
          <CampusButtonGroup campuses={campuses} todayData={resp} />
          <BuildingPicker selectedCampusData={selectedCampusData} />
          <ClassTimePicker
            selectedCampusData={selectedCampusData}
            todayDate={resp.data?.date}
          />
          {selectedCampusData ? (
            <ErrorBoundary
              fallback={
                <Alert
                  type="error"
                  showIcon
                  message="教室列表加载失败，请刷新页面重试"
                />
              }
            >
              <Suspense fallback={null}>
                <TodayClassroomTable
                  selectedCampusData={selectedCampusData}
                  selectedBuildings={selectedBuildings}
                  selectedClassTimes={selectedClassTimes}
                />
              </Suspense>
            </ErrorBoundary>
          ) : null}
          <GlobalEmpty todayData={resp} isError={isError} onRetry={retry} />
          <Footer />
        </div>
      </Spin>
    </ConfigProvider>
  );
}

function App() {
  return (
    <SelectionProvider>
      <AppContent />
    </SelectionProvider>
  );
}

export default App;
