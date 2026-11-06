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
          // decode() guarantees display_name (falling back to name), so the
          // pre-0.3.x missing-field case never reaches this point.
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
