import PropTypes from "prop-types";
import { Button, Card } from "antd";
import "./BuildingPicker.css";

function BuildingPicker(props) {
  if (props.todayData.code !== 0 || props.selectedCampus === "") {
    return null;
  }

  const campus = props.todayData.data?.campuses?.find(
    (item) => item.id === props.selectedCampus
  );
  const buildings = campus?.buildings || [];

  return (
    <Card
      style={{
        boxShadow: "0 12px 32px 4px #0000000a, 0 8px 20px #00000014",
      }}
      className="building-picker responsive-card"
      bodyStyle={{}}
    >
      {buildings.map((building) => (
        <Button
          key={props.selectedCampus + building.name}
          type={
            props.selectedBuildings.includes(building.name)
              ? "primary"
              : "outline"
          }
          onClick={() => {
            if (props.selectedBuildings.includes(building.name)) {
              props.setSelectedBuildings(
                props.selectedBuildings.filter((x) => x !== building.name)
              );
            } else {
              props.setSelectedBuildings([
                ...props.selectedBuildings,
                building.name,
              ]);
            }
          }}
          style={{
            borderRadius: "0px",
          }}
        >
          {building.name}
        </Button>
      ))}
    </Card>
  );
}

BuildingPicker.propTypes = {
  todayData: PropTypes.object,
  selectedBuildings: PropTypes.array,
  setSelectedBuildings: PropTypes.func,
  selectedCampus: PropTypes.string,
};

export default BuildingPicker;
