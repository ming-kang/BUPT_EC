import PropTypes from "prop-types";
import { Button, Divider, Modal, Switch, Typography } from "antd";
import { GithubOutlined } from "@ant-design/icons";

function CampusSettingsModal(props) {
  return (
    <Modal
      title="设置"
      open={props.open}
      closable={true}
      footer={null}
      onCancel={props.onClose}
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
            checked={props.showClassTime}
            onChange={(v) => {
              localStorage.setItem("showClassTime", v ? "true" : "false");
              props.setShowClassTime(v);
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
            checked={props.canSelectAllDay}
            onChange={(v) => {
              localStorage.setItem("canSelectAllDay", v ? "true" : "false");
              props.setCanSelectAllDay(v);
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
          {props.todayData.data?.updated_at
            ? new Date(props.todayData.data.updated_at).toLocaleString()
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
  );
}

CampusSettingsModal.propTypes = {
  open: PropTypes.bool.isRequired,
  todayData: PropTypes.object.isRequired,
  showClassTime: PropTypes.bool,
  setShowClassTime: PropTypes.func,
  canSelectAllDay: PropTypes.bool,
  setCanSelectAllDay: PropTypes.func,
  onClose: PropTypes.func,
};

export default CampusSettingsModal;
