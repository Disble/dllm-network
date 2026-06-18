import type { GenerationView } from './inference-detail.types';

/** How many context token IDs to show before eliding the rest. */
const CONTEXT_PREVIEW_LIMIT = 6;

/**
 * buildGenerationView parses a captured Ollama generate/chat response body into
 * a display-ready, LLM-aware view. The body is plain JSON — nothing is
 * encrypted; `response` is a (possibly JSON) string and `context` is an array
 * of tokenizer IDs summarised by count + a short preview.
 *
 * Returns null (honest, never fabricated) when the body is absent or is not a
 * generation payload — i.e. it carries none of `response`/`context`/`done_reason`.
 */
export function buildGenerationView(body: string | undefined): GenerationView | null {
  if (body === undefined || body === '') {
    return null;
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(body);
  } catch {
    return null;
  }

  if (typeof parsed !== 'object' || parsed === null) {
    return null;
  }

  const record = parsed as Record<string, unknown>;
  const hasResponse = typeof record.response === 'string';
  const hasContext = Array.isArray(record.context);
  const hasDoneReason = typeof record.done_reason === 'string';

  if (!hasResponse && !hasContext && !hasDoneReason) {
    return null;
  }

  const responseText = hasResponse ? (record.response as string) : '';
  const { output, outputIsJson } = formatOutput(responseText);

  const contextTokens = hasContext ? (record.context as unknown[]) : null;

  return {
    output,
    outputRaw: responseText,
    outputIsJson,
    contextTokenCount: contextTokens === null ? null : contextTokens.length,
    contextPreview: contextTokens === null ? '' : previewContext(contextTokens),
    doneReason: hasDoneReason ? (record.done_reason as string) : null,
  };
}

/**
 * formatOutput re-indents the model output when it is itself valid JSON,
 * otherwise returns it verbatim as plain text.
 */
function formatOutput(responseText: string): { output: string; outputIsJson: boolean } {
  try {
    const inner = JSON.parse(responseText);
    if (typeof inner === 'object' && inner !== null) {
      return { output: JSON.stringify(inner, null, 2), outputIsJson: true };
    }
  } catch {
    // Not JSON — legitimate free-form model text.
  }
  return { output: responseText, outputIsJson: false };
}

/**
 * previewContext joins the first few token IDs and elides the remainder so the
 * UI never dumps a thousand-element array.
 */
function previewContext(tokens: readonly unknown[]): string {
  const head = tokens.slice(0, CONTEXT_PREVIEW_LIMIT).join(', ');
  return tokens.length > CONTEXT_PREVIEW_LIMIT ? `${head}, …` : head;
}
