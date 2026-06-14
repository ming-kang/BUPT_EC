import PropTypes from "prop-types";
import { Button, Card } from "antd";
import "./BuildingPicker.css";

function BuildingPicker(props) {
  if (!props.selectedCampusData) {
    return null;
  }

  const selectedBuildings = props.selectedBuildings || [];
  const buildings = Array.isArray(props.selectedCampusData.buildings)
    ? props.selectedCampusData.buildings
    : [];

  return (
    <Card
      style={{
        boxShadow: "0 12px 32px 4px #0000000a, 0 8px 20px #00000014",
      }}
      className="building-picker responsive-card"
    >
      {buildings.map((building) => (
        <Button
          key={`${props.selectedCampusData.id}-${building.name}`}
          type={
            selectedBuildings.includes(building.name)
              ? "primary"
              : "outline"
          }
          onClick={() => {
            if (selectedBuildings.includes(building.name)) {
              props.setSelectedBuildings(
                selectedBuildings.filter((x) => x !== building.name)
              );
            } else {
              props.setSelectedBuildings([
                ...selectedBuildings,
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
  selectedCampusData: PropTypes.object,
  selectedBuildings: PropTypes.array,
  setSelectedBuildings: PropTypes.func,
};

export default BuildingPicker;
