import type { TokenRateBadgeProps } from './token-rate-badge.types';

/**
 * TokenRateBadge renders tokens-per-second rate derived from capture data.
 * Renders an honest em-dash when data is unavailable (null) — never shows fabricated zero.
 */
export function TokenRateBadge({ perSec }: Readonly<TokenRateBadgeProps>) {
  if (perSec === null) {
    return (
      <span className="token-rate-badge token-rate-badge--unavailable" aria-label="Token rate unavailable">
        {'—'}
      </span>
    );
  }

  const formatted = `${perSec.toFixed(1)} tok/s`;

  return (
    <span className="token-rate-badge" aria-label={`${perSec.toFixed(1)} tokens per second`}>
      {formatted}
    </span>
  );
}
