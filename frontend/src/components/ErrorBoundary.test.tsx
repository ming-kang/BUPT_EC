/**
 * @vitest-environment jsdom
 *
 * ErrorBoundary (R13): renders the fallback on a child render error and no
 * longer swallows the error silently (componentDidCatch logs it).
 */
import { cleanup, render, screen } from "@testing-library/react";
import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type MockInstance,
} from "vitest";
import ErrorBoundary from "./ErrorBoundary";

function Boom(): null {
  throw new Error("boom");
}

function Fine() {
  return <p>正常内容</p>;
}

describe("ErrorBoundary", () => {
  let errorSpy: MockInstance;

  beforeEach(() => {
    // React itself also logs the caught error; the spy both silences that noise
    // and lets us assert our own componentDidCatch line.
    errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    cleanup();
    errorSpy.mockRestore();
  });

  it("renders the fallback and logs the error when a child throws", () => {
    render(
      <ErrorBoundary fallback={<div>页面出错，请刷新重试</div>}>
        <Boom />
      </ErrorBoundary>
    );

    expect(screen.getByText("页面出错，请刷新重试")).toBeTruthy();
    const logged = errorSpy.mock.calls.find(
      ([tag]) => tag === "[ErrorBoundary]"
    )!;
    expect(logged).toBeTruthy();
    expect(logged[1]).toBeInstanceOf(Error);
    expect(logged[1].message).toBe("boom");
    // componentStack from the error info (string in React 18).
    expect(typeof logged[2]).toBe("string");
  });

  it("renders nothing when a child throws and no fallback is given", () => {
    const { container } = render(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>
    );
    expect(container.firstChild).toBeNull();
  });

  it("renders children untouched and logs nothing when they do not throw", () => {
    render(
      <ErrorBoundary fallback={<div>页面出错，请刷新重试</div>}>
        <Fine />
      </ErrorBoundary>
    );

    expect(screen.getByText("正常内容")).toBeTruthy();
    expect(screen.queryByText("页面出错，请刷新重试")).toBeNull();
    expect(
      errorSpy.mock.calls.some(([tag]) => tag === "[ErrorBoundary]")
    ).toBe(false);
  });
});
