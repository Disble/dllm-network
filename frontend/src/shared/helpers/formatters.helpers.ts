/** pad2 left-pads a number to two digits ("9" → "09"). */
function pad2(value: number): string {
  return String(value).padStart(2, '0');
}

/**
 * formatLocalOffset renders the date's offset from UTC in the host's local zone
 * as "±HH:MM" (e.g. "-05:00"). getTimezoneOffset returns the minutes to ADD to
 * local time to reach UTC, so its sign is inverted from the ISO convention: a
 * positive offset (behind UTC) renders with a leading "-". We surface this
 * explicitly instead of "Z" so a local time is never mislabelled as UTC.
 */
function formatLocalOffset(date: Date): string {
  const offsetMinutes = date.getTimezoneOffset();
  const sign = offsetMinutes > 0 ? '-' : '+';
  const abs = Math.abs(offsetMinutes);
  return `${sign}${pad2(Math.floor(abs / 60))}:${pad2(abs % 60)}`;
}

/**
 * localDateParts renders the calendar date ("YYYY-MM-DD") and wall-clock time
 * ("HH:MM:SS") in the HOST'S local time zone — the user's computer clock — from
 * an absolute instant. The Date already holds the correct instant regardless of
 * whether the source carried "Z" or an explicit offset.
 */
function localDateParts(date: Date): { date: string; time: string } {
  const ymd = `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}`;
  const hms = `${pad2(date.getHours())}:${pad2(date.getMinutes())}:${pad2(date.getSeconds())}`;
  return { date: ymd, time: hms };
}

/**
 * formatTimestamp renders a stable timestamp or fallback label for passive
 * telemetry surfaces, in the host's local time with an explicit "±HH:MM" offset.
 */
export function formatTimestamp(value: string): string {
  if (value === '') {
    return 'Unavailable';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return 'Unavailable';
  }

  const { date: ymd, time } = localDateParts(date);
  return `${ymd} ${time} ${formatLocalOffset(date)}`;
}

/**
 * formatClockTime renders just the wall-clock HH:MM:SS in the host's LOCAL time
 * from an RFC3339 timestamp for dense table columns. The offset is omitted here
 * because the column is space-constrained and every row shares the same zone.
 * Returns "—" when empty or unparseable.
 */
export function formatClockTime(value: string): string {
  if (value === '') {
    return '—';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }

  return localDateParts(date).time;
}

/**
 * formatClockDateTime renders an RFC3339 timestamp as "YYYY-MM-DD HH:MM:SS ±HH:MM"
 * in the host's LOCAL time (no milliseconds) for compact telemetry tiles. Returns
 * "—" when empty or unparseable.
 */
export function formatClockDateTime(value: string): string {
  if (value === '') {
    return '—';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }

  const { date: ymd, time } = localDateParts(date);
  return `${ymd} ${time} ${formatLocalOffset(date)}`;
}

/**
 * formatBytes renders byte counts into small human-readable labels.
 * Handles B, KB, MB, and GB ranges.
 */
export function formatBytes(value: number): string {
  if (value <= 0) {
    return '0 B';
  }

  if (value < 1024) {
    return `${value} B`;
  }

  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KB`;
  }

  if (value < 1024 * 1024 * 1024) {
    return `${(value / (1024 * 1024)).toFixed(1)} MB`;
  }

  return `${(value / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

/**
 * formatExpiresAt renders an ISO-8601 expires_at timestamp as a relative time label.
 * Returns "—" when the value is empty or unparseable.
 * Returns the formatted time if more than 0 seconds remain; "expired" if in the past.
 */
export function formatExpiresAt(value: string, now?: Date): string {
  if (value === '') {
    return '—';
  }

  const expiresDate = new Date(value);
  if (Number.isNaN(expiresDate.getTime())) {
    return '—';
  }

  const referenceTime = now ?? new Date();
  const diffMs = expiresDate.getTime() - referenceTime.getTime();

  if (diffMs <= 0) {
    return 'expired';
  }

  const diffSec = Math.floor(diffMs / 1000);
  const minutes = Math.floor(diffSec / 60);
  const seconds = diffSec % 60;

  if (minutes === 0) {
    return `in ${seconds}s`;
  }

  return `in ${minutes}m ${seconds}s`;
}
