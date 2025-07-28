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

### 2.5 Audio Core Types & Canonical Requirements

LiveKit’s real-time stack **always processes 10 ms chunks** of 16-bit signed PCM. In practice this means:

* `SamplesPerChannel == SampleRate / 100` – e.g. 480 samples at 48 kHz.
* Allowed `SampleRate` values: **48 000 Hz** (canonical, Opus), plus 16 000 Hz for STT/VAD fallbacks.
* Supported `NumChannels`: **1 (mono)** or **2 (stereo)**.
* `Data` layout: interleaved `int16` little-endian.

Create a single authoritative type in `pkg/rtc` so every package shares the same definition:

```go
// pkg/rtc/audio.go
package rtc

// AudioFrame represents exactly 10 ms of PCM audio.
// Len(Data) == SamplesPerChannel * NumChannels * 2.
// All fields are immutable after creation except Data when processed in-place.
//
// A zero Timestamp means "live"; otherwise it points to absolute wall-clock.
type AudioFrame struct {
    Data              []byte        // 16-bit PCM, little-endian
    SampleRate        int           // 48 000 or 16 000
    SamplesPerChannel int           // SampleRate / 100
    NumChannels       int           // 1 or 2
    Timestamp         time.Duration // optional
}
```

### 2.6 AudioProcessor (Echo Cancellation, NS, AGC)

The AudioProcessor abstracts WebRTC’s `AudioProcessingModule` (AEC3). Later phases will provide the CGO/FFI implementation; **for Phase 3 deliver only the interface and a no-op fake** so downstream code compiles.

```go
// pkg/ai/audio/processor.go
package audio

// Config toggles individual WebRTC sub-modules.
type ProcessorConfig struct {
    EchoCancellation bool
    NoiseSuppression bool
    HighPassFilter   bool
    AutoGainControl  bool
}

type Processor interface {
    // Far-end (speaker output) reference – MUST be 10 ms frames.
    ProcessReverse(frame rtc.AudioFrame) error
    // Near-end (microphone) capture – processed in-place.
    ProcessCapture(frame *rtc.AudioFrame) error

    // Provide measured delay between reverse/capture paths when EC is on.
    SetStreamDelay(d time.Duration) error
    Close() error
}
```

Implement a stub in `pkg/ai/audio/fake` that returns frames unchanged.

### 2.7 AudioGate Helper

During TTS playback the microphone stream may need to be *logically* muted (frames discarded) when interruptions are disabled. Expose a minimal helper so higher-level voice logic does not hard-code that policy:

```go
// pkg/voice/gate.go
type AudioGate interface {
    SetTTSPlaying(playing bool)
    ShouldDiscardAudio() bool // true → drop mic frame
}
```

Provide a default implementation with atomic booleans in `pkg/voice/internal/gate`.

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