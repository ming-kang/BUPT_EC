import PropTypes from "prop-types";
import { Button, Divider, Modal, Switch, Typography } from "antd";
import { Fragment, useState } from "react";
import { GithubOutlined, SettingOutlined } from "@ant-design/icons";
import "./CampusButtonGroup.css";

function CampusButtonGroup(props) {
  const {
    campuses,
    todayData,
    selectedCampus,
    setSelectedCampus,
    setSelectedBuildings,
    setSelectedClassTimes,
    showClassTime,
    setShowClassTime,
    canSelectAllDay,
    setCanSelectAllDay,
  } = props;
  const [openSettingModal, setOpenSettingModal] = useState(false);
  const settingsSplitIndex = Math.floor(campuses.length / 2);

  return (
    <div className="campus-button-group">
      <div className="campus-buttons">
        {campuses.map((campus, index) => (
          <Fragment key={campus.id}>
            {index === settingsSplitIndex ? (
              <Button
                className="settings-trigger"
                icon={<SettingOutlined />}
                onClick={() => setOpenSettingModal(true)}
                aria-label="设置"
              />
            ) : null}
            <Button
              type={selectedCampus === campus.id ? "primary" : "default"}
              onClick={() => {
                setSelectedCampus(campus.id);
                setSelectedBuildings([]);
                setSelectedClassTimes([]);
              }}
            >
              {campus.name}
            </Button>
          </Fragment>
        ))}
      </div>
      <Modal
        title="设置"
        open={openSettingModal}
        closable={true}
        footer={null}
        onCancel={() => {
          setOpenSettingModal(false);
        }}
      >
        <div>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 8,
              marginBottom: 12,
            }}
          >
            <Switch
              checked={showClassTime}
              onChange={(v) => {
                localStorage.setItem("showClassTime", v ? "true" : "false");
                setShowClassTime(v);
              }}
              size="small"
            />
            <Typography.Text>显示课程时间</Typography.Text>
          </div>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 8,
              marginBottom: 12,
            }}
          >
            <Switch
              checked={canSelectAllDay}
              onChange={(v) => {
                localStorage.setItem("canSelectAllDay", v ? "true" : "false");
                setCanSelectAllDay(v);
              }}
              size="small"
            />
            <Typography.Text>全选时包含已结束节次</Typography.Text>
          </div>
          <Divider />
          <div style={{ lineHeight: "1.9em", color: "rgba(0,0,0,0.65)" }}>
            数据来源：教务系统当天空闲教室接口
          </div>
          <div style={{ lineHeight: "1.9em", color: "rgba(0,0,0,0.65)" }}>
            当前数据刷新时间：
            {todayData.data?.updated_at
              ? new Date(todayData.data.updated_at).toLocaleString()
              : "未知"}
          </div>
          <div
            style={{
              lineHeight: "1.9em",
              color: "rgba(0,0,0,0.65)",
              display: "flex",
              alignItems: "center",
              gap: 4,
            }}
          >
            项目已开源：
            <Button
              onClick={() =>
                window.open(
                  "https://github.com/ming-kang/BUPT_EC",
                  "_blank",
                  "noopener,noreferrer"
                )
              }
              icon={<GithubOutlined />}
              size="small"
              type="link"
            >
              Github
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}

CampusButtonGroup.propTypes = {
  campuses: PropTypes.arrayOf(
    PropTypes.shape({
      id: PropTypes.string.isRequired,
      name: PropTypes.string.isRequired,
    })
  ).isRequired,
  todayData: PropTypes.object.isRequired,
  selectedCampus: PropTypes.string,
  setSelectedCampus: PropTypes.func,
  setSelectedBuildings: PropTypes.func,
  setSelectedClassTimes: PropTypes.func,
  showClassTime: PropTypes.bool,
  setShowClassTime: PropTypes.func,
  canSelectAllDay: PropTypes.bool,
  setCanSelectAllDay: PropTypes.func,
};

export default CampusButtonGroup;
