import { Card } from "antd";
import type { CampusInfo } from "../api/types";
import { useSelection } from "../selectionContext";
import { ToggleButtonGroup } from "./ToggleButtonGroup";
import "./BuildingPicker.css";

const BUILDING_ALIASES: Record<string, string> = {
  未来学习大楼: "主楼",
};

function displayBuildingName(name: string): string {
  if (typeof name !== "string") return name;
  const trimmed = name.trim();
  if (BUILDING_ALIASES[trimmed]) return BUILDING_ALIASES[trimmed];
  return /^\d+$/.test(trimmed) ? `教${trimmed}` : name;
}

interface BuildingPickerProps {
  selectedCampusData?: CampusInfo | null;
}

function BuildingPicker({ selectedCampusData }: BuildingPickerProps) {
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

export default BuildingPicker;
