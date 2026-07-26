// Shared vitest setup. The default test environment stays node (pure helper
// tests); component tests opt into jsdom via a file-level
// `@vitest-environment jsdom` directive. Everything here must therefore be
// guarded so it no-ops without a DOM.

// antd (responsiveObserver / rc-* internals) may call window.matchMedia when
// components like Modal render; install a minimal always-false stub when the
// DOM implementation lacks it so component tests cannot crash on it.
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
