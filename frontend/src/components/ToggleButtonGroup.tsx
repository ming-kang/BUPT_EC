import type { ReactNode } from "react";
import { Button } from "antd";
import "./ToggleButtonGroup.css";

/**
 * ToggleButton is the single toggle-button primitive: selection is conveyed
 * by antd primary/default styling plus aria-pressed, or role=radio +
 * aria-checked when `checked` is provided for a single-select group.
 * Neither attribute is rendered without its prop, so non-toggle buttons
 * (select-all, settings trigger) stay plain buttons.
 */
type ToggleButtonProps = {
  className?: string;
  disabled?: boolean;
  onClick?: () => void;
  children?: ReactNode;
} & (
  | { pressed: boolean; checked?: never }
  | { checked: boolean; pressed?: never }
  | { pressed?: never; checked?: never }
);

function ToggleButton({ pressed, checked, className, disabled, onClick, children }: ToggleButtonProps) {
  return (
    <Button
      className={className}
      disabled={disabled}
      onClick={onClick}
      type={pressed || checked ? "primary" : "default"}
      {...(checked !== undefined
        ? { role: "radio", "aria-checked": checked }
        : { "aria-pressed": pressed })}
    >
      {children}
    </Button>
  );
}

/**
 * ToggleButtonGroup renders a multi-select group of toggle buttons from
 * option descriptors. Domain logic (store dispatch, pruning, select-all)
 * stays with the callers; this component only owns the shared presentation.
 */
interface ToggleOption<T extends string | number> {
  value: T;
  label?: ReactNode;
  /** content overrides label when a richer node layout is needed. */
  content?: ReactNode;
  className?: string;
  disabled?: boolean;
}

interface ToggleButtonGroupProps<T extends string | number> {
  options: ToggleOption<T>[];
  selectedValues?: T[];
  onToggle: (value: T) => void;
  className?: string;
}

function ToggleButtonGroup<T extends string | number>({
  options,
  selectedValues = [],
  onToggle,
  className,
}: ToggleButtonGroupProps<T>) {

  return (
    <div className={`toggle-button-group ${className || ""}`}>
      {options.map((option) => (
        <ToggleButton
          key={option.value}
          pressed={selectedValues.includes(option.value)}
          className={option.className}
          disabled={option.disabled}
          onClick={() => onToggle(option.value)}
        >
          {option.content || option.label}
        </ToggleButton>
      ))}
    </div>
  );
}

export { ToggleButtonGroup, ToggleButton };
