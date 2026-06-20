// Pin a fixed, DST-free time zone (UTC-5) so the local-time formatters render
// deterministic output regardless of the host running the suite. Node re-reads
// process.env.TZ when assigned, so Date instances created in tests honour it.
// eslint-disable-next-line no-undef -- process is available in the vitest Node runner; the flat config only declares browser globals.
process.env.TZ = 'America/Bogota';

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
