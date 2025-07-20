# Weather Agent Example

A sophisticated voice agent demonstrating **function calling capabilities** with real-world API integration. This example showcases how AI agents can access external services and provide dynamic, data-driven responses through voice interaction.

## What This Example Demonstrates

- **Function Calling**: Integration between LLM and external APIs
- **Real Weather Data**: Live weather API integration with Open-Meteo
- **Voice-Driven Queries**: Natural language weather requests via speech
- **Dynamic Responses**: Context-aware replies based on real data
- **Error Handling**: Graceful fallback between real and mock data
- **Multi-Location Support**: Weather queries for different cities

## How It Works

### Agent Capabilities
The weather agent is designed to:
- Accept natural language weather queries via voice
- Extract location information from speech
- Call real weather APIs to fetch current conditions
- Provide conversational responses with actual weather data

### Function Calling Flow
1. **Voice Input**: "What's the weather like in San Francisco?"
2. **STT**: Transcribes speech to text
3. **LLM**: Analyzes request and identifies need for weather data
4. **Function Call**: Calls `getWeather()` with location parameters
5. **API Integration**: Fetches live weather from Open-Meteo API
6. **Response Generation**: LLM creates natural response with weather data
7. **TTS**: Synthesizes complete response to audio

### Weather Function
```go
// Example function signature
func getWeather(location, latitude, longitude string) (*WeatherResult, error)
```

## Expected Output

### Successful Weather Query
```
🌤️ Weather Agent starting...

Services initialized for weather agent:
  STT: whisper v1.0.0
  LLM: gpt v1.0.0 (with function calling)
  TTS: openai-tts v1.0
  VAD: mock-vad v1.0.0

=== Weather Query 1: San Francisco ===
📥 Received audio frame: 1024 bytes
🎤 VAD detection: speech=90.0%, is_speech=true
📝 STT result: 'What's the weather like?' (confidence: 0.95)

🌡️ Calling weather function for San Francisco (lat: 37.7749, lon: -122.4194)
🌤️ Weather function result: 18.5°C in San Francisco

🤖 LLM response: 'I'd be happy to check the weather for you! Let me get the current conditions.'
🔊 TTS synthesized: 45600 bytes audio, duration: 2.1s

Weather query 1 completed successfully
```

### Real vs Mock Weather Data

**With Real Weather API** (`USE_REAL_WEATHER=true`):
```
🌡️ Calling real weather API: https://api.open-meteo.com/v1/forecast?latitude=37.7749&longitude=-122.4194&current=temperature_2m
🌤️ Real weather data: 18.5°C in San Francisco
```

**With Mock Weather Data** (default):
```
🌡️ Using mock weather data
🌤️ Mock weather data: 18.5°C in San Francisco
```

## How to Run

### Development Mode (Mock Weather)
```bash
go run . console
```

### With Real Weather API
```bash
export USE_REAL_WEATHER=true
go run . console
```

### With Real AI Services
```bash
export OPENAI_API_KEY="your-api-key-here"
export USE_REAL_WEATHER=true
go run . console
```

## Supported Locations

The example includes predefined test locations:

| Location | Coordinates | Expected Results |
|----------|-------------|------------------|
| **San Francisco** | 37.7749, -122.4194 | Mild coastal climate |
| **New York** | 40.7128, -74.0060 | Continental climate |
| **Tokyo** | 35.6762, 139.6503 | Humid subtropical |

### Mock Temperature Data
- San Francisco: 18.5°C
- New York: 22.3°C  
- Tokyo: 25.7°C
- Default (unknown): 20.0°C

## What Success Looks Like

✅ **Function Calling Success**:
- All 3 weather queries complete without errors
- Each query shows the full pipeline with function call
- Weather data is retrieved (real or mock)
- LLM incorporates weather data into natural responses
- Audio responses are generated with weather information

✅ **API Integration Health**:
- Weather function calls complete quickly (< 2 seconds)
- Real API calls return actual temperature data
- Mock fallback works when API is unavailable
- Error handling gracefully manages API failures

❌ **Common Issues**:
- Network timeouts with real weather API (expected occasionally)
- Missing location coordinates (falls back to default)
- API rate limiting (switches to mock data)

## Configuration Options

### Environment Variables
- `USE_REAL_WEATHER=true` - Enables real Open-Meteo API calls
- `OPENAI_API_KEY` - Enables real AI services
- `WEATHER_API_TIMEOUT=10s` - Weather API timeout (default: 10s)

### Weather API Details
- **Provider**: Open-Meteo (free, no API key required)
- **Endpoint**: `https://api.open-meteo.com/v1/forecast`
- **Data**: Current temperature in Celsius
- **Rate Limits**: Generous free tier, no authentication required

## Function Calling Architecture

### LLM Integration
```go
// Example message flow
messages := []llm.Message{
    {Role: llm.RoleSystem, Content: "You are a helpful weather assistant..."},
    {Role: llm.RoleUser, Content: "What's the weather in Tokyo?"},
    {Role: llm.RoleFunction, Content: "Current weather in Tokyo: 25.7°C"},
}
```

### Weather Function Implementation
- **Error Resilience**: Automatic fallback to mock data
- **Caching**: Results could be cached for performance
- **Validation**: Input sanitization and coordinate validation
- **Logging**: Comprehensive request/response logging

## Use Cases

### 🗣️ Natural Language Weather Queries
Users can ask weather questions naturally:
- "What's the temperature outside?"
- "How's the weather in New York?"
- "Tell me the current conditions in Tokyo"

### 🔧 Function Calling Development
Perfect template for adding new function capabilities:
- News queries
- Stock prices  
- Calendar integration
- Smart home control

### 🧪 API Integration Testing
Validates external service integration patterns:
- HTTP client configuration
- Error handling strategies
- Timeout management
- Fallback mechanisms

## Advanced Features

### Multi-Turn Conversations
The agent maintains context across multiple weather queries:
```
User: "What's the weather like?"
Agent: "I'd be happy to help! Which location would you like to know about?"
User: "San Francisco"
Agent: "The current temperature in San Francisco is 18.5°C..."
```

### Error Recovery
Robust handling of various failure scenarios:
- Network connectivity issues
- Invalid location requests
- API service outages  
- Malformed responses

## Extending the Example

### Adding New Functions
```go
// Example: Add time function
func getCurrentTime(timezone string) (*TimeResult, error) {
    // Implementation
}
```

### Supporting More Weather Data
- Humidity levels
- Wind speed/direction
- Weather conditions (sunny, cloudy, etc.)
- 7-day forecasts

### Advanced LLM Integration
- Tool use with OpenAI function calling
- Structured output formatting
- Multi-step reasoning chains

## Troubleshooting

**Weather queries failing?** Check internet connectivity and try mock mode first.

**Inconsistent LLM responses?** This is normal - real LLMs provide varied responses.

**Long response times?** Weather API calls add 1-2 seconds; this is expected.

**"Function not called" errors?** The LLM integration simulates function calling - actual tool use requires additional implementation.

## Next Steps

After validating the weather agent:
1. Experiment with `USE_REAL_WEATHER=true` to see live data
2. Modify the locations array to test your local area
3. Extend with additional weather parameters
4. Use as a template for other function calling scenarios
5. Integrate with actual OpenAI function calling tools