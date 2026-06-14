import "./App.css";
import logo from "./assets/logo.png";
import { Alert, ConfigProvider, Spin, Typography, theme } from "antd";
import { useEffect, useState } from "react";
import CampusButtonGroup from "./components/CampusButtonGroup";
import BuildingPicker from "./components/BuildingPicker";
import ClassTimePicker from "./components/ClassTimePicker";
import EmptyClassroomTable from "./components/EmptyClassroomTable";
import GlobalEmpty from "./components/GlobalEmpty";
import Footer from "./components/Footer";

function App() {
  const [spining, setSpining] = useState(true);
  const [isError, setIsError] = useState(false);
  const [resp, setResp] = useState({ code: 1, msg: "加载中" });
  const [selectedCampus, setSelectedCampus] = useState("");
  const [selectedBuildings, setSelectedBuildings] = useState([]);
  const [selectedClassTimes, setSelectedClassTimes] = useState([]);
  const [showClassTime, setShowClassTime] = useState(false);
  const [canSelectAllDay, setCanSelectAllDay] = useState(false);
  const [isDark, setIsDark] = useState(false);

  const { Title } = Typography;

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

    fetch("/api/get_data")
      .then((response) => response.json())
      .then((data) => {
        setResp(data);
        setIsError(data.code !== 0);
        setSpining(false);
      })
      .catch(() => {
        setIsError(true);
        setResp({ code: 500, msg: "数据获取失败，请稍后重试" });
        setSpining(false);
      });

    setShowClassTime(localStorage.getItem("showClassTime") !== "false");
    setCanSelectAllDay(localStorage.getItem("canSelectAllDay") === "true");

    return () => {
      mql.removeEventListener("change", matchMode);
    };
  }, []);

  return (
    <ConfigProvider
      theme={{
        algorithm:
          localStorage.getItem("darkMode") === "true"
            ? theme.darkAlgorithm
            : theme.defaultAlgorithm,
      }}
    >
      <Spin spinning={spining}>
        <div className="App">
          <img src={logo} className="logo" alt="BUPT" />
          <Title
            level={3}
            style={{
              marginBottom: "6px",
            }}
          >
            BUPT 今日空教室
          </Title>
          {resp.code === 0 && resp.data?.date ? (
            <div className="today-caption">
              {resp.data.date} · 教务系统当天数据
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
            todayData={resp}
            selectedBuildings={selectedBuildings}
            setSelectedBuildings={setSelectedBuildings}
            selectedCampus={selectedCampus}
          />
          <ClassTimePicker
            todayData={resp}
            selectedClassTimes={selectedClassTimes}
            setSelectedClassTimes={setSelectedClassTimes}
            selectedCampus={selectedCampus}
            showClassTime={showClassTime}
            canSelectAllDay={canSelectAllDay}
            isDark={isDark}
          />
          <EmptyClassroomTable
            todayData={resp}
            selectedCampus={selectedCampus}
            selectedBuildings={selectedBuildings}
            selectedClassTimes={selectedClassTimes}
            setIsError={setIsError}
          />
          <GlobalEmpty todayData={resp} isError={isError} />
          <Footer />
        </div>
      </Spin>
    </ConfigProvider>
  );
}

export default App;
