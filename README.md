# LiveKit Agents Go

**Experimental Go framework for building real-time AI agents on LiveKit.**

A prototype Go framework for building real-time AI agents on LiveKit. This is an early-stage implementation and is not production-ready.

## Overview

LiveKit Agents Go is an experimental framework for building AI-powered agents that can interact with users through voice, video, and data channels in real-time. This prototype implementation explores Go patterns for agent development and offers:

- **Plugin Architecture**: Extensible system for AI service integrations
- **Voice Pipeline**: Built-in orchestration for STT, LLM, and TTS services  
- **Worker Management**: Scalable job scheduling and execution
- **CLI Tools**: Development, console, and production modes
- **Type Safety**: Full Go type safety with comprehensive interfaces

## Quick Start

### Getting Started (Experimental)

```bash
# 1. Clone the repository
git clone https://livekit-agents-go
cd livekit-agents-go

# 2. Try console mode (may require troubleshooting)
go run ./examples/basic-agent console

# Note: This is prototype software - expect issues and incomplete features.
```

### Installation for Your Own Project

```bash
go mod init my-agent
go get livekit-agents-go
```

### Basic Agent

```go
package main

import (
    "context"
    "log"
    "os"
    
    "livekit-agents-go/agents"
    "livekit-agents-go/plugins/openai"
)

type MyAgent struct {
    *agents.BaseAgent
}

func (a *MyAgent) Start(ctx context.Context, session *agents.AgentSession) error {
    log.Println("Agent started!")
    return nil
}

func entrypoint(ctx *agents.JobContext) error {
    agent := &MyAgent{BaseAgent: agents.NewBaseAgent("MyAgent")}
    ctx.Session.Agent = agent
    return ctx.Session.Start()
}

func main() {
    // Register OpenAI plugin
    openai.RegisterPlugin(os.Getenv("OPENAI_API_KEY"))
    
    opts := &agents.WorkerOptions{
        EntrypointFunc: entrypoint,
        AgentName:      "MyAgent",
        APIKey:         os.Getenv("LIVEKIT_API_KEY"),
        APISecret:      os.Getenv("LIVEKIT_API_SECRET"),
    }
    
    agents.RunApp(opts)
}
```

### Run Your Agent

```bash
# Development mode with hot reload
go run main.go dev

# Console mode for local testing 
go run ./examples/basic-agent console

# Production mode
go run main.go start

# Test CLI commands
go run main.go test         # Run mock service tests
go run main.go connect      # Connect to existing room
go run main.go download-files  # Download audio files
```

## Console Mode Requirements

**Console mode uses local audio I/O with the LiveKit FFI library:**

```bash
# Console mode - just run it (from project root)
go run ./examples/basic-agent console
```

**Library Setup (Experimental):**
- ⚠️ **FFI library included** - The `liblivekit_ffi.dylib` is bundled but may not work in all environments
- ⚠️ **Basic CGO integration** - Audio processing may be unstable
- ⚠️ **Limited platform support** - Primarily tested on macOS

**What this aims to provide:**
- Local microphone input and speaker output (basic implementation)
- Audio processing via LiveKit's WebRTC components (experimental)
- Echo cancellation and noise suppression (may not work reliably)

**Other modes (dev/start) don't need the FFI library** - they connect to LiveKit rooms where audio processing happens server-side.

## Architecture

### Core Components

- **Agent**: Interface for implementing custom agent behavior
- **Worker**: Manages agent lifecycle and job scheduling  
- **Session**: Orchestrates voice pipeline and room interaction
- **Pipeline**: Configurable processing chain for audio/text
- **Registry**: Plugin system for AI service discovery

### Service Interfaces

- **STT** (Speech-to-Text): Convert audio to text
- **TTS** (Text-to-Speech): Convert text to audio
- **LLM** (Large Language Model): Generate intelligent responses
- **VAD** (Voice Activity Detection): Detect speech in audio

### Plugin System

Plugins provide AI service implementations:

```go
// Register services
openai.RegisterPlugin(apiKey)     // GPT, Whisper, TTS
anthropic.RegisterPlugin(apiKey)  // Claude
deepgram.RegisterPlugin(apiKey)   // Streaming STT

// Use services
stt, _ := plugins.CreateSTT("whisper")
llm, _ := plugins.CreateLLM("gpt-4")
tts, _ := plugins.CreateTTS("openai-tts")
```

## Examples

### Minimal Agent
Simple greeting agent with OpenAI integration:
```bash
cd examples/minimal && go run main.go dev
```

### Voice Assistant  
Full voice pipeline with VAD, STT, LLM, and TTS:
```bash  
cd examples/voice-assistant && go run main.go dev
```

### Weather Agent
Function calling agent with weather API:
```bash
cd examples/weather-agent && go run main.go dev
```

### Echo Agent
Pipeline testing agent for development:
```bash
cd examples/echo-agent && go run main.go dev
```

### STT/TTS Demo
Standalone service demonstration:
```bash
cd examples/stt-tts-demo && go run main.go dev
```

## Environment Variables

```bash
# LiveKit Server
export LIVEKIT_API_KEY="your-api-key"
export LIVEKIT_API_SECRET="your-api-secret"  
export LIVEKIT_HOST="your-server-url"

# AI Services (optional)
export OPENAI_API_KEY="your-openai-key"
export ANTHROPIC_API_KEY="your-anthropic-key"
export DEEPGRAM_API_KEY="your-deepgram-key"
```

## Development

### Project Structure

```
├── agents/           # Core agent framework
├── services/         # Service interfaces (STT, TTS, LLM, VAD)
├── plugins/          # Plugin implementations (OpenAI, Anthropic)
├── media/            # Audio processing utilities  
├── examples/         # Complete working examples
├── test/mock/        # Mock implementations for testing
└── docs/             # Documentation
```

### Building from Source

```bash
git clone https://livekit-agents-go
cd livekit-agents-go
go mod tidy
go test ./...
```

### Creating Plugins

```go
type MyPlugin struct {
    *plugins.BasePlugin
}

func (p *MyPlugin) Register(registry *plugins.Registry) error {
    registry.RegisterSTT("my-stt", func() stt.STT {
        return NewMySTT()
    })
    return nil
}
```

## CLI Reference

### Commands

- `dev`: Development mode with file watching and auto-restart
- `console`: Local testing mode with terminal I/O
- `start`: Production mode with optimizations
- `test`: Run comprehensive service tests with mock implementations
- `connect`: Connect to existing room for testing
- `download-files`: Download audio files for development

### Flags

- `--host`: Server host (default: localhost for dev, 0.0.0.0 for start)
- `--port`: Server port (default: 8080)
- `--api-key`: LiveKit API key
- `--api-secret`: LiveKit API secret
- `--room`: Room name for connect mode
- `--participant-identity`: Participant identity for connect mode

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality  
4. Ensure all tests pass
5. Submit a pull request

## License

Apache 2.0 License - see [LICENSE](LICENSE) for details.

## Links

- [LiveKit](https://livekit.io/) - Real-time communication platform
- [Documentation](https://docs.livekit.io/) - LiveKit documentation
- [Examples](examples/) - Complete example agents
- [Issues](https://livekit-agents-go/issues) - Bug reports and feature requests