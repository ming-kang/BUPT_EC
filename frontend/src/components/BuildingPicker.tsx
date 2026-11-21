import type { CampusInfo } from "../api/types";
import { useSelection } from "../selectionContext";
import { ToggleButtonGroup } from "./ToggleButtonGroup";
import Panel from "./Panel";
import "./BuildingPicker.css";

interface BuildingPickerProps {
  selectedCampusData?: CampusInfo | null;
}

function BuildingPicker({ selectedCampusData }: BuildingPickerProps) {
  const { selectedBuildings, setBuildings } = useSelection();

  if (!selectedCampusData) {
    return null;
  }

  const { buildings } = selectedCampusData;

  return (
    <Panel className="building-picker responsive-card">
      <ToggleButtonGroup
        options={buildings.map((building) => ({
          value: building.name,
          label: building.display_name,
        }))}
        selectedValues={selectedBuildings}
        onToggle={(name) => {
          const next = selectedBuildings.includes(name)
            ? selectedBuildings.filter((x) => x !== name)
            : [...selectedBuildings, name];
          setBuildings(next);
        }}
      />
    </Panel>
  );
}

export default BuildingPicker;
