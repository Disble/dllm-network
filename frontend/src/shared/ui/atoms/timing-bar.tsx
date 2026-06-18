import type { TimingBarProps } from './timing-bar.types';

/**
 * TimingBar renders a DevTools-style stacked waterfall of a request's phases
 * (load, eval, and the remaining time) as proportions of total duration.
 * Renders an honest em-dash when total timing is unavailable.
 */
export function TimingBar({ loadMS, evalMS, totalMS }: Readonly<TimingBarProps>) {
  if (totalMS === null || totalMS <= 0) {
    return (
      <span className="timing-bar timing-bar--unavailable" aria-label="Timing unavailable">
        {'—'}
      </span>
    );
  }

  const load = loadMS ?? 0;
  const evaluate = evalMS ?? 0;
  const other = Math.max(totalMS - load - evaluate, 0);

  const pct = (value: number) => `${((value / totalMS) * 100).toFixed(1)}%`;

  return (
    <div className="timing-bar" role="img" aria-label={`Total ${Math.round(totalMS)}ms`}>
      <span className="timing-bar__segment timing-bar__segment--load" style={{ width: pct(load) }} title={`load ${Math.round(load)}ms`} />
      <span className="timing-bar__segment timing-bar__segment--eval" style={{ width: pct(evaluate) }} title={`eval ${Math.round(evaluate)}ms`} />
      <span className="timing-bar__segment timing-bar__segment--other" style={{ width: pct(other) }} title={`other ${Math.round(other)}ms`} />
    </div>
  );
}
