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

type generationKind int

const (
	generationKindUnknown generationKind = iota
	generationKindOpenAI
	generationKindOllama
)

// GenerationAccumulator incrementally builds a normalized Generation snapshot
// from one provider-specific response payload at a time.
type GenerationAccumulator struct {
	kind generationKind

	output    strings.Builder
	reasoning strings.Builder
	finish    string
	context   []int
	found     bool

	toolByKey map[int]*ToolCall
	toolOrder []int
}

// NewGenerationAccumulator creates an endpoint-aware accumulator that accepts
// one response body payload at a time.
func NewGenerationAccumulator(endpoint string) *GenerationAccumulator {
	a := &GenerationAccumulator{kind: classifyGenerationEndpoint(endpoint)}
	if a.kind == generationKindOpenAI {
		a.toolByKey = make(map[int]*ToolCall)
	}
	return a
}

// Feed merges one response body payload into the accumulator state. The payload
// can be a single streamed chunk or an assembled response body.
func (a *GenerationAccumulator) Feed(body []byte) {
	if a == nil || len(bytes.TrimSpace(body)) == 0 {
		return
	}

	switch a.kind {
	case generationKindOpenAI:
		for _, raw := range bytes.Split(body, []byte("\n")) {
			a.feedOpenAILine(raw)
		}
	case generationKindOllama:
		for _, raw := range bytes.Split(body, []byte("\n")) {
			a.feedOllamaLine(raw)
		}
	}
}

// Build returns the current normalized snapshot. Nil means no decodable
// generation data has been observed yet.
func (a *GenerationAccumulator) Build() *Generation {
	if a == nil {
		return nil
	}
	return buildGeneration(a.found, a.output.String(), a.reasoning.String(), a.finish, a.context, a.buildToolCalls())
}

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
	acc := NewGenerationAccumulator(endpoint)
	acc.Feed(body)
	return acc.Build()
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

// openaiMessage is the shared shape of an OpenAI streaming `delta` and a
// non-streaming `message`: text content, reasoning, and any tool/function calls.
type openaiMessage struct {
	Content   string `json:"content"`
	Reasoning string `json:"reasoning"`
	ToolCalls []struct {
		Index    *int `json:"index"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls"`
}

func classifyGenerationEndpoint(endpoint string) generationKind {
	switch {
	case openaiEndpoints[endpoint]:
		return generationKindOpenAI
	case inferenceEndpoints[endpoint]:
		return generationKindOllama
	default:
		return generationKindUnknown
	}
}

func (a *GenerationAccumulator) feedOpenAILine(raw []byte) {
	line := bytes.TrimSpace(bytes.TrimPrefix(bytes.TrimSpace(raw), []byte("data: ")))
	if len(line) == 0 || bytes.Equal(line, []byte("[DONE]")) {
		return
	}

	var chunk struct {
		Choices []struct {
			Delta        openaiMessage `json:"delta"`
			Message      openaiMessage `json:"message"`
			FinishReason string        `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(line, &chunk); err != nil {
		return
	}

	for _, choice := range chunk.Choices {
		a.mergeOpenAIMessage(choice.Delta)
		a.mergeOpenAIMessage(choice.Message)
		if choice.FinishReason != "" {
			a.finish = choice.FinishReason
		}
		a.found = true
	}
}

func (a *GenerationAccumulator) feedOllamaLine(raw []byte) {
	line := bytes.TrimSpace(raw)
	if len(line) == 0 {
		return
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
		return
	}

	a.output.WriteString(obj.Response)
	a.output.WriteString(obj.Message.Content)
	a.reasoning.WriteString(obj.Thinking)
	a.reasoning.WriteString(obj.Message.Thinking)
	if obj.DoneReason != "" {
		a.finish = obj.DoneReason
	}
	if len(obj.Context) > 0 {
		a.context = obj.Context
	}
	a.found = true
}

func (a *GenerationAccumulator) mergeOpenAIMessage(msg openaiMessage) {
	a.output.WriteString(msg.Content)
	a.reasoning.WriteString(msg.Reasoning)
	for i, tc := range msg.ToolCalls {
		a.mergeToolCallDelta(toolCallKey(tc.Index, i), tc.Function.Name, tc.Function.Arguments)
	}
}

func (a *GenerationAccumulator) mergeToolCallDelta(key int, name, args string) {
	if a.toolByKey == nil {
		a.toolByKey = make(map[int]*ToolCall)
	}
	tc, ok := a.toolByKey[key]
	if !ok {
		tc = &ToolCall{}
		a.toolByKey[key] = tc
		a.toolOrder = append(a.toolOrder, key)
	}
	if name != "" {
		tc.Name = name
	}
	tc.Arguments += args
}

func (a *GenerationAccumulator) buildToolCalls() []ToolCall {
	if len(a.toolOrder) == 0 {
		return nil
	}
	tools := make([]ToolCall, 0, len(a.toolOrder))
	for _, key := range a.toolOrder {
		tools = append(tools, *a.toolByKey[key])
	}
	return tools
}

func toolCallKey(idx *int, fallback int) int {
	if idx != nil {
		return *idx
	}
	return -(fallback + 1)
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
