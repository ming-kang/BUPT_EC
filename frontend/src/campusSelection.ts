import type { CampusInfo } from "./api/types";

const PREFERRED_CAMPUS_NAME = "沙河";

function hasCampusSnapshot(campus: CampusInfo): boolean {
  return campus.buildings.length > 0 || campus.nodes.length > 0;
}

interface ChooseCampusIdOptions {
  campuses: CampusInfo[];
  partialCampusIds?: string[];
  selectedCampusId: string;
}

/**
 * Choose a campus from the decoded snapshot:
 * keep a selection with data, else prefer non-partial 沙河 / first non-partial,
 * then the first campus with data, and finally the first placeholder.
 */
export function chooseCampusId({
  campuses,
  partialCampusIds,
  selectedCampusId,
}: ChooseCampusIdOptions): string {
  if (campuses.length === 0) {
    return "";
  }

  const selected = campuses.find((campus) => campus.id === selectedCampusId);
  if (selected && hasCampusSnapshot(selected)) {
    return selected.id;
  }

  const partialIds = new Set(partialCampusIds);
  const nonPartial = campuses.filter((campus) => !partialIds.has(campus.id));
  const preferred =
    nonPartial.find((campus) => campus.name === PREFERRED_CAMPUS_NAME) ??
    nonPartial[0] ??
    campuses.find(hasCampusSnapshot) ??
    campuses[0];
  return preferred.id;
}
