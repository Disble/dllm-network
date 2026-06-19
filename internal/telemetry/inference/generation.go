package inference

import (
	"bytes"
	"encoding/json"
	"strings"
)

// contextPreviewLimit bounds how many Ollama context token IDs ExtractGeneration
// surfaces in Generation.ContextPreview, so the wire never carries the full
// (often thousands-long) KV-cache handle array.
const contextPreviewLimit = 6

// ExtractGeneration derives the normalized generated content from an assembled
// response body, discriminating the provider purely by endpoint. It is the
// SINGLE place in the system that understands an LLM response wire format:
// Ollama-native NDJSON (/api/generate, /api/chat) and OpenAI-compatible SSE or
// chat.completion (/v1/...) both collapse into one *Generation — exactly as
// ExtractOpenAIStats / extractTokenStats already do for metrics.
//
// The body MUST be the FULL assembled stream (the joined chunks), not a single
// line: streamed generations carry one token per line/chunk and must be
// reassembled here. Returns nil — never a fabricated value — for metadata-only
// endpoints, empty bodies, or bodies carrying no decodable content.
func ExtractGeneration(endpoint string, body []byte) *Generation {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}

	switch {
	case openaiEndpoints[endpoint]:
		return extractOpenAIGeneration(body)
	case inferenceEndpoints[endpoint]:
		return extractOllamaGeneration(body)
	default:
		return nil
	}
}

// HasGeneratedContent reports whether a SINGLE response chunk/line carries
// generated output or reasoning text. The capture pipeline uses it to observe
// the time-to-first-token: the first chunk for which this is true marks the
// transition from prompt-processing to generation. Keeping the wire-format
// knowledge here (not in the pipeline) preserves the single anti-corruption
// boundary.
func HasGeneratedContent(endpoint string, body []byte) bool {
	g := ExtractGeneration(endpoint, body)
	return g != nil && (g.Output != "" || g.Reasoning != "" || len(g.ToolCalls) > 0)
}

// extractOpenAIGeneration reassembles an OpenAI-compatible body. Streaming
// bodies are `data: {json}` SSE lines whose tokens live in choices[].delta;
// non-streaming bodies are a single chat.completion whose text lives in
// choices[].message. Both shapes are decoded per line so one function handles
// either. The trailing usage chunk (empty choices) and the [DONE] sentinel
// contribute nothing.
// openaiMessage is the shared shape of an OpenAI streaming `delta` and a
// non-streaming `message`: text content, reasoning, and any tool/function calls.
type openaiMessage struct {
	Content   string `json:"content"`
	Reasoning string `json:"reasoning"`
	ToolCalls []struct {
		Index    int `json:"index"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls"`
}

func extractOpenAIGeneration(body []byte) *Generation {
	var output, reasoning strings.Builder
	var finish string
	found := false

	// Tool calls accumulate across chunks keyed by their index (arguments arrive
	// fragmented in streaming); toolOrder preserves first-seen order for stable
	// rendering of multi-call responses.
	toolByIndex := map[int]*ToolCall{}
	var toolOrder []int
	addToolDelta := func(idx int, name, args string) {
		tc, ok := toolByIndex[idx]
		if !ok {
			tc = &ToolCall{}
			toolByIndex[idx] = tc
			toolOrder = append(toolOrder, idx)
		}
		if name != "" {
			tc.Name = name
		}
		tc.Arguments += args
	}
	accumulate := func(m openaiMessage) {
		output.WriteString(m.Content)
		reasoning.WriteString(m.Reasoning)
		for _, tc := range m.ToolCalls {
			addToolDelta(tc.Index, tc.Function.Name, tc.Function.Arguments)
		}
	}

	for _, raw := range bytes.Split(body, []byte("\n")) {
		line := bytes.TrimSpace(bytes.TrimPrefix(bytes.TrimSpace(raw), []byte("data: ")))
		if len(line) == 0 || bytes.Equal(line, []byte("[DONE]")) {
			continue
		}

		var chunk struct {
			Choices []struct {
				Delta        openaiMessage `json:"delta"`
				Message      openaiMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}

		for _, c := range chunk.Choices {
			accumulate(c.Delta)
			accumulate(c.Message)
			if c.FinishReason != "" {
				finish = c.FinishReason
			}
			found = true
		}
	}

	tools := make([]ToolCall, 0, len(toolOrder))
	for _, idx := range toolOrder {
		tools = append(tools, *toolByIndex[idx])
	}

	return buildGeneration(found, output.String(), reasoning.String(), finish, nil, tools)
}

// extractOllamaGeneration reassembles an Ollama-native NDJSON body. /api/generate
// tokens arrive in `response` (+ `thinking` for reasoning models); /api/chat
// tokens arrive in `message.content` (+ `message.thinking`). The terminal line
// carries `done_reason` and, for /api/generate, the `context` KV-cache handle.
func extractOllamaGeneration(body []byte) *Generation {
	var output, reasoning strings.Builder
	var finish string
	var context []int
	found := false

	for _, raw := range bytes.Split(body, []byte("\n")) {
		line := bytes.TrimSpace(raw)
		if len(line) == 0 {
			continue
		}

		var obj struct {
			Response string `json:"response"`
			Thinking string `json:"thinking"`
			Message  struct {
				Content  string `json:"content"`
				Thinking string `json:"thinking"`
			} `json:"message"`
			DoneReason string `json:"done_reason"`
			Context    []int  `json:"context"`
		}
		if err := json.Unmarshal(line, &obj); err != nil {
			continue
		}

		output.WriteString(obj.Response)
		output.WriteString(obj.Message.Content)
		reasoning.WriteString(obj.Thinking)
		reasoning.WriteString(obj.Message.Thinking)
		if obj.DoneReason != "" {
			finish = obj.DoneReason
		}
		if len(obj.Context) > 0 {
			context = obj.Context
		}
		found = true
	}

	return buildGeneration(found, output.String(), reasoning.String(), finish, context, nil)
}

// buildGeneration assembles the final *Generation, applying the context preview
// bound and the honest-nil contract: nothing parsed, or parsed-but-empty,
// yields nil rather than a hollow value.
func buildGeneration(found bool, output, reasoning, finish string, context []int, tools []ToolCall) *Generation {
	if !found {
		return nil
	}

	g := &Generation{Output: output, Reasoning: reasoning, FinishReason: finish}
	if len(context) > 0 {
		g.ContextSize = len(context)
		preview := make([]int, min(contextPreviewLimit, len(context)))
		copy(preview, context)
		g.ContextPreview = preview
	}
	if len(tools) > 0 {
		g.ToolCalls = tools
	}

	if g.Output == "" && g.Reasoning == "" && g.FinishReason == "" && g.ContextSize == 0 && len(g.ToolCalls) == 0 {
		return nil
	}
	return g
}
