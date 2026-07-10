import { describe, expect, it } from "vitest";
import {
  chooseCampusId,
  hasCampusSnapshot,
  PREFERRED_CAMPUS_NAME,
} from "./campusSelection";

const shaheEmpty = {
  id: "04",
  name: "沙河",
  buildings: [],
  nodes: [],
};

const shaheWithData = {
  id: "04",
  name: "沙河",
  buildings: [{ name: "S1", rooms: [] }],
  nodes: [],
};

const xituchengWithData = {
  id: "01",
  name: "西土城",
  buildings: [{ name: "主楼", rooms: [] }],
  nodes: [],
};

const xituchengEmpty = {
  id: "01",
  name: "西土城",
  buildings: [],
  nodes: [],
};

describe("hasCampusSnapshot", () => {
  it("detects buildings or nodes", () => {
    expect(hasCampusSnapshot(shaheEmpty)).toBe(false);
    expect(hasCampusSnapshot(shaheWithData)).toBe(true);
    expect(hasCampusSnapshot({ id: "x", nodes: [{ node: 1 }] })).toBe(true);
  });
});

describe("chooseCampusId", () => {
  it("prefers non-partial Shahe when both campuses are usable", () => {
    expect(
      chooseCampusId({
        campuses: [xituchengWithData, shaheWithData],
        partialCampusIds: [],
        selectedCampusId: "",
      })
    ).toBe("04");
    expect(PREFERRED_CAMPUS_NAME).toBe("沙河");
  });

  it("selects Xitucheng when cold Shahe is a partial empty placeholder", () => {
    expect(
      chooseCampusId({
        campuses: [xituchengWithData, shaheEmpty],
        partialCampusIds: ["04"],
        selectedCampusId: "",
      })
    ).toBe("01");
  });

  it("keeps selected partial campus when it still has same-day snapshot data", () => {
    expect(
      chooseCampusId({
        campuses: [xituchengWithData, shaheWithData],
        partialCampusIds: ["04"],
        selectedCampusId: "04",
      })
    ).toBe("04");
  });

  it("switches away from selected empty partial placeholder", () => {
    expect(
      chooseCampusId({
        campuses: [xituchengWithData, shaheEmpty],
        partialCampusIds: ["04"],
        selectedCampusId: "04",
      })
    ).toBe("01");
  });

  it("falls back among all-partial campuses to first with snapshot data", () => {
    expect(
      chooseCampusId({
        campuses: [shaheEmpty, xituchengWithData],
        partialCampusIds: ["04", "01"],
        selectedCampusId: "",
      })
    ).toBe("01");
  });

  it("uses first campus when nothing has snapshot data", () => {
    expect(
      chooseCampusId({
        campuses: [shaheEmpty, xituchengEmpty],
        partialCampusIds: ["04", "01"],
        selectedCampusId: "",
      })
    ).toBe("04");
  });

  it("returns empty string for empty campus lists", () => {
    expect(
      chooseCampusId({
        campuses: [],
        partialCampusIds: ["04"],
        selectedCampusId: "04",
      })
    ).toBe("");
  });

  it("re-selects when the current campus disappears from the payload", () => {
    expect(
      chooseCampusId({
        campuses: [xituchengWithData],
        partialCampusIds: [],
        selectedCampusId: "04",
      })
    ).toBe("01");
  });
});
