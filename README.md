# LiveKit Agents Go

**NOTE: This is a work in progress and likely doesn't work fully.**

A robust, idiomatic Go framework for building real-time AI agents on LiveKit.

## Overview

LiveKit Agents Go provides a comprehensive framework for building AI-powered agents that can interact with users through voice, video, and data channels in real-time. Following the proven patterns from the Python LiveKit Agents framework, this Go implementation offers:

- **Plugin Architecture**: Extensible system for AI service integrations
- **Voice Pipeline**: Built-in orchestration for STT, LLM, and TTS services  
- **Worker Management**: Scalable job scheduling and execution
- **CLI Tools**: Development, console, and production modes
- **Type Safety**: Full Go type safety with comprehensive interfaces

## Quick Start

### Installation

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
go run main.go console

# Production mode
go run main.go start
```

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
Simple greeting agent that welcomes participants:
```bash
cd examples/minimal && go run main.go dev
```

### Voice Assistant  
Full voice pipeline with STT, LLM, and TTS:
```bash  
cd examples/voice-assistant && go run main.go dev
```

### Function Calling
Agent with custom function tools:
```bash
cd examples/function-calling && go run main.go dev  
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
├── plugins/          # Plugin implementations
├── media/            # Audio processing utilities  
├── examples/         # Example agents
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

### Flags

- `--host`: Server host (default: localhost for dev, 0.0.0.0 for start)
- `--port`: Server port (default: 8080)
- `--api-key`: LiveKit API key
- `--api-secret`: LiveKit API secret

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