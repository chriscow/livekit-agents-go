# Pipeline Test Example

A **service integration testing agent** that validates the voice processing pipeline components without actual audio I/O. This example tests that STT, LLM, TTS, and VAD services work correctly together using simulated audio data.

## What This Example Demonstrates

- **Service Integration Testing**: Validates STT, LLM, TTS, VAD services work together
- **Plugin System**: Automatic service selection (real AI services vs mocks)  
- **Pipeline Simulation**: Tests the voice processing flow with simulated audio data
- **Environment-Based Configuration**: Seamless dev/production switching
- **AI Service Validation**: Confirms OpenAI/mock services integrate correctly
- **Error Handling**: Tests service error scenarios and recovery

## ⚠️ **Important: No Audio I/O**

**You will NOT hear any audio** from this example because:
- It creates **silent audio frames** (zeros) for testing
- It **does not play TTS output** to speakers
- It **does not accept microphone input**
- It's designed to **test AI services**, not provide voice interaction

For actual voice interaction, use the `basic-agent` or other voice examples.

## How It Works

### Testing Philosophy
This example prioritizes **service integration validation** over user interaction:
- **Simulated Audio**: Uses silent frames to test VAD behavior
- **Service Workflows**: Tests complete STT → LLM → TTS pipeline
- **Error Scenarios**: Validates error handling and recovery
- **Performance**: Measures service response times and token usage

### Pipeline Flow (Simulated)
1. **Audio Input**: Creates silent audio frames (1024 bytes of zeros)
2. **VAD**: Tests voice detection on silent audio (will show ~35-45% speech probability)
3. **STT**: Would transcribe audio if speech detected (skipped due to no speech)
4. **LLM**: Would generate responses if speech detected (skipped due to no speech)  
5. **TTS**: Would synthesize audio if responses generated (skipped due to no speech)

**Note**: With silent audio, VAD correctly detects no speech, so most pipeline steps are skipped.

## Expected Output

### Typical Run (Silent Audio Input)
```
Starting Pipeline Test Agent: PipelineTestAgent
Services initialized for pipeline testing:
  STT: whisper v1.0.0 (or mock)
  LLM: gpt v1.0.0 (or mock)
  TTS: openai-tts v1.0 (or mock)
  VAD: mock-vad v1.0.0

=== Pipeline Test 1 ===
📥 Received audio frame: 1024 bytes
🎤 VAD detection: speech=35-45%, is_speech=false
⏭️ No speech detected, skipping test (expected with silent audio)

=== Pipeline Test 2 ===
📥 Received audio frame: 1024 bytes
🎤 VAD detection: speech=41.63%, is_speech=false
⏭️ No speech detected, skipping test (expected with silent audio)

🎉 Pipeline test simulation completed - all services working!
```

### With Real AI Services (API Keys Present)
```
🔍 Auto-discovering plugins...
🔑 Found OpenAI API key, registering OpenAI plugin
✅ OpenAI plugin registered successfully (STT: 1, TTS: 1, LLM: 5)

Services initialized for pipeline testing:
  STT: whisper v1.0.0
  LLM: gpt v1.0.0
  TTS: openai-tts v1.0
  VAD: mock-vad v1.0.0

=== Pipeline Test 1 ===
🎙️ Starting Whisper transcription...
🎯 Whisper result: (actual transcription if speech detected)
🤖 GPT-4o response: (intelligent conversation)
✅ OpenAI TTS completed: 45000 bytes, 2.1s audio
```

## How to Run

### Development Mode (Mock Services)
```bash
go run . console
```

### Production Mode (Real AI Services)
```bash
export OPENAI_API_KEY="your-api-key-here"
go run . console
```

### Available Commands
- `console` - Interactive console mode for testing
- `dev` - Development mode with hot reload
- `start` - Production mode (requires LiveKit connection)

## What Success Looks Like

✅ **Successful Run Indicators**:
- All 3 pipeline tests complete without errors
- VAD correctly detects no speech in silent audio (~35-45% probability)
- All tests are skipped due to no speech detection (expected behavior)
- No panics or crashes  
- Services auto-selected based on available API keys
- "Pipeline test simulation completed" message appears

❌ **Common Issues**:
- Missing API keys → Falls back to mock services (expected behavior)
- Network errors → Check internet connection and API key validity
- Build errors → Ensure Go 1.21+ and all dependencies installed

## Technical Architecture

### Service Integration
- **Automatic Plugin Discovery**: Detects available AI services via environment variables
- **Smart Fallbacks**: Uses real services when available, mocks when not
- **Thread-Safe Operations**: Concurrent service calls with proper synchronization
- **Error Resilience**: Graceful handling of API failures

### Configuration
- **Environment Variables**: `OPENAI_API_KEY`, `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`
- **LiveKit Settings**: Configurable room connection for production use
- **Agent Metadata**: Version, description, and capability information

## Use Cases

### 1. **Service Validation**
Test that all voice processing services are correctly installed and configured.

### 2. **Integration Testing**
Validate that the complete voice pipeline works end-to-end before deploying agents.

### 3. **Development Debugging**
Debug service integration issues without the complexity of real audio handling.

### 4. **CI/CD Pipeline**
Automated testing of service configurations in continuous integration environments.

## Next Steps

After validating this pipeline test works:
1. Try the **basic-agent** example for real voice interactions
2. Test **weather-agent** for function calling capabilities  
3. Experiment with different OpenAI models by modifying plugin configuration
4. Use this as a foundation for building custom service integration tests

## Troubleshooting

**All tests skipped?** This is expected behavior with silent audio. VAD correctly detects no speech.

**Service errors?** Check your API keys and network connectivity. Services will fall back to mocks if real ones fail.

**"Plugin not found" errors?** Ensure you've imported the plugin packages in your imports section.

**High API costs?** This example makes real API calls when keys are present. Monitor your usage or use mock mode for development.

**Want to test with real audio?** Use the `basic-agent` example instead - this is specifically for service testing without audio I/O.