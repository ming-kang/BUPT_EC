/**
 * @vitest-environment jsdom
 *
 * TodayClassroomTable native-table rendering (R6): header semantics, row
 * filtering/sorting, free_nodes tag rendering, and the empty-state branches.
 * Plus the room-info modal (R9): content derived from props at render time.
 */
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import type React from "react";
import { afterEach, describe, expect, it } from "vitest";
import TodayClassroomTable from "./TodayClassroomTable";
import type { CampusInfo } from "../api/types";

const campusData = {
  buildings: [
    {
      name: "教三",
      rooms: [
        {
          display_name: "3-201",
          capacity: 120,
          free_nodes: [1, 2, 3, 5],
          free_times: [
            { node: 2, time: "10:25-11:10" },
            { node: 3, time: "11:15-12:00" },
          ],
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

function renderTable(
  overrides?: Partial<{
    selectedCampusData: CampusInfo | null;
    selectedBuildings: string[];
    selectedClassTimes: number[];
  }>
) {
  return render(
    <TodayClassroomTable
      selectedCampusData={campusData as CampusInfo}
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
      (row) => row.querySelector(".room-name")!.textContent
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
      (row) => row.querySelector(".room-name")!.textContent === "3-201"
    )!;
    // free_nodes [1,2,3,5] intersect selected [2,3] -> 02, 03 only.
    const tags = [...row3201.querySelectorAll("td")[2].children].map(
      (tag) => tag.textContent
    );
    expect(tags).toEqual(["02", "03"]);
    expect(within(row3201 as HTMLElement).queryByText("01")).toBeNull();
    expect(within(row3201 as HTMLElement).queryByText("05")).toBeNull();
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

describe("TodayClassroomTable room modal (R9)", () => {
  afterEach(() => {
    cleanup();
  });

  function rerenderTable(
    rerender: (ui: React.ReactElement) => void,
    campus: React.ComponentProps<typeof TodayClassroomTable>["selectedCampusData"],
    overrides: Partial<{
      selectedCampusData: CampusInfo | null;
      selectedBuildings: string[];
      selectedClassTimes: number[];
    }> = {}
  ) {
    rerender(
      <TodayClassroomTable
        selectedCampusData={campus}
        selectedBuildings={["教三", "教四"]}
        selectedClassTimes={[2, 3]}
        {...overrides}
      />
    );
  }

  it("updates open-modal content when props refresh in the background", () => {
    const { rerender } = renderTable();
    fireEvent.click(screen.getByRole("button", { name: "3-201" }));
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("10:25-11:10")).toBeTruthy();

    // Background refresh: 3-201's free times move to the evening.
    rerenderTable(rerender, {
      buildings: [
        {
          name: "教三",
          rooms: [
            {
              display_name: "3-201",
              capacity: 120,
              free_nodes: [2, 3],
              free_times: [{ node: 11, time: "18:40-19:25" }],
            },
          ],
        },
      ],
    });
    expect(within(dialog).queryByText("10:25-11:10")).toBeNull();
    expect(within(dialog).getByText("18:40-19:25")).toBeTruthy();
  });

  it("keeps the modal open with fallback content when the room disappears", () => {
    const { rerender } = renderTable();
    fireEvent.click(screen.getByRole("button", { name: "3-201" }));
    const dialog = screen.getByRole("dialog");

    // Refresh drops 教三 entirely; 教四 still fills the table.
    rerenderTable(rerender, {
      buildings: [
        {
          name: "教四",
          rooms: [
            {
              display_name: "4-102",
              capacity: 60,
              free_nodes: [2, 3, 4],
              free_times: [],
            },
          ],
        },
      ],
    });
    expect(within(dialog).getByText("暂无空闲节次")).toBeTruthy();
    expect(within(dialog).getByText("—")).toBeTruthy();
    expect((document.querySelector(".ant-modal-wrap") as HTMLElement).style.display).not.toBe(
      "none"
    );
    // The title comes from the opened room's identity, so the user can still
    // tell which room the now-empty dialog refers to.
    expect(within(dialog).getByText("3-201")).toBeTruthy();
  });

  it("keeps the modal mounted when the table falls into the empty state", () => {
    const { rerender } = renderTable();
    fireEvent.click(screen.getByRole("button", { name: "3-201" }));
    const dialog = screen.getByRole("dialog");

    // Refresh leaves no room matching the selected nodes → empty-state card,
    // but the modal must stay mounted and keep following the room's data.
    rerenderTable(rerender, {
      buildings: [
        {
          name: "教三",
          rooms: [
            {
              display_name: "3-201",
              capacity: 120,
              free_nodes: [9],
              free_times: [{ node: 9, time: "20:20-21:05" }],
            },
          ],
        },
      ],
    });
    expect(screen.getByText("没有符合条件的空教室")).toBeTruthy();
    expect(within(dialog).getByText("20:20-21:05")).toBeTruthy();
  });

  it("converts capacity at render time and follows refreshes", () => {
    const { rerender } = renderTable();
    fireEvent.click(screen.getByRole("button", { name: "3-101" }));
    const dialog = screen.getByRole("dialog");
    // capacity 0 renders as 未知 (converted at render, not stored).
    expect(within(dialog).getByText("未知")).toBeTruthy();

    rerenderTable(rerender, {
      buildings: [
        {
          name: "教三",
          rooms: [
            {
              display_name: "3-101",
              capacity: 45,
              free_nodes: [2, 3],
              free_times: [],
            },
          ],
        },
      ],
    });
    expect(within(dialog).queryByText("未知")).toBeNull();
    expect(within(dialog).getByText("45")).toBeTruthy();
  });
});
