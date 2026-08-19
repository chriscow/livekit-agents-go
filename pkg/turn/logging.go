// Structured slog logging for turn detection metrics and events (Phase 5.5-B).
//
// The turn detector already records end-of-utterance (EOU) metrics as expvar
// variables. Per the Phase 5.5-B spec, the same values must also be emitted as
// structured slog entries (JSON when the application installs a JSON handler,
// as done by examples/echo-bot) so log-based observability pipelines can
// observe every inference and turn end. The detector calls RecordEouInference
// after each EOU inference and RecordTurnEnd when a user turn completes.
package turn

import (
	"log/slog"
	"time"
)

// Structured-log attribute keys for turn detection metrics and events.
const (
	// AttrEouProbability is the probability the detector assigned to the
	// hypothesis that the user has finished speaking.
	AttrEouProbability        = "eou_probability"
	// AttrInferenceLatencyMs is the time taken by a single EOU inference,
	// in milliseconds.
	AttrInferenceLatencyMs    = "inference_latency_ms"
	// AttrEndOfUtteranceDelayMs is the time between the user actually
	// finishing speaking and the detector signalling the end of the
	// utterance, in milliseconds.
	AttrEndOfUtteranceDelayMs = "end_of_utterance_delay_ms"
	// AttrEvent is the turn lifecycle event name (for example "turn_ended").
	AttrEvent                 = "event"
)

// durationToMillis converts a duration to fractional milliseconds so
// sub-millisecond inference times are preserved in structured logs.
func durationToMillis(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1e6
}

// RecordEouInference records a single EOU inference as a structured slog entry
// carrying eou_probability and inference_latency_ms. It complements the expvar
// metrics maintained by the detector; both are emitted on every inference.
func RecordEouInference(eouProbability float64, inferenceLatency time.Duration) {
	slog.Info("turn.eou_inference",
		slog.Float64(AttrEouProbability, eouProbability),
		slog.Float64(AttrInferenceLatencyMs, durationToMillis(inferenceLatency)),
	)
}

// RecordTurnEnd records the completion of a user turn as a structured slog
// entry carrying eou_probability, inference_latency_ms and
// end_of_utterance_delay_ms. It complements the expvar metrics maintained by
// the detector.
//
// endOfUtteranceDelay is the elapsed time between the user finishing speaking
// and the detector signalling the end of the utterance; a low value means the
// agent reacts promptly. inferenceLatency is the cost of the EOU inference
// that triggered the turn end.
func RecordTurnEnd(eouProbability float64, inferenceLatency, endOfUtteranceDelay time.Duration) {
	slog.Info("turn.ended",
		slog.Float64(AttrEouProbability, eouProbability),
		slog.Float64(AttrInferenceLatencyMs, durationToMillis(inferenceLatency)),
		slog.Float64(AttrEndOfUtteranceDelayMs, durationToMillis(endOfUtteranceDelay)),
	)
}

// LogTurnEvent records a turn lifecycle event (for example "turn_started" or
// "turn_interrupted") as a structured slog entry with any additional
// attributes supplied by the caller.
func LogTurnEvent(event string, attrs ...slog.Attr) {
	attrs = append(attrs, slog.String(AttrEvent, event))
	slog.Info("turn.event", attrs...)
}
