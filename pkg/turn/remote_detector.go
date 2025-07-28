package turn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/llm"
)

// RemoteDetector implements turn detection using a remote HTTP endpoint.
type RemoteDetector struct {
	endpoint   string
	httpClient *http.Client
	fallback   Detector // Optional local fallback detector
}

// NewRemoteDetector creates a new remote turn detector.
func NewRemoteDetector(endpoint string, fallback Detector) *RemoteDetector {
	return &RemoteDetector{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 2 * time.Second, // 2s timeout as specified
		},
		fallback: fallback,
	}
}

// RemoteRequest represents the request payload sent to the remote endpoint.
type RemoteRequest struct {
	Messages []llm.Message `json:"messages"`
	Language string        `json:"language,omitempty"`
}

// RemoteResponse represents the response from the remote endpoint.
type RemoteResponse struct {
	Probability float64 `json:"eou_probability"`
	Error       string  `json:"error,omitempty"`
}

// UnlikelyThreshold returns the language-specific threshold.
// For remote detectors, we use a default or delegate to fallback.
func (d *RemoteDetector) UnlikelyThreshold(language string) (float64, error) {
	if d.fallback != nil {
		return d.fallback.UnlikelyThreshold(language)
	}
	
	// Default threshold for common languages
	switch language {
	case "en-US", "en-GB", "en":
		return 0.85, nil
	default:
		return 0.80, nil // Conservative default
	}
}

// SupportsLanguage returns true if the remote endpoint supports the language.
// We assume remote endpoints support all languages unless fallback says otherwise.
func (d *RemoteDetector) SupportsLanguage(language string) bool {
	if d.fallback != nil {
		return d.fallback.SupportsLanguage(language)
	}
	
	// Assume remote endpoints support common languages
	return true
}

// PredictEndOfTurn sends a request to the remote endpoint for EOU prediction.
func (d *RemoteDetector) PredictEndOfTurn(ctx context.Context, chatCtx ChatContext) (float64, error) {
	// Prepare request payload
	request := RemoteRequest{
		Messages: chatCtx.Messages,
		Language: chatCtx.Language,
	}
	
	requestBody, err := json.Marshal(request)
	if err != nil {
		return d.fallbackPredict(ctx, chatCtx, fmt.Errorf("failed to marshal request: %w", err))
	}
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", d.endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return d.fallbackPredict(ctx, chatCtx, fmt.Errorf("failed to create request: %w", err))
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "livekit-agents-go/turn-detector")
	
	// Send request
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return d.fallbackPredict(ctx, chatCtx, fmt.Errorf("HTTP request failed: %w", err))
	}
	defer resp.Body.Close()
	
	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return d.fallbackPredict(ctx, chatCtx, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body)))
	}
	
	// Parse response
	var response RemoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return d.fallbackPredict(ctx, chatCtx, fmt.Errorf("failed to decode response: %w", err))
	}
	
	// Check for application-level errors
	if response.Error != "" {
		return d.fallbackPredict(ctx, chatCtx, fmt.Errorf("remote error: %s", response.Error))
	}
	
	// Validate probability range
	if response.Probability < 0 || response.Probability > 1 {
		return d.fallbackPredict(ctx, chatCtx, fmt.Errorf("invalid probability: %f", response.Probability))
	}
	
	return response.Probability, nil
}

// fallbackPredict attempts to use the fallback detector if available.
func (d *RemoteDetector) fallbackPredict(ctx context.Context, chatCtx ChatContext, originalErr error) (float64, error) {
	if d.fallback == nil {
		return 0, fmt.Errorf("remote inference failed and no fallback available: %w", originalErr)
	}
	
	// Log the fallback attempt
	fmt.Printf("Remote turn detection failed, using fallback: %v\n", originalErr)
	
	return d.fallback.PredictEndOfTurn(ctx, chatCtx)
}