import type { FormattedJsonStream } from './inference-detail-body.types';

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

/**
 * formatJsonStream re-indents an NDJSON stream, returning each parsed object
 * with a document count. Non-JSON lines (such as SSE [DONE] sentinels) and
 * bare scalars are ignored. Returns null when no JSON objects are found.
 */
export function formatJsonStream(raw: string): FormattedJsonStream | null {
  const documents: unknown[] = [];

  for (const line of raw.split('\n')) {
    const trimmed = line.trim();
    if (trimmed === '') {
      continue;
    }

    let parsed: unknown;
    try {
      parsed = JSON.parse(trimmed);
    } catch {
      continue;
    }

    if (typeof parsed === 'object' && parsed !== null) {
      documents.push(parsed);
    }
  }

  if (documents.length === 0) {
    return null;
  }

  const pretty = documents.map((doc) => JSON.stringify(doc, null, 2)).join('\n');
  return { count: documents.length, pretty };
}
