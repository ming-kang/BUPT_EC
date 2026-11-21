/**
 * @vitest-environment jsdom
 *
 * ToggleButtonGroup: shared toggle-button presentation for the pickers.
 * aria-pressed (or aria-checked for radio mode) is rendered only for boolean
 * `pressed`/`checked`; plain action buttons stay attribute-free.
 */
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ToggleButton, ToggleButtonGroup } from "./ToggleButtonGroup";

afterEach(() => {
  cleanup();
});

describe("ToggleButton", () => {
  // antd inserts a space between two-CJK-character button labels.
  it("maps pressed to type and aria-pressed", () => {
    const { rerender } = render(
      <ToggleButton pressed onClick={() => {}}>
        沙河
      </ToggleButton>
    );
    const button = screen.getByRole("button", { name: /沙\s*河/ });
    expect(button.getAttribute("aria-pressed")).toBe("true");
    expect(button.classList.contains("ant-btn-primary")).toBe(true);

    rerender(
      <ToggleButton pressed={false} onClick={() => {}}>
        沙河
      </ToggleButton>
    );
    expect(button.getAttribute("aria-pressed")).toBe("false");
    expect(button.classList.contains("ant-btn-primary")).toBe(false);
  });

  it("maps checked to type, role=radio and aria-checked", () => {
    const { rerender } = render(
      <ToggleButton checked onClick={() => {}}>
        沙河
      </ToggleButton>
    );
    const radio = screen.getByRole("radio", { name: /沙\s*河/ });
    expect(radio.getAttribute("aria-checked")).toBe("true");
    expect(radio.hasAttribute("aria-pressed")).toBe(false);
    expect(radio.classList.contains("ant-btn-primary")).toBe(true);

    rerender(
      <ToggleButton checked={false} onClick={() => {}}>
        沙河
      </ToggleButton>
    );
    expect(radio.getAttribute("aria-checked")).toBe("false");
    expect(radio.classList.contains("ant-btn-primary")).toBe(false);
  });

  it("renders no aria-pressed when pressed is not provided", () => {
    render(
      <ToggleButton className="select-all-btn" onClick={() => {}}>
        全选
      </ToggleButton>
    );
    const button = screen.getByRole("button", { name: /全\s*选/ });
    expect(button.hasAttribute("aria-pressed")).toBe(false);
  });

  it("forwards disabled", () => {
    render(
      <ToggleButton pressed disabled onClick={() => {}}>
        12
      </ToggleButton>
    );
    expect((screen.getByRole("button", { name: "12" }) as HTMLButtonElement).disabled).toBe(true);
  });
});

describe("ToggleButtonGroup", () => {
  it("starts unselected when the optional selection is omitted", () => {
    render(
      <ToggleButtonGroup
        options={[{ value: "1", label: "教1" }]}
        onToggle={vi.fn()}
      />
    );
    expect(screen.getByRole("button", { name: "教1" }).getAttribute("aria-pressed")).toBe("false");
  });

  it("marks selected values and toggles via onToggle", () => {
    const onToggle = vi.fn();
    render(
      <ToggleButtonGroup
        options={[
          { value: "1", label: "教1" },
          { value: "2", label: "教2" },
        ]}
        selectedValues={["1"]}
        onToggle={onToggle}
      />
    );
    expect(screen.getByRole("button", { name: "教1" }).getAttribute("aria-pressed")).toBe("true");
    expect(screen.getByRole("button", { name: "教2" }).getAttribute("aria-pressed")).toBe("false");

    fireEvent.click(screen.getByRole("button", { name: "教2" }));
    expect(onToggle).toHaveBeenCalledWith("2");
  });

  it("renders disabled options without firing onToggle and renders content nodes", () => {
    const onToggle = vi.fn();
    render(
      <ToggleButtonGroup
        options={[
          { value: "2", label: "教2", disabled: true },
          { value: "3", label: "教3", content: <strong>教3</strong> },
        ]}
        selectedValues={[]}
        onToggle={onToggle}
      />
    );
    expect((screen.getByRole("button", { name: "教2" }) as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "教2" }));
    expect(onToggle).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "教3" }).querySelector("strong")).not.toBeNull();
  });
});
