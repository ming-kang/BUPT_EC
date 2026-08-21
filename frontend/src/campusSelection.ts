/** Preferred default campus name when multiple usable campuses exist. */
export const PREFERRED_CAMPUS_NAME = "沙河";

/**
 * Minimal structural view of a campus payload. Real payloads come from the
 * (validated) API types, but these helpers deliberately tolerate partial or
 * malformed shapes, so fields stay `unknown` and are guarded at runtime.
 */
export interface CampusLike {
  id?: unknown;
  name?: unknown;
  buildings?: unknown;
  nodes?: unknown;
}

/**
 * True when a campus payload carries buildings or nodes that can drive the UI.
 * Empty placeholders used for cold partial failures return false.
 */
export function hasCampusSnapshot(campus: unknown): boolean {
  const view = campus as CampusLike | null | undefined;
  if (!view || typeof view !== "object") {
    return false;
  }
  const buildings = Array.isArray(view.buildings) ? view.buildings : [];
  const nodes = Array.isArray(view.nodes) ? view.nodes : [];
  return buildings.length > 0 || nodes.length > 0;
}

function normalizePartialIds(partialCampusIds: unknown): Set<string> {
  if (!Array.isArray(partialCampusIds)) {
    return new Set();
  }
  return new Set(
    partialCampusIds
      .filter((value) => typeof value === "string" || typeof value === "number")
      .map((value) => String(value).trim())
      .filter(Boolean)
  );
}

function isPartialCampus(campus: CampusLike, partialIds: Set<string>): boolean {
  if (!campus) {
    return false;
  }
  return partialIds.has(String(campus.id));
}

export interface ChooseCampusIdOptions {
  campuses?: CampusLike[] | null;
  partialCampusIds?: unknown;
  selectedCampusId?: unknown;
}

/**
 * Choose which campus ID the UI should select for the current payload.
 *
 * Rules:
 * 1. Keep the current selection when it still has snapshot data (even if partial).
 * 2. Prefer non-partial 沙河, else the first non-partial campus.
 * 3. Among all-partial payloads, pick the first campus that still has snapshot data.
 * 4. Stable fallback to the first campus ID; empty list returns "".
 */
export function chooseCampusId({
  campuses,
  partialCampusIds,
  selectedCampusId,
}: ChooseCampusIdOptions = {}): string {
  const list = Array.isArray(campuses) ? campuses : [];
  if (list.length === 0) {
    return "";
  }

  const partialIds = normalizePartialIds(partialCampusIds);
  const selected = list.find(
    (campus) => String(campus?.id) === String(selectedCampusId ?? "")
  );

  if (selected && hasCampusSnapshot(selected)) {
    return String(selected.id);
  }

  const nonPartial = list.filter((campus) => !isPartialCampus(campus, partialIds));
  if (nonPartial.length > 0) {
    const preferred =
      nonPartial.find((campus) => campus?.name === PREFERRED_CAMPUS_NAME) ||
      nonPartial[0];
    return preferred?.id != null ? String(preferred.id) : "";
  }

  const withSnapshot = list.find((campus) => hasCampusSnapshot(campus));
  if (withSnapshot?.id != null) {
    return String(withSnapshot.id);
  }

  return list[0]?.id != null ? String(list[0].id) : "";
}
