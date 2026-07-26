import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App.jsx";
import ErrorBoundary from "./components/ErrorBoundary.jsx";
import "./index.css";

// R13: last-resort boundary. The fallback is plain HTML with inline styles on
// purpose — a crash can originate inside ConfigProvider/antd itself, so the
// fallback must not depend on them. Component-level boundaries (App.jsx) still
// handle local degradation first.
const rootFallback = (
  <div
    style={{
      padding: "48px 16px",
      textAlign: "center",
      fontSize: "16px",
      lineHeight: 1.6,
    }}
  >
    页面出错，请刷新重试
  </div>
);

ReactDOM.createRoot(document.getElementById("root")).render(
  <React.StrictMode>
    <ErrorBoundary fallback={rootFallback}>
      <App />
    </ErrorBoundary>
  </React.StrictMode>
);
