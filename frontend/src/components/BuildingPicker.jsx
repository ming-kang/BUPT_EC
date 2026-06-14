import PropTypes from "prop-types";
import { Button, Card } from "antd";
import "./BuildingPicker.css";

const BUILDING_ALIASES = {
  未来学习大楼: "主楼",
};

function displayBuildingName(name) {
  if (typeof name !== "string") return name;
  const trimmed = name.trim();
  if (BUILDING_ALIASES[trimmed]) return BUILDING_ALIASES[trimmed];
  return /^\d+$/.test(trimmed) ? `教${trimmed}` : name;
}

function BuildingPicker(props) {
  if (!props.selectedCampusData) {
    return null;
  }

  const selectedBuildings = props.selectedBuildings || [];
  const buildings = Array.isArray(props.selectedCampusData.buildings)
    ? props.selectedCampusData.buildings
    : [];

  return (
    <Card className="building-picker responsive-card">
      {buildings.map((building) => (
        <Button
          key={`${props.selectedCampusData.id}-${building.name}`}
          type={selectedBuildings.includes(building.name) ? "primary" : "outline"}
          onClick={() => {
            if (selectedBuildings.includes(building.name)) {
              props.setSelectedBuildings(
                selectedBuildings.filter((x) => x !== building.name)
              );
            } else {
              props.setSelectedBuildings([...selectedBuildings, building.name]);
            }
          }}
        >
          {displayBuildingName(building.name)}
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
