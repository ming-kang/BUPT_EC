import PropTypes from "prop-types";
import { Button } from "antd";
import "./ToggleButtonGroup.css";

/**
 * ToggleButton is the single toggle-button primitive: selection is conveyed
 * by antd primary/default styling plus aria-pressed (design D10 / a11y R12).
 * aria-pressed is only rendered when `pressed` is provided as a boolean, so
 * non-toggle buttons (select-all, settings trigger) stay plain buttons.
 */
function ToggleButton({ pressed, className, disabled, onClick, children }) {
  const buttonProps = {
    className,
    disabled,
    onClick,
  };
  if (typeof pressed === "boolean") {
    buttonProps.type = pressed ? "primary" : "default";
    buttonProps["aria-pressed"] = pressed;
  }
  return <Button {...buttonProps}>{children}</Button>;
}

/**
 * ToggleButtonGroup renders a multi-select group of toggle buttons from
 * option descriptors. Domain logic (store dispatch, pruning, select-all)
 * stays with the callers; this component only owns the shared presentation.
 */
function ToggleButtonGroup({ options, selectedValues, onToggle, className }) {
  const selected = Array.isArray(selectedValues) ? selectedValues : [];
  const list = Array.isArray(options) ? options : [];

  return (
    <div className={`toggle-button-group ${className || ""}`}>
      {list.map((option) => (
        <ToggleButton
          key={option.value}
          pressed={selected.includes(option.value)}
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

ToggleButton.propTypes = {
  // Omitted `pressed` keeps the button a plain action button (no aria-pressed).
  pressed: PropTypes.bool,
  className: PropTypes.string,
  disabled: PropTypes.bool,
  onClick: PropTypes.func,
  children: PropTypes.node,
};

ToggleButtonGroup.propTypes = {
  options: PropTypes.arrayOf(
    PropTypes.shape({
      value: PropTypes.string.isRequired,
      label: PropTypes.node,
      // content overrides label when a richer node layout is needed.
      content: PropTypes.node,
      className: PropTypes.string,
      disabled: PropTypes.bool,
    })
  ).isRequired,
  selectedValues: PropTypes.arrayOf(PropTypes.string),
  onToggle: PropTypes.func.isRequired,
  className: PropTypes.string,
};

export { ToggleButtonGroup, ToggleButton };
