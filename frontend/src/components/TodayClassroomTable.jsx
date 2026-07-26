import PropTypes from "prop-types";
import { Card, Empty, Modal, Tag } from "antd";
import { useMemo, useState } from "react";
import "./TodayClassroomTable.css";

/** Same format as the table row key: display_name can repeat across buildings. */
function roomKey(room) {
  return `${room.building}-${room.display_name}`;
}

function TodayClassroomTable(props) {
  const [activeRoomKey, setActiveRoomKey] = useState(null);
  const selectedBuildings = useMemo(
    () => (Array.isArray(props.selectedBuildings) ? props.selectedBuildings : []),
    [props.selectedBuildings]
  );
  const selectedClassTimes = useMemo(
    () =>
      Array.isArray(props.selectedClassTimes) ? props.selectedClassTimes : [],
    [props.selectedClassTimes]
  );

  const buildings = useMemo(
    () =>
      Array.isArray(props.selectedCampusData?.buildings)
        ? props.selectedCampusData.buildings
        : [],
    [props.selectedCampusData]
  );

  const emptyClassrooms = useMemo(() => {
    if (
      !props.selectedCampusData ||
      selectedBuildings.length === 0 ||
      selectedClassTimes.length === 0
    ) {
      return [];
    }

    return buildings
      .filter((building) => selectedBuildings.includes(building.name))
      .flatMap((building) =>
        (Array.isArray(building.rooms) ? building.rooms : []).map((room) => ({
          ...room,
          building: building.name,
        }))
      )
      .filter((room) =>
        selectedClassTimes.every((node) =>
          Array.isArray(room.free_nodes)
            ? room.free_nodes.includes(node)
            : false
        )
      )
      .sort((a, b) => a.display_name.localeCompare(b.display_name));
  }, [
    buildings,
    props.selectedCampusData,
    selectedBuildings,
    selectedClassTimes,
  ]);

  // Derive the modal's room during render from the FULL buildings list (not
  // the filtered emptyClassrooms) so background refreshes update an open
  // modal and building/time filter changes cannot blank it out.
  const activeRoom = useMemo(() => {
    if (activeRoomKey == null) {
      return null;
    }
    return (
      buildings
        .flatMap((building) =>
          (Array.isArray(building.rooms) ? building.rooms : []).map((room) => ({
            ...room,
            building: building.name,
          }))
        )
        .find((room) => roomKey(room) === activeRoomKey) || null
    );
  }, [buildings, activeRoomKey]);

  if (!props.selectedCampusData) {
    return null;
  }

  const activeFreeTimes = Array.isArray(activeRoom?.free_times)
    ? activeRoom.free_times
    : [];

  // Rendered by BOTH branches below: the modal must survive the empty-state
  // early return, otherwise a background refresh that empties the table
  // unmounts an open modal (and it pops back once rows return). When the
  // room itself disappears from the data the modal stays open and the empty
  // free-times fallback row takes over.
  const roomInfoModal = (
    <Modal
      title={activeRoom?.display_name ?? ""}
      open={activeRoomKey != null}
      footer={null}
      onCancel={() => {
        setActiveRoomKey(null);
      }}
    >
      <div className="room-info">
        <div className="room-info__capacity">
          <span className="room-info__capacity-label">座位数：</span>
          <span className="room-info__capacity-value">
            {activeRoom ? activeRoom.capacity || "未知" : "—"}
          </span>
        </div>
        <div className="room-info__section-title">空闲节次</div>
        <div className="ec-table-wrap">
          <table className="ec-table">
            <thead>
              <tr>
                <th className="room-info__col-node">节次</th>
                <th className="room-info__col-time">时间</th>
              </tr>
            </thead>
            <tbody>
              {activeFreeTimes.length === 0 ? (
                <tr>
                  <td colSpan={2} className="room-info__empty">
                    暂无空闲节次
                  </td>
                </tr>
              ) : (
                activeFreeTimes.map((item) => (
                  <tr key={item.node}>
                    <td className="room-info__col-node">
                      {String(item.node).padStart(2, "0")}
                    </td>
                    <td className="room-info__col-time">{item.time}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </Modal>
  );

  const showEmptyState =
    selectedBuildings.length === 0 ||
    selectedClassTimes.length === 0 ||
    emptyClassrooms.length === 0;

  // Single root for both branches: the Card and Modal keep their positions
  // in the element tree, so flipping between empty state and table never
  // remounts an open modal (a remount would detach its portal mid-view).
  return (
    <div className="today-classroom-table">
      {showEmptyState ? (
        <Card className="responsive-card compact-card">
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              selectedBuildings.length === 0
                ? selectedClassTimes.length === 0
                  ? "请选择教学楼和上课时间"
                  : "请选择教学楼"
                : selectedClassTimes.length === 0
                ? "请选择上课时间"
                : "没有符合条件的空教室"
            }
          />
        </Card>
      ) : (
        <Card
          className="responsive-card"
          styles={{
            body: {
              padding: 0,
            },
          }}
        >
          <div className="ec-table-wrap">
            <table className="ec-table">
              <thead>
                <tr>
                  <th scope="col">教室</th>
                  <th scope="col">座位数</th>
                  <th scope="col">空闲节次</th>
                </tr>
              </thead>
              <tbody>
                {emptyClassrooms.map((record) => (
                  <tr key={roomKey(record)}>
                    <td>
                      <button
                        type="button"
                        className="room-name"
                        onClick={() => {
                          setActiveRoomKey(roomKey(record));
                        }}
                      >
                        {record.display_name}
                      </button>
                    </td>
                    <td>{record.capacity || "未知"}</td>
                    <td>
                      {(Array.isArray(record.free_nodes)
                        ? record.free_nodes
                        : []
                      )
                        .filter((node) => selectedClassTimes.includes(node))
                        .map((node) => (
                          <Tag key={node} bordered={false}>
                            {String(node).padStart(2, "0")}
                          </Tag>
                        ))}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}
      {roomInfoModal}
    </div>
  );
}

TodayClassroomTable.propTypes = {
  selectedCampusData: PropTypes.object,
  selectedBuildings: PropTypes.array,
  selectedClassTimes: PropTypes.array,
};

export default TodayClassroomTable;
