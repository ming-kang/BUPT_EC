import PropTypes from "prop-types";
import { Button, Card, Empty } from "antd";
import "./GlobalEmpty.css";

function GlobalEmpty(props) {
  if (props.todayData.code === 0) {
    return null;
  }

  return (
    <Card className="global-empty responsive-card compact-card">
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description={
          props.isError
            ? props.todayData.msg ||
              "数据获取失败，请刷新重试，若一直失败，可以点击上方按钮反馈"
            : "加载中"
        }
      >
        {props.isError ? <Button onClick={props.onRetry}>重试</Button> : null}
      </Empty>
    </Card>
  );
}

GlobalEmpty.propTypes = {
  todayData: PropTypes.object.isRequired,
  isError: PropTypes.bool.isRequired,
  onRetry: PropTypes.func,
};

export default GlobalEmpty;
