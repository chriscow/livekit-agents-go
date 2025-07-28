# Phase 5.5 – Turn Detection

> Timeline target: **Week 10–11**

Add language-aware end-of-utterance (EOU) detection that matches the accuracy of the Python `turn_detector.multilingual` plugin while integrating cleanly with the existing Go VAD layer.

---

## 1. Background & Current State

* **Phase 5** delivered a plugin system and basic VAD (Silero energy-fallback).  
* The Go `Agent` today stops listening after `vad.VADEventSpeechEnd` plus a fixed pause.  
* Python Agents use an ONNX language model that inspects recent dialog turns and outputs an EOU probability, dramatically reducing false interruptions.

Goal: replicate that behaviour in Go with NO architectural guesswork required from contributors.

---

## 2. Goals & Deliverables

| ID | Deliverable | Description |
|----|-------------|-------------|
| P5.5-D1 | `pkg/turn` interface | Go interface mirroring Python `_TurnDetector` (`unlikely_threshold`, `supports_language`, `predict_end_of_turn`) |
| P5.5-D2 | ONNX loader | CPU-only runtime using `onnxruntime-go`; configurable English **and** Multilingual models |
| P5.5-D3 | Model download tool | `lk-go turn download-models` CLI sub-command downloads ONNX **plus** tokenizer.json & languages.json into `${LK_MODEL_PATH}/turn-detector` |
| P5.5-D4 | Agent integration | Replace fixed-pause logic with probability-based turn completion |
| P5.5-D5 | Metrics & logging | Record `eou_probability`, inference latency, and final end-of-turn delay in both `expvar` and structured logs |
| P5.5-D6 | Stand-alone CLI | `lk-go turn predict --json < chat_history.json` prints probability to stdout (used by tests) |
| P5.5-D7 | Remote inference fallback | Support `LIVEKIT_REMOTE_EOT_URL`; if set, detector POSTs to remote endpoint instead of local ONNX |

> No new TODOs or placeholders may remain in the code without written approval from maintainers.

---

## 3. Interface Design

`pkg/turn/detector.go`

```go
package turn

import (
	"context"

	"github.com/chriscow/livekit-agents-go/pkg/ai/llm"
)

type Detector interface {
	// Returns language-specific threshold or nil if language unsupported.
	UnlikelyThreshold(language string) (float64, error)

	// True if the detector has a tuned threshold for this language.
	SupportsLanguage(language string) bool

	// Returns probability (0–1) that the user **has finished** speaking
	// given recent chat context.
	PredictEndOfTurn(ctx context.Context, chatCtx llm.ChatContext) (float64, error)
}
```

*No other public API is required; junior devs must not invent new methods.*

---

## 4. Model Handling

1. **Repo & revisions**

   | Model | Repo | Revision | Size |
   |-------|------|----------|------|
   | English | `livekit/turn-detector` | `v1.2.2-en` | ~200 MB |
   | Multilingual | same | `v0.3.0-intl` | ~400 MB |

2. **Download logic**

   * Use `huggingface_hub-go` (read-only) or a tiny HTTP GET if we bundle URLs.  
   * Store at `${LK_MODEL_PATH:-$HOME/.livekit/models}/turn-detector/<revision>/model_q8.onnx`.  
   * Verify existence and SHA-256 (hash list provided in repo).  
   * Download errors must be fatal – never leave an empty file.

3. **ONNX runtime settings**

   * CPU execution provider only.  
   * `intra_op_num_threads = max(1, runtime.NumCPU()/2)`  
   * `inter_op_num_threads = 1`  
   * `session.dynamic_block_base = 4`

4. **Tokenizer**

   * Uses `tokenizer.json` downloaded from the same HF repository (no CGO).  The tokenizer must apply the **chat template** with `<|im_end|>` markers exactly as in Python.
   * Limit to **128 tokens**, keep at most **6** recent turns, left-truncate.

---

## 5. Agent Wiring

1. New field in `agent.Config`:

   ```go
   TurnDetector turn.Detector // required
   ```

2. Event flow:

   ```
   VAD SpeechEnd  → start VAD-silence timer
                  → when silence ≥ 50 ms, pass chatCtx to detector
                  → if prob ≥ unlikelyThreshold(language) **(absolute thresholds from languages.json)**
                        OR timer ≥ 2 s
                        → finish user turn
                  → else keep feeding audio
   ```

3. `Agent` must measure:

   * `end_of_utterance_delay` – time from VAD SpeechEnd to detector positive.  
   * `transcription_delay` – unchanged.  
   * Log structure identical to Python metrics to simplify dashboards.

---

## 6. CLI Enhancements

```bash
lk-go turn
    download-models            # downloads English + multilingual models
    predict --json …           # reads chat history JSON from stdin, prints probability
```

**Flags**

* `--model english|multilingual` (default `english`)  
* `--threshold 0.85` optional override  
* `--language en-US` hints the detector for faster path
* `--remote-url https://…` overrides `LIVEKIT_REMOTE_EOT_URL`

---

## 7. Testing Strategy

*Unit tests run with a **tiny stub ONNX** (provided in `testdata/turn/stub_model.onnx`, returns deterministic 0.95).*

| Test | Purpose |
|------|---------|
| `detector_load_test.go` | Loads both revisions, checks `SupportsLanguage` table |
| `predict_e2e_test.go` | Uses stub model → feed dummy chatCtx → verify 0.95 prob |
| `agent_turn_test.go` | Simulated mic input, verify Agent waits until probability ≥ threshold then switches to `StateThinking` |
| CLI test | `lk-go turn predict` outputs JSON with `eou_probability` field |
| `remote_fallback_test.go` | Sets `LIVEKIT_REMOTE_EOT_URL` to stub server, detector must call server and return value |

No network calls in CI: use `LK_MODEL_PATH=$TMPDIR/turn-detector-test`.

---

## 8. Acceptance Criteria

* `go test ./...` passes with `-race -count=1`.  
* `lk-go turn download-models` places ONNX **and** `tokenizer.json` & `languages.json`; idempotent.  
* `lk-go agent demo` with `--turn-model english` does **not** interrupt speaker during provided 3-second pause test clip.  
* Memory usage for English model ≤ 300 MB RSS; inference latency ≤ 25 ms on 4-core CPU (benchmarked by `go test -bench . ./pkg/turn`).  
* **No TODO or placeholder** lines remain in any new file.

---

## 9. Documentation

Add section to docs:

```
docs/go-livekit-agents/turn-detection.md
```

covering:

* Conceptual difference between VAD and turn detection  
* Environment variables  
* Example CLI usage  
* Troubleshooting model download

---

## 10. Implementation Notes (for devs)

* Do **not** introduce CGO unless expressly approved.  
* Keep model paths and hashes in `pkg/turn/internal/models.go`.  
* Use `sync.Once` to lazily load ONNX session – share between goroutines.  
* Follow existing logging style (`slog` JSON).

---

Happy building!  With Phase 5.5 complete, the Go Agents stack will match Python’s conversational flow fidelity without premature interruptions.

* All model files and `languages.json` are Apache-2.0 licensed – include NOTICE file when redistributing.
* When `LIVEKIT_REMOTE_EOT_URL` is defined the detector must bypass local ONNX and use remote HTTP with 2 s timeout.
