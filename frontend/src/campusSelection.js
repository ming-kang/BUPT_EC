/** Preferred default campus name when multiple usable campuses exist. */
export const PREFERRED_CAMPUS_NAME = "沙河";

/**
 * True when a campus payload carries buildings or nodes that can drive the UI.
 * Empty placeholders used for cold partial failures return false.
 */
export function hasCampusSnapshot(campus) {
  if (!campus || typeof campus !== "object") {
    return false;
  }
  const buildings = Array.isArray(campus.buildings) ? campus.buildings : [];
  const nodes = Array.isArray(campus.nodes) ? campus.nodes : [];
  return buildings.length > 0 || nodes.length > 0;
}

function normalizePartialIds(partialCampusIds) {
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

function isPartialCampus(campus, partialIds) {
  if (!campus) {
    return false;
  }
  return partialIds.has(String(campus.id));
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
} = {}) {
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
