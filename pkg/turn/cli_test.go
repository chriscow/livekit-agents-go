package turn

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chriscow/livekit-agents-go/pkg/ai/llm"
)

func TestCLIPredictWithStub(t *testing.T) {
	// Create a stub detector that returns deterministic values
	stub := &StubDetector{
		probability: 0.95,
		threshold:   0.85,
		supported:   true,
	}

	// Create test input
	input := struct {
		Messages []llm.Message `json:"messages"`
		Language string         `json:"language,omitempty"`
	}{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hello"},
			{Role: llm.RoleAssistant, Content: "Hi there!"},
			{Role: llm.RoleUser, Content: "How are you?"},
		},
		Language: "en-US",
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	// Simulate CLI predict operation
	chatCtx := ChatContext{
		Messages: input.Messages,
		Language: input.Language,
	}

	probability, err := stub.PredictEndOfTurn(nil, chatCtx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify result
	if probability != 0.95 {
		t.Errorf("Expected probability 0.95, got %f", probability)
	}

	t.Logf("CLI predict test successful: input=%s, probability=%f", inputJSON, probability)
}

func TestRemoteDetectorWithMockServer(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected JSON content type, got %s", r.Header.Get("Content-Type"))
		}

		// Read and parse request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
		}

		var request RemoteRequest
		if err := json.Unmarshal(body, &request); err != nil {
			t.Errorf("Failed to unmarshal request: %v", err)
		}

		// Verify request content
		if len(request.Messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(request.Messages))
		}

		if request.Language != "en-US" {
			t.Errorf("Expected language en-US, got %s", request.Language)
		}

		// Send response
		response := RemoteResponse{
			Probability: 0.92,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create remote detector
	detector := NewRemoteDetector(server.URL, nil)

	// Test prediction
	chatCtx := ChatContext{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hello"},
			{Role: llm.RoleAssistant, Content: "Hi!"},
		},
		Language: "en-US",
	}

	probability, err := detector.PredictEndOfTurn(context.Background(), chatCtx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if probability != 0.92 {
		t.Errorf("Expected probability 0.92, got %f", probability)
	}
}

func TestRemoteDetectorWithFallback(t *testing.T) {
	// Create fallback detector
	fallback := &StubDetector{
		probability: 0.75,
		threshold:   0.85,
		supported:   true,
	}

	// Create remote detector with invalid URL (will fail)
	detector := NewRemoteDetector("http://invalid-url", fallback)

	// Test prediction - should fall back
	chatCtx := ChatContext{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hello"},
		},
		Language: "en-US",
	}

	probability, err := detector.PredictEndOfTurn(context.Background(), chatCtx)
	if err != nil {
		t.Fatalf("Expected no error with fallback, got %v", err)
	}

	// Should get fallback result
	if probability != 0.75 {
		t.Errorf("Expected fallback probability 0.75, got %f", probability)
	}
}

func TestJSONInputOutput(t *testing.T) {
	// Test that our JSON structures work correctly
	testInput := `{"messages": [{"role": "user", "content": "Hello world"}], "language": "en-US"}`
	
	var input struct {
		Messages []llm.Message `json:"messages"`
		Language string         `json:"language,omitempty"`
	}
	
	if err := json.Unmarshal([]byte(testInput), &input); err != nil {
		t.Fatalf("Failed to unmarshal test input: %v", err)
	}
	
	if len(input.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(input.Messages))
	}
	
	if input.Messages[0].Role != llm.RoleUser {
		t.Errorf("Expected user role, got %s", input.Messages[0].Role)
	}
	
	if input.Messages[0].Content != "Hello world" {
		t.Errorf("Expected 'Hello world', got %s", input.Messages[0].Content)
	}
	
	if input.Language != "en-US" {
		t.Errorf("Expected 'en-US', got %s", input.Language)
	}
}