import { useEffect, useState } from 'react';

/**
 * useElapsedNow returns the current epoch-ms clock, re-rendering on an interval
 * while `active` so callers can display live-elapsed values (e.g. an in-flight
 * request's growing duration). When `active` is false it stops the timer — no
 * ticking, no wasted re-renders — and simply returns the last sampled time.
 */
export function useElapsedNow(active: boolean, intervalMS = 500): number {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!active) {
      return;
    }
    // Tick on the interval until inactive/unmounted. The lazy initial state is
    // already current at mount, so no synchronous catch-up is needed here.
    const id = window.setInterval(() => setNow(Date.now()), intervalMS);
    return () => window.clearInterval(id);
  }, [active, intervalMS]);

  return now;
}
