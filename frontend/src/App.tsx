import "./App.css";
import { Alert, ConfigProvider, Spin, theme } from "antd";
import { Suspense, lazy, useEffect, useMemo, useState } from "react";
import CampusButtonGroup from "./components/CampusButtonGroup";
import BuildingPicker from "./components/BuildingPicker";
import ClassTimePicker from "./components/ClassTimePicker";
import ErrorBoundary from "./components/ErrorBoundary";
import GlobalEmpty from "./components/GlobalEmpty";
import Footer from "./components/Footer";
import useTodayClassrooms from "./useTodayClassrooms";
import { classroomWarningMessage } from "./todayClassroomsResponse";
import SelectionProvider from "./SelectionProvider";
import { useSelection } from "./selectionContext";
import {
  applyDarkClass,
  getSystemPrefersDark,
  resolveDarkMode,
} from "./darkMode";
import { chooseCampusId } from "./campusSelection";

const TodayClassroomTable = lazy(
  () => import("./components/TodayClassroomTable")
);

const WEEKDAY_LABELS: string[] = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];

function formatDateWithWeekday(dateStr: string): string {
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

  const campuses = useMemo(
    () =>
      resp.code === 0 && Array.isArray(resp.data?.campuses)
        ? resp.data.campuses
        : [],
    [resp]
  );
  // Derive the effective campus during render so the first data frame
  // already shows the selected campus and its cards (no post-effect blank
  // frame). The effect below only reconciles the store, whose reducer also
  // resets buildings/class times on campus change.
  const activeCampusId = chooseCampusId({
    campuses,
    partialCampusIds: resp.data?.partial_campuses,
    selectedCampusId: selectedCampus,
  });
  const selectedCampusData = useMemo(
    () => campuses.find((campus) => campus.id === activeCampusId) || null,
    [campuses, activeCampusId]
  );

  useEffect(() => {
    const mql = window.matchMedia("(prefers-color-scheme: dark)");

    function matchMode(e: { matches: boolean }) {
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
    if (activeCampusId !== selectedCampus) {
      dispatch({ type: "SET_CAMPUS", id: activeCampusId });
    }
  }, [activeCampusId, selectedCampus, dispatch]);

  return (
    <ConfigProvider
      theme={{
        algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
      }}
    >
      <Spin spinning={spinning}>
        <div className="App">
          <div className="app-header">
            <h3 className="app-title">
              BUPT 今日空教室
            </h3>
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
              message={classroomWarningMessage(resp.data)}
            />
          ) : null}
          <CampusButtonGroup
            campuses={campuses}
            todayData={resp}
            activeCampusId={activeCampusId}
          />
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
