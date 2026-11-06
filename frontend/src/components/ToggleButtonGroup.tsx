import type { ReactNode } from "react";
import { Button } from "antd";
import "./ToggleButtonGroup.css";

/**
 * ToggleButton is the single toggle-button primitive: selection is conveyed
 * by antd primary/default styling plus aria-pressed (design D10 / a11y R12).
 * aria-pressed is only rendered when `pressed` is provided as a boolean, so
 * non-toggle buttons (select-all, settings trigger) stay plain buttons.
 */
interface ToggleButtonProps {
  /** Omitted `pressed` keeps the button a plain action button (no aria-pressed). */
  pressed?: boolean;
  className?: string;
  disabled?: boolean;
  onClick?: () => void;
  children?: ReactNode;
}

function ToggleButton({ pressed, className, disabled, onClick, children }: ToggleButtonProps) {
  return (
    <Button
      className={className}
      disabled={disabled}
      onClick={onClick}
      type={pressed ? "primary" : "default"}
      aria-pressed={pressed}
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
