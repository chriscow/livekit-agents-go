# LiveKit Agents Documentation Reference

A comprehensive guide to building realtime multimodal and voice AI agents using the LiveKit Agents framework.

## 🚀 Getting Started

### Introduction & Setup
- **[Introduction](livekit-agents/livekit-agents-01-intro.md)** - Framework overview, use cases, and architecture
- **[Voice AI Quickstart](livekit-agents/livekit-agents-02-quickstart.md)** - 10-minute setup guide with complete examples
- **[Building Voice Agents Overview](livekit-agents/livekit-agents-05-overview.md)** - AgentSession, RoomIO, and core capabilities

### Platform Integration
- **[Web & Mobile Frontends](livekit-agents/livekit-agents-04-mobile.md)** - JavaScript, Swift, Android, Flutter SDKs
- **[Telephony Integration](livekit-agents/livekit-agents-03-telephony.md)** - SIP integration for inbound/outbound calls

## 🏗️ Core Building Blocks

### Agent Architecture
- **[Workflows](livekit-agents/livekit-agents-06-workflow.md)** - Multi-agent orchestration and handoff
- **[Pipeline Nodes & Hooks](livekit-agents/livekit-agents-10-pipeline-nodes-and-hooks.md)** - Lifecycle hooks and pipeline customization

### Media Processing
- **[Speech & Audio](livekit-agents/livekit-agents-07-speech-and-audio.md)** - Speech generation, playback control, SSML
- **[Vision](livekit-agents/livekit-agents-08-vision.md)** - Image/video integration and live video processing
- **[Text & Transcriptions](livekit-agents/livekit-agents-11-text-and-transcriptions.md)** - Realtime text, TTS alignment

### Tool System
- **[Tool Definition & Use](livekit-agents/livekit-agents-09-tool-use.md)** - Function tools, external APIs, Model Context Protocol

## 🎯 Advanced Features

### Turn Detection
- **[Turn Detection Overview](livekit-agents/livekit-agents-12-turn-detection-overview.md)** - Conversation flow management
- **[Turn Detection Plugin](livekit-agents/livekit-agents-13-turn-detection-plugin.md)** - Custom turn detection implementation
- **[Silero VAD Plugin](livekit-agents/livekit-agents-14-turn-detection-silero-vad-plugin.md)** - Voice activity detection

### External Data & RAG
- **[External Data & RAG](livekit-agents/livekit-agents-15-external-data-and-rag.md)** - Database integration, RAG implementation, user feedback

### Monitoring & Analytics
- **[Capturing Metrics](livekit-agents/livekit-agents-16-capturing-metrics.md)** - Performance monitoring and analytics
- **[Events & Error Handling](livekit-agents/livekit-agents-17-events-and-error-handling.md)** - Event management and error handling

## 🧪 Testing & Quality

### Testing Framework
- **[Testing & Evaluation](livekit-agents/livekit-agents-18-testing-and-evaluation.md)** - Behavioral testing, pytest integration, CI/CD

## 🔧 Worker Lifecycle

### Worker Management
- **[Worker Lifecycle Overview](livekit-agents/livekit-agents-19-worker-lifecycle-overview.md)** - Worker architecture and lifecycle
- **[Agent Dispatch](livekit-agents/livekit-agents-20-worker-lifecycle-agent-dispatch.md)** - Agent routing and dispatch logic
- **[Job Lifecycle](livekit-agents/livekit-agents-21-worker-lifecycle-job-lifecycle.md)** - Job management and execution
- **[Worker Options](livekit-agents/livekit-agents-22-worker-lifecycle-worker-options.md)** - Configuration and optimization

## 🚀 Deployment & Operations

### Production Deployment
- **[Deployment Overview](livekit-agents/livekit-agents-23-deployment-overview.md)** - Production deployment strategies
- **[Session Recording & Transcripts](livekit-agents/livekit-agents-24-session-recording-and-transcripts.md)** - Recording, transcription, and conversation history

## 📚 Quick Reference

### Common Use Cases
- **Telehealth & Call Centers** - [Telephony Integration](livekit-agents/livekit-agents-03-telephony.md)
- **Mobile Applications** - [Web & Mobile Frontends](livekit-agents/livekit-agents-04-mobile.md)
- **Voice Assistants** - [Voice AI Quickstart](livekit-agents/livekit-agents-02-quickstart.md)
- **Multi-Agent Systems** - [Workflows](livekit-agents/livekit-agents-06-workflow.md)

### Key Concepts
- **AgentSession** - Core orchestration object for agent lifecycle
- **RoomIO** - Media stream management and WebRTC integration
- **Pipeline Nodes** - STT, LLM, TTS processing components
- **Hooks** - Lifecycle events for custom processing
- **Tool System** - Function calling and external API integration

### Supported Providers
- **Speech-to-Text** - Deepgram, AssemblyAI, Azure, Google
- **LLM** - OpenAI, Anthropic, Google, Groq, Ollama
- **Text-to-Speech** - ElevenLabs, OpenAI, Azure, Google
- **Realtime Models** - OpenAI Realtime API, Google Gemini Live

---

*This reference document provides quick access to all LiveKit Agents documentation. Each link leads to detailed implementation guides, code examples, and best practices.*