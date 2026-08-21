import { Button, Divider, Modal, Switch, Typography } from "antd";
import type { Envelope } from "../todayClassroomsResponse";
import { GithubIcon } from "./icons";
import { formatShanghaiDateTime } from "../classTimeUtils";
import { useSelection } from "../selectionContext";

interface CampusSettingsModalProps {
  open: boolean;
  todayData: Envelope;
  onClose?: () => void;
}

function CampusSettingsModal(props: CampusSettingsModalProps) {
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
          当前数据更新时间：
          {props.todayData.data?.updated_at
            ? formatShanghaiDateTime(new Date(props.todayData.data.updated_at))
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
            icon={<GithubIcon />}
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

export default CampusSettingsModal;
