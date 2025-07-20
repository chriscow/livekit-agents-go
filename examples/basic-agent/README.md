# Basic Agent Example

A **comprehensive voice agent** with weather function calling that mirrors the Python `basic_agent.py` example. This demonstrates the complete agent framework with real LiveKit room integration, voice processing pipeline, and LLM function tools.

## What This Example Demonstrates

- **Complete Voice Agent**: Full conversational AI with personality (Kelly)
- **Weather Function Calling**: LLM can call weather lookup function with location data
- **LiveKit Room Integration**: Real room connection and participant interaction
- **Service Auto-Discovery**: Automatic OpenAI vs mock service selection
- **Three CLI Modes**: dev/console/start modes matching Python agents
- **Agent Lifecycle**: Proper agent entry, function calling, and session management

## 🎤 **Full Voice Interaction**

**This example provides complete voice interaction** including:
- **Real microphone input** (in console mode) 
- **Audio playback** to speakers/headphones
- **LiveKit room integration** for multi-participant conversations
- **Function calling** - agent can look up weather when asked
- **Agent personality** - Kelly is curious, friendly, with humor

This matches the functionality of Python `basic_agent.py` precisely.

## How It Works

### Agent Personality
The basic agent is configured as "Kelly" - a curious, friendly AI with a sense of humor who:
- Keeps responses concise and to the point for voice interaction
- Can look up weather information when asked
- Provides helpful conversational responses
- Matches the personality from Python `basic_agent.py`

### Pipeline Flow (Real Audio)
1. **Audio Input**: Real microphone input from participants (console mode) or LiveKit room
2. **VAD**: Voice Activity Detection on actual audio streams
3. **STT**: Speech-to-Text transcription (Whisper or Deepgram)
4. **Function Calling**: Weather lookup when user asks about weather
5. **LLM**: Conversational response generation (GPT-4 or mock)
6. **TTS**: Text-to-Speech synthesis (OpenAI TTS or mock)
7. **Audio Output**: Real audio playback to speakers/participants

## Expected Output

### Console Mode (Local Audio)
```
Starting Basic Agent: BasicAgent
Services initialized:
  STT: whisper
  LLM: gpt-4o-mini  
  TTS: openai-tts
  VAD: silero

Agent entered session - generating initial reply

🎤 Listening for speech...

[User says: "What's the weather in San Francisco?"]
📝 STT: "What's the weather in San Francisco?"
🌤️ Looking up weather for San Francisco (lat: 37.7749, lon: -122.4194)
🤖 LLM: "The weather in San Francisco is sunny with a temperature of 70 degrees."
🔊 TTS: Playing weather response
```

### Production Mode (Real LiveKit Room)
```
🔍 Auto-discovering plugins...
🔑 Found OpenAI API key, registering OpenAI plugin
✅ Connected to LiveKit room: agent-demo

Services initialized:
  STT: whisper
  LLM: gpt-4o-mini
  TTS: openai-tts
  VAD: silero

Agent entered session - generating initial reply
👋 "Hi! I'm Kelly, your friendly assistant. Ask me about the weather!"

[Participant joins and speaks]
📝 STT: "What's the weather like in Tokyo?"
🌤️ Weather function called for Tokyo
🤖 Kelly: "It's currently 25 degrees Celsius in Tokyo with partly cloudy skies!"
```

## How to Run

### Console Mode (Local Testing - No Server Required)
```bash
# Run from project root - it just works!
go run ./examples/basic-agent console
```

### Development Mode (Hot Reload)
```bash
# Run from project root
export OPENAI_API_KEY="your-api-key-here"
go run ./examples/basic-agent dev
```

### Production Mode (LiveKit Room)
```bash
# Run from project root
export OPENAI_API_KEY="your-api-key-here"
export LIVEKIT_API_KEY="your-livekit-key"
export LIVEKIT_API_SECRET="your-livekit-secret" 
export LIVEKIT_URL="wss://your-server.livekit.cloud"
go run ./examples/basic-agent start
```

### Available Commands (Matching Python CLI)
- `console` - Local audio I/O testing (no LiveKit server required)
- `dev` - Development mode with hot reload and debug logging
- `start` - Production mode (connects to LiveKit room)

## What Success Looks Like

✅ **Successful Run Indicators**:
- Agent connects to room or starts console mode
- Kelly generates initial greeting when agent enters
- Real audio input/output works in console mode
- Weather function calling works when asked about weather
- Conversational responses from Kelly personality
- Services auto-selected based on available API keys
- No panics or crashes during voice interaction

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

## Next Steps

After validating this example works:
1. Try the other examples (echo-agent, weather-agent, stt-tts-demo)
2. Experiment with different OpenAI models by modifying the plugin configuration
3. Add custom personality by editing the `instructions` string
4. Integrate with real LiveKit rooms for live voice interactions

## Troubleshooting

**No audio in console mode?** Ensure your microphone/speakers are working and not muted. Console mode requires local audio hardware.

**Weather function not working?** Set `USE_REAL_WEATHER=true` to call actual weather API, or agent will return mock weather data.

**"Plugin not found" errors?** Ensure you've imported the plugin packages in your imports section.

**High API costs?** This example makes real API calls when OpenAI key is present. Remove `OPENAI_API_KEY` to use mock services for development.

**Agent not responding?** In console mode, speak clearly into your microphone. In production mode, ensure participants are properly connected to the LiveKit room.