// Tests for the structured slog logging helpers introduced by Phase 5.5-B.
package turn

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

// captureTurnLog runs fn with the default logger replaced by a JSON handler
// writing to a buffer, then decodes and returns the first emitted record.
func captureTurnLog(t *testing.T, fn func()) map[string]any {
	t.Helper()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	prev := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(prev)

	fn()

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("structured log output is not valid JSON: %v\noutput: %q", err, buf.String())
	}
	return entry
}

func TestRecordEouInferenceEmitsStructuredMetrics(t *testing.T) {
	entry := captureTurnLog(t, func() {
		RecordEouInference(0.92, 12*time.Millisecond)
	})

	if got := entry[AttrEouProbability]; got != 0.92 {
		t.Errorf("eou_probability = %v, want 0.92", got)
	}
	if got := entry[AttrInferenceLatencyMs]; got != 12.0 {
		t.Errorf("inference_latency_ms = %v, want 12.0", got)
	}
	if _, ok := entry[AttrEndOfUtteranceDelayMs]; ok {
		t.Errorf("inference record should not carry %s", AttrEndOfUtteranceDelayMs)
	}
}

func TestRecordTurnEndEmitsStructuredMetrics(t *testing.T) {
	entry := captureTurnLog(t, func() {
		RecordTurnEnd(0.95, 10*time.Millisecond, 250*time.Millisecond)
	})

	if got := entry[AttrEouProbability]; got != 0.95 {
		t.Errorf("eou_probability = %v, want 0.95", got)
	}
	if got := entry[AttrInferenceLatencyMs]; got != 10.0 {
		t.Errorf("inference_latency_ms = %v, want 10.0", got)
	}
	if got := entry[AttrEndOfUtteranceDelayMs]; got != 250.0 {
		t.Errorf("end_of_utterance_delay_ms = %v, want 250.0", got)
	}
}

func TestLogTurnEventEmitsStructuredEvent(t *testing.T) {
	entry := captureTurnLog(t, func() {
		LogTurnEvent("turn_ended", slog.Float64(AttrEouProbability, 0.88))
	})

	if got := entry[AttrEvent]; got != "turn_ended" {
		t.Errorf("event = %v, want %q", got, "turn_ended")
	}
	if got := entry[AttrEouProbability]; got != 0.88 {
		t.Errorf("eou_probability = %v, want 0.88", got)
	}
}
