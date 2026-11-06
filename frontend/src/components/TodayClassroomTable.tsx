import { Empty, Modal } from "antd";
import { useMemo, useState } from "react";
import type { CampusInfo, RoomInfo } from "../api/types";
import Panel from "./Panel";
import "./TodayClassroomTable.css";

type RoomRecord = RoomInfo & {
  building: string;
};

interface TodayClassroomTableProps {
  /** decode() guarantees the full CampusInfo shape before it reaches here. */
  selectedCampusData?: Pick<CampusInfo, "buildings"> | null;
  selectedBuildings: string[];
  selectedClassTimes: number[];
}

/** Same format as the table row key: display_name can repeat across buildings. */
function roomKey(room: RoomRecord) {
  return `${room.building}-${room.display_name}`;
}

function TodayClassroomTable({
  selectedCampusData,
  selectedBuildings,
  selectedClassTimes,
}: TodayClassroomTableProps) {
  // Holds the opened room's identity (stable key + display name), never its
  // data: capacity and free_times always come from the latest payload below.
  const [openedRoom, setOpenedRoom] = useState<{
    key: string;
    displayName: string;
  } | null>(null);
  const buildings = selectedCampusData?.buildings;
  // One full room list drives both filtering and the open modal.
  const rooms = useMemo(
    () =>
      buildings?.flatMap((building) =>
        building.rooms.map((room) => ({ ...room, building: building.name }))
      ) ?? [],
    [buildings]
  );

  const emptyClassrooms = useMemo(() => {
    if (selectedClassTimes.length === 0) {
      return [];
    }
    return rooms
      .filter(
        (room) =>
          selectedBuildings.includes(room.building) &&
          selectedClassTimes.every((node) => room.free_nodes.includes(node))
      )
      .sort((a, b) => a.display_name.localeCompare(b.display_name));
  }, [rooms, selectedBuildings, selectedClassTimes]);

  // Use the full list so refreshes update an open modal even when its room
  // no longer matches the current building/time filters.
  const activeRoom = useMemo(
    () => rooms.find((room) => roomKey(room) === openedRoom?.key),
    [rooms, openedRoom]
  );

  if (!selectedCampusData) {
    return null;
  }

  const activeFreeTimes = activeRoom?.free_times ?? [];

  // Rendered by BOTH branches below: the modal must survive the empty-state
  // early return, otherwise a background refresh that empties the table
  // unmounts an open modal (and it pops back once rows return). When the
  // room itself disappears from the data the modal stays open and the empty
  // free-times fallback row takes over.
  const roomInfoModal = (
    <Modal
      title={activeRoom?.display_name ?? openedRoom?.displayName ?? ""}
      open={openedRoom != null}
      footer={null}
      onCancel={() => {
        setOpenedRoom(null);
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

  const showEmptyState = emptyClassrooms.length === 0;

  // Single root for both branches: the Panel and Modal keep their positions
  // in the element tree, so flipping between empty state and table never
  // remounts an open modal (a remount would detach its portal mid-view).
  return (
    <div className="today-classroom-table">
      {showEmptyState ? (
        <Panel className="responsive-card compact-card">
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
        </Panel>
      ) : (
        <Panel
          className="responsive-card"
          bodyStyle={{ padding: 0 }}
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
                          setOpenedRoom({
                            key: roomKey(record),
                            displayName: record.display_name,
                          });
                        }}
                      >
                        {record.display_name}
                      </button>
                    </td>
                    <td>{record.capacity || "未知"}</td>
                    <td>
                      {record.free_nodes
                        .filter((node) => selectedClassTimes.includes(node))
                        .map((node) => (
                          <span key={node} className="node-tag">
                            {String(node).padStart(2, "0")}
                          </span>
                        ))}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
      )}
      {roomInfoModal}
    </div>
  );
}

export default TodayClassroomTable;
