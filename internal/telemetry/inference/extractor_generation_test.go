package inference

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// TestExtractGeneration exercises the provider-agnostic generation extractor.
// It is the anti-corruption boundary: Ollama-native NDJSON and OpenAI SSE both
// collapse into one normalized *Generation so the frontend never parses a wire
// format. Cases assert OBSERVABLE content, not the parsing mechanics.
func TestExtractGeneration(t *testing.T) {
	t.Parallel()

	// OpenAI streaming: assembled SSE blob. Reasoning arrives in delta.reasoning
	// before the visible content; finish_reason rides the last content chunk; a
	// trailing usage chunk and [DONE] sentinel carry no content.
	openaiStream := "data: " + `{"object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"","reasoning":"The "}}]}` + "\n" +
		"data: " + `{"object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"","reasoning":"user."}}]}` + "\n" +
		"data: " + `{"object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"¡Hola"}}]}` + "\n" +
		"data: " + `{"object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":"stop"}]}` + "\n" +
		"data: " + `{"object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2}}` + "\n" +
		"data: [DONE]\n"

	// OpenAI non-streaming: a single chat.completion object.
	openaiNonStream := `{"object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"Hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":17,"completion_tokens":3}}`

	// Ollama /api/generate: each NDJSON line carries one `response` token; the
	// terminal line carries done_reason + the context KV-cache handle.
	ollamaGenerate := `{"model":"gemma4:12b","response":"Hola","done":false}` + "\n" +
		`{"model":"gemma4:12b","response":" mundo","done":false}` + "\n" +
		`{"model":"gemma4:12b","response":"","done":true,"done_reason":"stop","context":[1,2,3,4,5,6,7,8],"eval_count":2}` + "\n"

	// Ollama /api/chat: tokens arrive under message.content; reasoning models add
	// message.thinking.
	ollamaChat := `{"message":{"role":"assistant","content":"Hi"},"done":false}` + "\n" +
		`{"message":{"role":"assistant","content":" there","thinking":"pondering "},"done":false}` + "\n" +
		`{"message":{"role":"assistant","content":"","thinking":"done"},"done":true,"done_reason":"stop"}` + "\n"

	tests := []struct {
		name     string
		endpoint string
		body     string
		want     *Generation
	}{
		{
			name:     "openai streaming joins content and reasoning",
			endpoint: "/v1/chat/completions",
			body:     openaiStream,
			want:     &Generation{Output: "¡Hola!", Reasoning: "The user.", FinishReason: "stop"},
		},
		{
			name:     "openai non-streaming reads message.content",
			endpoint: "/v1/chat/completions",
			body:     openaiNonStream,
			want:     &Generation{Output: "Hi", FinishReason: "stop"},
		},
		{
			name:     "ollama generate joins response tokens and summarises context",
			endpoint: "/api/generate",
			body:     ollamaGenerate,
			want:     &Generation{Output: "Hola mundo", FinishReason: "stop", ContextSize: 8, ContextPreview: []int{1, 2, 3, 4, 5, 6}},
		},
		{
			name:     "ollama chat joins message.content and thinking",
			endpoint: "/api/chat",
			body:     ollamaChat,
			want:     &Generation{Output: "Hi there", Reasoning: "pondering done", FinishReason: "stop"},
		},
		{
			name:     "metadata endpoint has no generation",
			endpoint: "/api/tags",
			body:     `{"models":[]}`,
			want:     nil,
		},
		{
			name:     "empty body has no generation",
			endpoint: "/v1/chat/completions",
			body:     "",
			want:     nil,
		},
		{
			name:     "garbage body has no generation",
			endpoint: "/api/generate",
			body:     "not json at all",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractGeneration(tt.endpoint, []byte(tt.body))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractGeneration() =\n  %+v\nwant\n  %+v", got, tt.want)
			}
		})
	}
}

// TestExtractGeneration_OpenAIToolCalls verifies that streamed and non-streamed
// OpenAI tool/function calls are reassembled into normalized ToolCalls — the
// real generated payload for agent clients (GitHub Copilot) whose `content` is
// empty. Streamed arguments arrive in fragments keyed by index and must be
// concatenated; the function name arrives only in the first delta.
func TestExtractGeneration_OpenAIToolCalls(t *testing.T) {
	t.Parallel()

	// Streaming: name in the first delta, arguments fragmented across chunks,
	// terminal finish_reason chunk carries no tool delta.
	stream := "data: " + `{"object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","index":0,"type":"function","function":{"name":"run_in_terminal","arguments":""}}]}}]}` + "\n" +
		"data: " + `{"object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"command\":"}}]}}]}` + "\n" +
		"data: " + `{"object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"ls\"}"}}]}}]}` + "\n" +
		"data: " + `{"object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}` + "\n" +
		"data: [DONE]\n"

	got := ExtractGeneration("/v1/chat/completions", []byte(stream))
	if got == nil {
		t.Fatal("expected a generation for a tool-call stream (content is empty but a tool was called)")
	}
	if len(got.ToolCalls) != 1 {
		t.Fatalf("ToolCalls: got %d, want 1 (%+v)", len(got.ToolCalls), got.ToolCalls)
	}
	if got.ToolCalls[0].Name != "run_in_terminal" {
		t.Errorf("tool name: got %q, want run_in_terminal", got.ToolCalls[0].Name)
	}
	if got.ToolCalls[0].Arguments != `{"command":"ls"}` {
		t.Errorf("tool arguments: got %q, want %q", got.ToolCalls[0].Arguments, `{"command":"ls"}`)
	}
	if got.FinishReason != "tool_calls" {
		t.Errorf("finish reason: got %q, want tool_calls", got.FinishReason)
	}

	// Non-streaming: the whole tool call lives in choices[].message.tool_calls.
	nonStream := `{"object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_9","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"NYC\"}"}}]},"finish_reason":"tool_calls"}]}`
	gotNS := ExtractGeneration("/v1/chat/completions", []byte(nonStream))
	if gotNS == nil || len(gotNS.ToolCalls) != 1 {
		t.Fatalf("expected one tool call from non-streaming message, got %+v", gotNS)
	}
	if gotNS.ToolCalls[0].Name != "get_weather" || gotNS.ToolCalls[0].Arguments != `{"city":"NYC"}` {
		t.Errorf("non-stream tool call: got %+v", gotNS.ToolCalls[0])
	}
}

// TestExtractGeneration_ContextPreviewBounded verifies a long Ollama context is
// summarised by count + a bounded preview — the wire never carries the full
// thousand-int array.
func TestExtractGeneration_ContextPreviewBounded(t *testing.T) {
	t.Parallel()

	ctx := make([]int, 100)
	for i := range ctx {
		ctx[i] = i
	}
	body := `{"response":"x","done":true,"context":` + intsJSON(ctx) + `}`

	got := ExtractGeneration("/api/generate", []byte(body))
	if got == nil {
		t.Fatal("expected a generation for an /api/generate body with context")
	}
	if got.ContextSize != 100 {
		t.Errorf("ContextSize: got %d, want 100", got.ContextSize)
	}
	if len(got.ContextPreview) != contextPreviewLimit {
		t.Errorf("ContextPreview length: got %d, want %d", len(got.ContextPreview), contextPreviewLimit)
	}
}

// intsJSON renders an []int as a JSON array literal for fixture bodies.
func intsJSON(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(x)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
