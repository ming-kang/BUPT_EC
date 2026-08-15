import PropTypes from "prop-types";
import { Card } from "antd";
import { useSelection } from "../selectionContext";
import { ToggleButtonGroup } from "./ToggleButtonGroup";
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
      <ToggleButtonGroup
        options={buildings.map((building) => ({
          value: building.name,
          label: displayBuildingName(building.name),
        }))}
        selectedValues={selectedBuildings}
        onToggle={(name) => {
          const next = selectedBuildings.includes(name)
            ? selectedBuildings.filter((x) => x !== name)
            : [...selectedBuildings, name];
          dispatch({ type: "SET_BUILDINGS", buildings: next });
        }}
      />
    </Card>
  );
}

BuildingPicker.propTypes = {
  selectedCampusData: PropTypes.object,
};

export default BuildingPicker;
