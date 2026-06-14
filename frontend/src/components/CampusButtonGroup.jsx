import PropTypes from "prop-types";
import { Button, Divider, Modal, Radio, Switch, Typography } from "antd";
import { useState } from "react";
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

  return (
    <div className="campus-button-group">
      <Radio.Group
        value={selectedCampus}
        onChange={(e) => {
          setSelectedCampus(e.target.value);
          setSelectedBuildings([]);
          setSelectedClassTimes([]);
        }}
        buttonStyle="solid"
        size="middle"
      >
        {campuses.map((campus) => (
          <Radio.Button value={campus.id} key={campus.id}>
            {campus.name}
          </Radio.Button>
        ))}
      </Radio.Group>
      <Button
        icon={<SettingOutlined />}
        onClick={() => setOpenSettingModal(true)}
      />
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
          <div style={{ display: "flex", alignItems: "center" }}>
            <Switch
              checked={showClassTime}
              onChange={(v) => {
                localStorage.setItem("showClassTime", v ? "true" : "false");
                setShowClassTime(v);
              }}
              size="small"
            />
            <Typography.Title level={5} style={{ margin: 8 }}>
              显示课程时间
            </Typography.Title>
          </div>
          <div style={{ display: "flex", alignItems: "center" }}>
            <Switch
              checked={canSelectAllDay}
              onChange={(v) => {
                localStorage.setItem("canSelectAllDay", v ? "true" : "false");
                setCanSelectAllDay(v);
              }}
              size="small"
            />
            <Typography.Title level={5} style={{ margin: 8 }}>
              全选时包含已结束节次
            </Typography.Title>
          </div>
          <Divider />
          <div
            style={{
              lineHeight: "2em",
            }}
          >
            数据来源：教务系统当天空闲教室接口
          </div>
          <div
            style={{
              lineHeight: "2em",
            }}
          >
            当前数据刷新时间：
            {todayData.data?.updated_at
              ? new Date(todayData.data.updated_at).toLocaleString()
              : "未知"}
          </div>
          <div
            style={{
              lineHeight: "2em",
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
