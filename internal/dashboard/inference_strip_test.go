package dashboard

import (
	"testing"
	"time"

	"dllm-network/internal/telemetry/inference"
)

func TestProject_StripsRecentBodiesButKeepsCurrentAndMetadata(t *testing.T) {
	proj := NewProjector(func() time.Time { return time.Unix(0, 0) })

	input := ProjectionInput{
		Inference: InferenceState{
			Current: inference.Inference{ID: "cur", ResponseBody: "live-body", RequestBody: "live-prompt", Generation: &inference.Generation{Output: "live-output"}},
			Recent: []inference.Inference{
				{
					ID:                    "r1",
					Model:                 "gemma4:12b",
					Endpoint:              "/v1/chat/completions",
					PromptSize:            1234,
					RequestBody:           "the prompt",
					RequestBodyTruncated:  true,
					ResponseBody:          "the big response",
					ResponseBodyTruncated: true,
					Generation:            &inference.Generation{Output: "the big output"},
					RequestHeaders:        []inference.Header{{Name: "Content-Type", Value: "application/json"}},
					ResponseHeaders:       []inference.Header{{Name: "Server", Value: "ollama"}},
				},
			},
		},
	}

	snap := proj.Project(input, nil)

	row := snap.Inference.Recent[0]
	// Heavy fields stripped from the recent list...
	if row.RequestBody != "" || row.ResponseBody != "" {
		t.Fatalf("recent bodies should be stripped, got req=%q resp=%q", row.RequestBody, row.ResponseBody)
	}
	if row.RequestHeaders != nil || row.ResponseHeaders != nil {
		t.Fatal("recent headers should be stripped")
	}
	if row.RequestBodyTruncated || row.ResponseBodyTruncated {
		t.Fatal("truncation flags should be reset once bodies are stripped")
	}
	if row.Generation != nil {
		t.Fatalf("recent generation should be stripped (heavy text), got %+v", row.Generation)
	}
	// ...but metadata kept for the table.
	if row.Model != "gemma4:12b" || row.Endpoint != "/v1/chat/completions" || row.PromptSize != 1234 {
		t.Fatalf("recent metadata should be preserved, got %+v", row)
	}

	// Current keeps its body for live detail without a round trip.
	if snap.Inference.Current.ResponseBody != "live-body" || snap.Inference.Current.RequestBody != "live-prompt" {
		t.Fatalf("current bodies must be preserved, got %+v", snap.Inference.Current)
	}
	// Current keeps its generation so the live Generation tab works without a
	// round trip (and the no-persistence fallback path keeps the output text).
	if snap.Inference.Current.Generation == nil || snap.Inference.Current.Generation.Output != "live-output" {
		t.Fatalf("current generation must be preserved, got %+v", snap.Inference.Current.Generation)
	}
}
