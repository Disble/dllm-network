// Test setup: polyfill browser APIs jsdom does not implement but that the app
// depends on. @tanstack/react-virtual observes element size via ResizeObserver,
// which jsdom lacks — without this stub the virtualized table throws on mount
// and would white-screen the full-tree smoke test.

class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}

const globalWithObservers = globalThis as typeof globalThis & {
  ResizeObserver?: typeof ResizeObserverStub;
};

if (globalWithObservers.ResizeObserver === undefined) {
  globalWithObservers.ResizeObserver = ResizeObserverStub;
}
