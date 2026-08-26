import { Button, Divider, Modal, Switch } from "antd";
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
          <span>显示课程时间</span>
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
          <span>允许选择已结束节次</span>
        </div>
        <Divider />
        <span className="text-secondary" style={{ display: "block", lineHeight: "1.9em" }}>
          数据来源：教务系统当天空闲教室接口
        </span>
        <span className="text-secondary" style={{ display: "block", lineHeight: "1.9em" }}>
          当前数据更新时间：
          {props.todayData.data?.updated_at
            ? formatShanghaiDateTime(new Date(props.todayData.data.updated_at))
            : "未知"}
        </span>
        {/* Omit the row entirely when the server sent no version (pre-0.3.0
            backend): an empty "未知" tells the user nothing, and an upgrade
            makes the row appear on its own. */}
        {props.todayData.version ? (
          <span className="text-secondary" style={{ display: "block", lineHeight: "1.9em" }}>
            当前运行版本：{props.todayData.version}
          </span>
        ) : null}
        <div
          style={{
            lineHeight: "1.9em",
            display: "flex",
            alignItems: "center",
            gap: 4,
          }}
        >
          <span className="text-secondary">项目已开源：</span>
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
