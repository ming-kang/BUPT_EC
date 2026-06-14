import PropTypes from "prop-types";
import { Card, Descriptions, Empty, Modal, Table, Tag } from "antd";
import { useMemo, useState } from "react";
import "./TodayClassroomTable.css";

function TodayClassroomTable(props) {
  const [modalTitle, setModalTitle] = useState("");
  const [modalContent, setModalContent] = useState([]);
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
    const freeTimes = (Array.isArray(room.free_times) ? room.free_times : [])
      .map((item) => `${String(item.node).padStart(2, "0")} ${item.time}`)
      .join("，");
    setModalTitle(room.display_name);
    setModalContent([
      {
        key: "座位数",
        value: room.capacity || "未知",
      },
      {
        key: "空闲节次",
        value: freeTimes,
      },
    ]);
    setOpenModal(true);
  }

  const columns = [
    {
      title: "教室",
      key: "display_name",
      dataIndex: "display_name",
      align: "center",
      render: (_, record) => (
        <button
          type="button"
          className="room-name"
          onClick={() => {
            showClassroomInfo(record);
          }}
        >
          {record.display_name}
        </button>
      ),
    },
    {
      title: "座位数",
      key: "capacity",
      dataIndex: "capacity",
      align: "center",
      render: (capacity) => capacity || "未知",
    },
    {
      title: "空闲节次",
      key: "free_nodes",
      dataIndex: "free_nodes",
      align: "center",
      render: (nodes) => (
        <>
          {(Array.isArray(nodes) ? nodes : [])
            .filter((node) => selectedClassTimes.includes(node))
            .map((node) => (
              <Tag key={node} bordered={false}>
                {String(node).padStart(2, "0")}
              </Tag>
            ))}
        </>
      ),
    },
  ];

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
        <Table
          dataSource={emptyClassrooms}
          columns={columns}
          pagination={false}
          bordered={false}
          tableLayout="auto"
          size="small"
          rowKey={(record) => `${record.building}-${record.display_name}`}
          style={{
            width: "100%",
          }}
          scroll={{ x: true }}
        />
      </Card>
      <Modal
        title={modalTitle}
        open={openModal}
        footer={null}
        onCancel={() => {
          setOpenModal(false);
        }}
      >
        <Descriptions column={1} size="small" layout="vertical">
          {modalContent.map((item, index) => (
            <Descriptions.Item key={index} label={item.key}>
              {item.value}
            </Descriptions.Item>
          ))}
        </Descriptions>
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
