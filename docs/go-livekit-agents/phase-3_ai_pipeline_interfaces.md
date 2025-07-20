<!--
Copyright 2024 LiveKit

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->

# Phase 3 – AI Pipeline Interfaces

> Timeline target: **Week 5–6**

Define the Go interfaces for STT, TTS, LLM, and VAD, plus minimal fake implementations so later phases can compile/test without real vendors.

---

## 1. Goals & Deliverables

| ID | Deliverable | Description |
|----|-------------|-------------|
| P3-D1 | `pkg/ai/stt`, `tts`, `llm`, `vad` interfaces | Clean, minimal Go interfaces + capabilities structs |
| P3-D2 | Mock providers | In-memory fakes for tests (`fake_stt` etc.) |
| P3-D3 | Streaming patterns | All audio/text streams use `<-chan` and `chan<-` idioms |
| P3-D4 | Error taxonomy | `ErrRecoverable`, `ErrFatal` sentinel errors |
| P3-D5 | CLI `lk-go stt echo` | Reads WAV, prints transcript using chosen provider |

---

## 2. Interface Design

### 2.1 Speech-to-Text (STT)

```go
type STT interface {
    // Streamed mode – push frames, receive events.
    NewStream(ctx context.Context, cfg StreamConfig) (STTStream, error)
}

type STTStream interface {
    Push(frame rtc.AudioFrame) error
    Events() <-chan SpeechEvent // interim+final results or errors
    CloseSend() error // flush & close
}
```

Keep `StreamConfig` a struct with `SampleRate`, `NumChannels`, `Lang`, `MaxRetry`.

### 2.2 Text-to-Speech (TTS)

Similar pattern but reversed: `Synthesize(ctx, req SynthesizeRequest) (<-chan rtc.AudioFrame, error)`.

### 2.3 Large Language Model (LLM)

Expose both chat and function-call API wrappers:

```go
type LLM interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}
```

### 2.4 Voice Activity Detection (VAD)

```go
type VAD interface {
    Detect(ctx context.Context, frames <-chan rtc.AudioFrame) (<-chan VADEvent, error)
}
```

---

## 3. Implementation Steps

1. **Create `pkg/ai` root** with sub-packages per capability.
2. **Define capability structs** mirroring Python (`STTCapabilities`, etc.).
3. **Implement `fake_*` providers** returning deterministic results for tests.
4. **Add `ErrRecoverable` / `ErrFatal`** to each package and document retry policy.
5. **CLI command** `lk-go stt echo --file sample.wav --provider fake`.

---

## 4. Testing Strategy

### 4.1 Interface Contract Tests

* Table-driven tests verifying that pushing frames after `CloseSend()` returns error.
* VAD detects speech segment within ±20 ms tolerance for provided test file.

### 4.2 Fuzzing

Use `go test -fuzz` on STTStream Push / Events ordering.

---

## 5. Acceptance Criteria

* `go vet` clean; all interfaces documented with examples.
* `fake` providers compile without CGO / external deps.
* CLI `lk-go stt echo` prints expected transcript for provided test WAV.

---

## 6. Python Parity Table

| Capability | Python class | Go interface |
|------------|--------------|--------------|
| STT | `agents.stt.RecognizeStream` | `ai/stt.STTStream` |
| TTS | `agents.tts.TTS` | `ai/tts.TTS` |
| VAD | `plugins.silero.vad.VAD` | `ai/vad.VAD` |

---

_Next: [Phase 4 – Voice Agent Framework](phase-4_voice_agent_framework.md)._ 