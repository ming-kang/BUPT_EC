/**
 * @vitest-environment jsdom
 *
 * TodayClassroomTable native-table rendering (R6): header semantics, row
 * filtering/sorting, free_nodes tag rendering, and the empty-state branches.
 */
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import TodayClassroomTable from "./TodayClassroomTable";

const campusData = {
  buildings: [
    {
      name: "教三",
      rooms: [
        {
          display_name: "3-201",
          capacity: 120,
          free_nodes: [1, 2, 3, 5],
          free_times: [],
        },
        {
          display_name: "3-101",
          capacity: 0,
          free_nodes: [2, 3],
          free_times: [],
        },
      ],
    },
    {
      name: "教四",
      rooms: [
        {
          display_name: "4-102",
          capacity: 60,
          free_nodes: [2, 3, 4],
          free_times: [],
        },
        {
          display_name: "4-103",
          capacity: 45,
          free_nodes: [4, 5],
          free_times: [],
        },
      ],
    },
  ],
};

function renderTable(overrides = {}) {
  return render(
    <TodayClassroomTable
      selectedCampusData={campusData}
      selectedBuildings={["教三", "教四"]}
      selectedClassTimes={[2, 3]}
      {...overrides}
    />
  );
}

describe("TodayClassroomTable", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders nothing without campus data", () => {
    const { container } = render(
      <TodayClassroomTable
        selectedCampusData={null}
        selectedBuildings={["教三"]}
        selectedClassTimes={[1]}
      />
    );
    expect(container.firstChild).toBeNull();
  });

  it("renders a 3-column thead with scope=col", () => {
    const { container } = renderTable();
    const headers = container.querySelectorAll("thead th");
    expect(headers.length).toBe(3);
    expect([...headers].map((th) => th.textContent)).toEqual([
      "教室",
      "座位数",
      "空闲节次",
    ]);
    for (const th of headers) {
      expect(th.getAttribute("scope")).toBe("col");
    }
  });

  it("renders only rooms free for every selected node, sorted by name", () => {
    const { container } = renderTable();
    // 4-103 lacks nodes 2 and 3 and must be filtered out.
    const rows = container.querySelectorAll("tbody tr");
    expect(rows.length).toBe(3);
    const names = [...rows].map(
      (row) => row.querySelector(".room-name").textContent
    );
    expect(names).toEqual(["3-101", "3-201", "4-102"]);
  });

  it("renders each room name as a button and capacity 0 as 未知", () => {
    const { container } = renderTable();
    const button = screen.getByRole("button", { name: "3-101" });
    expect(button.className).toBe("room-name");
    const rows = container.querySelectorAll("tbody tr");
    // Rows are sorted: 3-101 (capacity 0), 3-201 (120), 4-102 (60).
    const capacities = [...rows].map(
      (row) => row.querySelectorAll("td")[1].textContent
    );
    expect(capacities).toEqual(["未知", "120", "60"]);
  });

  it("shows only selected free nodes as zero-padded tags", () => {
    const { container } = renderTable();
    const rows = container.querySelectorAll("tbody tr");
    const row3201 = [...rows].find(
      (row) => row.querySelector(".room-name").textContent === "3-201"
    );
    // free_nodes [1,2,3,5] intersect selected [2,3] -> 02, 03 only.
    const tags = [...row3201.querySelectorAll("td")[2].children].map(
      (tag) => tag.textContent
    );
    expect(tags).toEqual(["02", "03"]);
    expect(within(row3201).queryByText("01")).toBeNull();
    expect(within(row3201).queryByText("05")).toBeNull();
  });

  it("asks for buildings and class times when both are empty", () => {
    renderTable({ selectedBuildings: [], selectedClassTimes: [] });
    expect(screen.getByText("请选择教学楼和上课时间")).toBeTruthy();
  });

  it("asks for buildings when only class times are selected", () => {
    renderTable({ selectedBuildings: [] });
    expect(screen.getByText("请选择教学楼")).toBeTruthy();
  });

  it("asks for class times when only buildings are selected", () => {
    renderTable({ selectedClassTimes: [] });
    expect(screen.getByText("请选择上课时间")).toBeTruthy();
  });

  it("reports no matching classrooms when filters exclude every room", () => {
    const { container } = renderTable({ selectedClassTimes: [1, 4] });
    expect(screen.getByText("没有符合条件的空教室")).toBeTruthy();
    expect(container.querySelector("table")).toBeNull();
  });
});
