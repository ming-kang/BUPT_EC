export {};

// Shared vitest setup. Component tests bypass main.tsx, so jsdom environments
// load the same antd React 19 renderer before their test modules. Pure node
// tests skip the antd graph entirely.
if (typeof window !== "undefined") {
  await import("@ant-design/v5-patch-for-react-19");
}

// DOM-dependent stubs below remain guarded for jsdom-only use.

// antd (responsiveObserver / rc-* internals) may call window.matchMedia when
// components like Modal render; install a minimal always-false stub when
// the DOM implementation lacks it so component tests cannot crash on it.
if (typeof window !== "undefined" && typeof window.matchMedia !== "function") {
  window.matchMedia = (query) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener() {},
    removeListener() {},
    addEventListener() {},
    removeEventListener() {},
    dispatchEvent() {
      return false;
    },
  });
}
