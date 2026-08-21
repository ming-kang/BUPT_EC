import { Button, Card, Empty } from "antd";
import "./GlobalEmpty.css";

import type { HookEnvelope } from "../useTodayClassrooms";

interface GlobalEmptyProps {
  todayData: HookEnvelope;
  isError: boolean;
  onRetry?: () => void;
}

function GlobalEmpty(props: GlobalEmptyProps) {
  if (props.todayData.code === 0) {
    return null;
  }

  return (
    <Card className="global-empty responsive-card compact-card">
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description={
          props.isError ? (
            <>
              {props.todayData.msg ||
                "数据获取失败，请刷新重试，若一直失败，可以点击上方按钮反馈"}
              {props.todayData.logId ? (
                <div className="global-empty__log-id">
                  log_id: {props.todayData.logId}
                </div>
              ) : null}
            </>
          ) : (
            "加载中"
          )
        }
      >
        {props.isError ? <Button onClick={props.onRetry}>重试</Button> : null}
      </Empty>
    </Card>
  );
}

export default GlobalEmpty;
