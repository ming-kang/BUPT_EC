import { describe, expect, it } from "vitest";
import { chooseCampusId } from "./campusSelection";
import type { CampusInfo } from "./api/types";

const shaheEmpty: CampusInfo = {
  id: "04",
  name: "沙河",
  buildings: [],
  nodes: [],
};

const shaheWithData: CampusInfo = {
  id: "04",
  name: "沙河",
  buildings: [{ name: "S1", display_name: "S1", rooms: [] }],
  nodes: [],
};

const xituchengWithData: CampusInfo = {
  id: "01",
  name: "西土城",
  buildings: [{ name: "主楼", display_name: "主楼", rooms: [] }],
  nodes: [],
};

const xituchengEmpty: CampusInfo = {
  id: "01",
  name: "西土城",
  buildings: [],
  nodes: [],
};

describe("chooseCampusId", () => {
  it("prefers non-partial Shahe when both campuses are usable", () => {
    expect(
      chooseCampusId({
        campuses: [xituchengWithData, shaheWithData],
        partialCampusIds: [],
        selectedCampusId: "",
      })
    ).toBe("04");
  });

  it("uses decoded campuses when the optional partial list is absent", () => {
    expect(
      chooseCampusId({
        campuses: [xituchengWithData, shaheWithData],
        selectedCampusId: "",
      })
    ).toBe("04");
  });

  it("keeps a selected campus with nodes but no buildings", () => {
    expect(
      chooseCampusId({
        campuses: [xituchengWithData, {
          ...shaheEmpty,
          nodes: [{ node: 1, time: "08:00-08:45", room_count: 0 }],
        }],
        partialCampusIds: ["04"],
        selectedCampusId: "04",
      })
    ).toBe("04");
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
