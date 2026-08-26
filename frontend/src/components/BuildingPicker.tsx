import type { CampusInfo } from "../api/types";
import { useSelection } from "../selectionContext";
import { ToggleButtonGroup } from "./ToggleButtonGroup";
import Panel from "./Panel";
import "./BuildingPicker.css";

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
    <Panel className="building-picker responsive-card">
      <ToggleButtonGroup
        options={buildings.map((building) => ({
          value: building.name,
          // Fall back to the raw name while a pre-deploy same-day cache is
          // still serving payloads without the new field.
          label: building.display_name || building.name,
        }))}
        selectedValues={selectedBuildings}
        onToggle={(name) => {
          const next = selectedBuildings.includes(name)
            ? selectedBuildings.filter((x) => x !== name)
            : [...selectedBuildings, name];
          dispatch({ type: "SET_BUILDINGS", buildings: next });
        }}
      />
    </Panel>
  );
}

export default BuildingPicker;
