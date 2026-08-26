import type { CSSProperties, ReactNode } from "react";
import "./Panel.css";

interface PanelProps {
  className?: string;
  children: ReactNode;
  bodyStyle?: CSSProperties;
}

/**
 * Lightweight card container replacing antd Card. Renders a bordered div with
 * padding that matches the antd Card token defaults (border-radius 8px,
 * 24px body padding). Add `compact-card` for reduced padding (16px).
 *
 * The inner `.panel__body` replicates the `.ant-card-body` wrapper that
 * existing CSS hooks into; responsive media queries target it directly.
 */
function Panel({ className, children, bodyStyle }: PanelProps) {
  return (
    <div className={`panel ${className || ""}`}>
      <div className="panel__body" style={bodyStyle}>
        {children}
      </div>
    </div>
  );
}

export default Panel;
