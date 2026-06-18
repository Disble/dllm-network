import { buildWaterfallGeometry } from './waterfall-bar.helpers';
import type { WaterfallBarProps } from './waterfall-bar.types';

/**
 * WaterfallBar renders a DevTools-style request bar for a dense table row. The
 * bar's width is scaled against the slowest request in the visible set so rows
 * are visually comparable; the load/eval/other segments split the bar by phase.
 * Renders an honest em-dash when this row's timing is unavailable (null != zero).
 */
export function WaterfallBar({ loadMS, evalMS, totalMS, maxMS }: Readonly<WaterfallBarProps>) {
  const geometry = buildWaterfallGeometry({ loadMS, evalMS, totalMS, maxMS });

  if (geometry === null) {
    return <span className="waterfall-bar waterfall-bar--unavailable" aria-label="Timing unavailable">{'—'}</span>;
  }

  // This is a CSS-composed data visualization (stacked proportional segments).
  // A native <img> needs a src and an <svg> would require restructuring the
  // styled segments without improving accessibility, so role="img" is kept.
  return (
    // eslint-disable-next-line react-doctor/prefer-tag-over-role
    <div className="waterfall-bar" role="img" aria-label={`Total ${Math.round(totalMS ?? 0)}ms`}>
      <div className="waterfall-bar__track">
        <div className="waterfall-bar__fill" style={{ width: `${geometry.barWidthPct}%` }}>
          <span className="waterfall-bar__segment waterfall-bar__segment--load" style={{ width: `${geometry.loadPct}%` }} />
          <span className="waterfall-bar__segment waterfall-bar__segment--eval" style={{ width: `${geometry.evalPct}%` }} />
          <span className="waterfall-bar__segment waterfall-bar__segment--other" style={{ width: `${geometry.otherPct}%` }} />
        </div>
      </div>
    </div>
  );
}
