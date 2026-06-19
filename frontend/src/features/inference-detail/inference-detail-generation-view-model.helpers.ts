import type { GenerationData, ToolCallData } from '../../shared/contracts/dashboard-snapshot.types';
import type { GenerationView, ToolCallView } from './inference-detail.types';

/**
 * buildGenerationView maps the NORMALIZED domain GenerationData (decoded at the
 * backend extractor boundary — Ollama-native and OpenAI streams both arrive in
 * one shape) into a display-ready view. This layer does PRESENTATION only: it
 * re-indents JSON output and formats the bounded context sample into a string.
 * It never parses a response wire format — that knowledge lives in the backend.
 *
 * Returns null (honest, never fabricated) when no generation payload exists.
 */
export function buildGenerationView(generation: GenerationData | null | undefined): GenerationView | null {
  if (generation === null || generation === undefined) {
    return null;
  }

  const { output, outputIsJson } = formatOutput(generation.output);

  return {
    output,
    outputRaw: generation.output,
    outputIsJson,
    reasoning: generation.reasoning,
    contextTokenCount: generation.contextSize > 0 ? generation.contextSize : null,
    contextPreview: formatContextPreview(generation.contextSize, generation.contextPreview),
    doneReason: generation.finishReason !== '' ? generation.finishReason : null,
    toolCalls: mapToolCalls(generation.toolCalls),
  };
}

/**
 * mapToolCalls turns the normalized tool calls into display-ready views,
 * pretty-printing each one's JSON arguments. Empty array when none.
 */
function mapToolCalls(toolCalls: readonly ToolCallData[] | null): readonly ToolCallView[] {
  if (toolCalls === null) {
    return [];
  }
  return toolCalls.map((call) => {
    const { output: prettyArgs, outputIsJson } = formatOutput(call.arguments);
    return {
      name: call.name,
      argumentsRaw: call.arguments,
      arguments: prettyArgs,
      argumentsIsJson: outputIsJson,
    };
  });
}

/**
 * formatOutput re-indents the model output when it is itself valid JSON,
 * otherwise returns it verbatim as plain text.
 */
function formatOutput(outputText: string): { output: string; outputIsJson: boolean } {
  try {
    const inner = JSON.parse(outputText);
    if (typeof inner === 'object' && inner !== null) {
      return { output: JSON.stringify(inner, null, 2), outputIsJson: true };
    }
  } catch {
    // Not JSON — legitimate free-form model text.
  }
  return { output: outputText, outputIsJson: false };
}

/**
 * formatContextPreview joins the bounded sample of context token IDs and elides
 * with an ellipsis when the full context is larger than the sample. Empty string
 * when no context is present.
 */
function formatContextPreview(contextSize: number, preview: readonly number[] | null): string {
  if (preview === null || preview.length === 0) {
    return '';
  }
  const head = preview.join(', ');
  return contextSize > preview.length ? `${head}, …` : head;
}
