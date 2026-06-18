/**
 * formatJsonPretty re-indents a raw body when it is valid JSON whose pretty form
 * actually differs from a one-liner — i.e. an object or array. Returns null for
 * non-JSON text and for bare scalars (numbers, strings, booleans, null), where a
 * Pretty/Raw toggle would be meaningless.
 */
export function formatJsonPretty(raw: string): string | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }

  if (typeof parsed !== 'object' || parsed === null) {
    return null;
  }

  return JSON.stringify(parsed, null, 2);
}
