import type { StatusCodePillProps } from './status-code-pill.types';

/**
 * StatusCodePill renders an HTTP status code with a class tied to its class
 * range (2xx ok, 3xx redirect, 4xx/5xx error). Renders an honest em-dash when
 * unavailable — never a fabricated zero.
 */
export function StatusCodePill({ statusCode }: Readonly<StatusCodePillProps>) {
  if (statusCode === null || statusCode === undefined || statusCode === 0) {
    return (
      <span className="status-code-pill status-code-pill--unavailable" aria-label="Status code unavailable">
        {'—'}
      </span>
    );
  }

  const variant = statusCode >= 500 || statusCode < 200
    ? 'error'
    : statusCode >= 400
      ? 'error'
      : statusCode >= 300
        ? 'redirect'
        : 'ok';

  return (
    <span className={`status-code-pill status-code-pill--${variant}`} aria-label={`HTTP ${statusCode}`}>
      {statusCode}
    </span>
  );
}
