import PropTypes from "prop-types";
import { Card, Descriptions, Empty, Modal, Table, Button, Tag } from "antd";
import { useMemo, useState } from "react";
import "./EmptyClassroomTable.css";

function EmptyClassroomTable(props) {
  const [modalTitle, setModalTitle] = useState("");
  const [modalContent, setModalContent] = useState([]);
  const [openModal, setOpenModal] = useState(false);

  const campus = useMemo(() => {
    if (props.todayData.code !== 0) {
      return null;
    }
    return props.todayData.data?.campuses?.find(
      (item) => item.id === props.selectedCampus
    );
  }, [props.selectedCampus, props.todayData.code, props.todayData.data]);

  const emptyClassrooms = useMemo(() => {
    if (
      !campus ||
      props.selectedBuildings.length === 0 ||
      props.selectedClassTimes.length === 0
    ) {
      return [];
    }

    return campus.buildings
      .filter((building) => props.selectedBuildings.includes(building.name))
      .flatMap((building) =>
        building.rooms.map((room) => ({
          ...room,
          building: building.name,
        }))
      )
      .filter((room) =>
        props.selectedClassTimes.every((node) => room.free_nodes.includes(node))
      )
      .sort((a, b) => a.display_name.localeCompare(b.display_name));
  }, [campus, props.selectedBuildings, props.selectedClassTimes]);

  if (props.todayData.code !== 0 || props.selectedCampus === "") {
    return null;
  }

  if (
    props.selectedBuildings.length === 0 ||
    props.selectedClassTimes.length === 0 ||
    emptyClassrooms.length === 0
  ) {
    return (
      <Card
        className="empty-classroom-table responsive-card compact-card"
        style={{
          boxShadow: "0 12px 32px 4px #0000000a, 0 8px 20px #00000014",
        }}
        bodyStyle={{}}
      >
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={
            props.selectedBuildings.length === 0
              ? props.selectedClassTimes.length === 0
                ? "请选择教学楼和上课时间"
                : "请选择教学楼"
              : props.selectedClassTimes.length === 0
              ? "请选择上课时间"
              : "没有符合条件的空教室"
          }
        />
      </Card>
    );
  }

  function showClassroomInfo(room) {
    const freeTimes = room.free_times
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
        <span style={{ display: "flex", justifyContent: "center" }}>
          <Button
            size="small"
            onClick={() => {
              showClassroomInfo(record);
            }}
          >
            {record.display_name}
          </Button>
        </span>
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
          {nodes
            .filter((node) => props.selectedClassTimes.includes(node))
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
    <div className="empty-classroom-table">
      <Card
        className="responsive-card"
        style={{
          boxShadow: "0 12px 32px 4px #0000000a, 0 8px 20px #00000014",
        }}
        bodyStyle={{
          padding: "0px",
        }}
      >
        <Table
          dataSource={emptyClassrooms}
          columns={columns}
          pagination={false}
          bordered={false}
          tableLayout="auto"
          size="small"
          rowKey={(record) => record.display_name}
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

EmptyClassroomTable.propTypes = {
  todayData: PropTypes.object,
  selectedCampus: PropTypes.string,
  selectedBuildings: PropTypes.array,
  selectedClassTimes: PropTypes.array,
  setIsError: PropTypes.func,
};

export default EmptyClassroomTable;
