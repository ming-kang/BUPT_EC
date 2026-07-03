import PropTypes from "prop-types";
import { Button, Card } from "antd";
import { useSelection } from "../selectionContext";
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

function BuildingPicker({ selectedCampusData }) {
  const { state, dispatch } = useSelection();

  if (!selectedCampusData) {
    return null;
  }

  const selectedBuildings = state.selectedBuildings || [];
  const buildings = Array.isArray(selectedCampusData.buildings)
    ? selectedCampusData.buildings
    : [];

  return (
    <Card className="building-picker responsive-card">
      {buildings.map((building) => (
        <Button
          key={`${selectedCampusData.id}-${building.name}`}
          type={selectedBuildings.includes(building.name) ? "primary" : "default"}
          onClick={() => {
            const next = selectedBuildings.includes(building.name)
              ? selectedBuildings.filter((x) => x !== building.name)
              : [...selectedBuildings, building.name];
            dispatch({ type: "SET_BUILDINGS", buildings: next });
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
};

export default BuildingPicker;
