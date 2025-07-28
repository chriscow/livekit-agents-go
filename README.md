# LiveKit Agents Go

**Prototype Go framework for building real-time AI agents on [LiveKit](https://livekit.io/).**  
⚠️ This is early-stage, experimental software – expect breaking changes and missing features.

## Overview

LiveKit Agents Go lets you build voice-first or text agents that join a LiveKit room, listen, think, and talk back in real time. It supplies:

- **Voice pipeline** – Speech-to-Text (STT) → Large Language Model (LLM) → Text-to-Speech (TTS)
- **Plugin registry** – Drop-in providers for OpenAI, Silero and lightweight fakes for deterministic tests
- **Job & Worker runtime** – Schedule units of work and connect to the LiveKit Job Queue
- **Turn/VAD utilities** – Detect end-of-utterance for natural conversations
- Written in **pure Go 1.24** (optional ONNX for VAD)

## Quick Start

Clone the repo and run the demo **echo-bot** (repeats whatever you say):

```bash
git clone https://github.com/chriscow/livekit-agents-go.git
cd livekit-agents-go

# Provide at least an OpenAI key for TTS
export OPENAI_API_KEY=<your-key>

# Run the local demo
go run ./examples/echo-bot
```

To connect to a LiveKit deployment instead, use the CLI:

```bash
# Build-and-run the CLI in one go
GO111MODULE=on go run ./cmd/lk-go agent demo \
  --url wss://your-livekit-host \
  --token <access-token> \
  --room demo
```

## Install in your project

```bash
go get github.com/chriscow/livekit-agents-go
```

## Minimal usage example

```go
voiceAgent, _ := agent.New(agent.Config{
    STT:      mySTT,   // implements stt.STT
    TTS:      myTTS,   // implements tts.TTS
    LLM:      myLLM,   // implements llm.LLM
    VAD:      myVAD,   // optional vad.VAD
    MicIn:    micChan, // <-chan rtc.AudioFrame
    TTSOut:   spkChan, // chan<- rtc.AudioFrame
    Language: "en-US",
})

_ = voiceAgent.Start(ctx, myJob) // joins the pipeline and blocks until completion
```

See `examples/echo-bot` for a full working reference implementation.

## CLI reference (`lk-go`)

| Command | Description |
|---------|-------------|
| `agent demo`  | Start the echo demo agent |
| `worker run`  | Connect a worker to the LiveKit Job Queue |
| `stt echo`    | Transcribe a local WAV with the chosen provider |
| `version`     | Print build information |

Run `lk-go --help` for all flags and sub-commands.

## Project structure

```
cmd/lk-go            – CLI entrypoints
examples/echo-bot    – reference agent implementation
internal/worker      – job-queue worker implementation
pkg/agent            – high-level voice agent state machine
pkg/ai               – STT / TTS / LLM / VAD interfaces & fakes
pkg/audio/wav        – WAV helpers
pkg/job              – job & session orchestration
pkg/plugin           – plugin registry (OpenAI, Silero, Fake, …)
pkg/rtc              – audio frame types shared across the project
pkg/turn             – turn-taking / end-of-utterance detection
```

## Environment variables

```
# LiveKit
LIVEKIT_URL=<wss://host>
LIVEKIT_ACCESS_TOKEN=<jwt>

# Optional AI providers
OPENAI_API_KEY=...
ANTHROPIC_API_KEY=...
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests (`go test ./...`)
4. Keep the implementation minimal – this is still an MVP
5. Submit a pull request

## License

Apache-2.0