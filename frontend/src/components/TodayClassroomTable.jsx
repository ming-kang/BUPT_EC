import PropTypes from "prop-types";
import { Card, Empty, Modal, Tag } from "antd";
import { useMemo, useState } from "react";
import "./TodayClassroomTable.css";

function TodayClassroomTable(props) {
  const [modalTitle, setModalTitle] = useState("");
  const [modalCapacity, setModalCapacity] = useState("");
  const [modalFreeTimes, setModalFreeTimes] = useState([]);
  const [openModal, setOpenModal] = useState(false);
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

  if (!props.selectedCampusData) {
    return null;
  }

  if (
    selectedBuildings.length === 0 ||
    selectedClassTimes.length === 0 ||
    emptyClassrooms.length === 0
  ) {
    return (
      <Card className="today-classroom-table responsive-card compact-card">
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
    );
  }

  function showClassroomInfo(room) {
    setModalTitle(room.display_name);
    setModalCapacity(room.capacity || "未知");
    setModalFreeTimes(
      Array.isArray(room.free_times) ? room.free_times : []
    );
    setOpenModal(true);
  }

  return (
    <div className="today-classroom-table">
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
                <tr key={`${record.building}-${record.display_name}`}>
                  <td>
                    <button
                      type="button"
                      className="room-name"
                      onClick={() => {
                        showClassroomInfo(record);
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
      <Modal
        title={modalTitle}
        open={openModal}
        footer={null}
        onCancel={() => {
          setOpenModal(false);
        }}
      >
        <div className="room-info">
          <div className="room-info__capacity">
            <span className="room-info__capacity-label">座位数：</span>
            <span className="room-info__capacity-value">{modalCapacity}</span>
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
                {modalFreeTimes.length === 0 ? (
                  <tr>
                    <td colSpan={2} className="room-info__empty">
                      暂无空闲节次
                    </td>
                  </tr>
                ) : (
                  modalFreeTimes.map((item) => (
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
    </div>
  );
}

TodayClassroomTable.propTypes = {
  selectedCampusData: PropTypes.object,
  selectedBuildings: PropTypes.array,
  selectedClassTimes: PropTypes.array,
};

export default TodayClassroomTable;
