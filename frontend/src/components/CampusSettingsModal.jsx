import PropTypes from "prop-types";
import { Button, Divider, Modal, Switch, Typography } from "antd";
import { GithubOutlined } from "@ant-design/icons";
import { useSelection } from "../selectionContext";

function CampusSettingsModal(props) {
  const { state, dispatch } = useSelection();

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
            checked={state.showClassTime}
            onChange={(v) => dispatch({ type: "SET_SHOW_CLASS_TIME", value: v })}
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
            checked={state.canSelectAllDay}
            onChange={(v) =>
              dispatch({ type: "SET_CAN_SELECT_ALL_DAY", value: v })
            }
            size="small"
          />
          <Typography.Text>允许选择已结束节次</Typography.Text>
        </div>
        <Divider />
        <Typography.Text type="secondary" style={{ display: "block", lineHeight: "1.9em" }}>
          数据来源：教务系统当天空闲教室接口
        </Typography.Text>
        <Typography.Text type="secondary" style={{ display: "block", lineHeight: "1.9em" }}>
          最近刷新尝试时间：
          {props.todayData.data?.updated_at
            ? new Date(props.todayData.data.updated_at).toLocaleString()
            : "未知"}
        </Typography.Text>
        <div
          style={{
            lineHeight: "1.9em",
            display: "flex",
            alignItems: "center",
            gap: 4,
          }}
        >
          <Typography.Text type="secondary">项目已开源：</Typography.Text>
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
  onClose: PropTypes.func,
};

export default CampusSettingsModal;
