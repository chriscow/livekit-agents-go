# STT/TTS Demo Example

A comprehensive demonstration of **Speech-to-Text (STT)** and **Text-to-Speech (TTS)** services in both **single-shot** and **streaming** modes. This example focuses specifically on audio processing capabilities without the complexity of full conversational agents.

## What This Example Demonstrates

- **STT Service Testing**: Speech recognition with various audio formats
- **TTS Service Testing**: Speech synthesis with different text inputs
- **Streaming Capabilities**: Real-time audio processing for both STT and TTS
- **Service Performance**: Processing time measurements and optimization
- **Audio Format Handling**: Multiple sample rates, channels, and bit depths
- **Real vs Mock Services**: Automatic switching based on API key availability

## How It Works

### Demo Structure
The example runs four distinct demonstrations:

1. **Basic STT Demo**: Single-shot speech recognition tests
2. **Basic TTS Demo**: Single-shot speech synthesis tests  
3. **STT Streaming Demo**: Real-time speech recognition
4. **TTS Streaming Demo**: Real-time speech synthesis

### Service Selection
- **With OpenAI API Key**: Uses Whisper STT and OpenAI TTS
- **Without API Key**: Uses mock services for testing
- **Automatic Fallback**: Graceful degradation when services unavailable

## Expected Output

### STT (Speech-to-Text) Tests
```
📝 === STT (Speech-to-Text) Demo ===

📥 STT Test 1: Short audio (0.15s) (4800 bytes, 16000Hz 16bit mono)
   Audio frame: AudioFrame{samples=2400, format={SampleRate:16000 Channels:1 BitsPerSample:16 Format:0}, duration=150ms}
🎙️ Starting Whisper transcription for 4800 bytes of audio (duration: 150ms)
🎯 Whisper transcription result: 'you'  [Note: OpenAI often returns 'you' for silence]
✅ STT result: 'you'
   Confidence: 95.00%, Language: , Final: true
   Processing time: 991ms

📥 STT Test 2: Medium audio (0.2s) (19200 bytes, 48000Hz 16bit mono)
[... similar output ...]

📥 STT Test 3: Long audio (0.25s) (48000 bytes, 48000Hz 16bit stereo)
[... similar output ...]
```

### TTS (Text-to-Speech) Tests
```
🔊 === TTS (Text-to-Speech) Demo ===

🗣️  TTS Test 1: 'Hello world!'
🔊 Starting OpenAI TTS synthesis: 'Hello world!' (voice: alloy, speed: 1.0)
✅ OpenAI TTS completed: 32400 bytes, 0.68s audio, 2.285s processing time
✅ TTS result: 32400 bytes audio
   Duration: 675ms, Format: 24000Hz 16bit mono
   Processing time: 2.285s
   Metadata: map[]

🗣️  TTS Test 2: 'This is a longer sentence that should generate more audio data.'
✅ OpenAI TTS completed: 191400 bytes, 3.99s audio, 3.587s processing time
[... shows larger audio output for longer text ...]
```

### Streaming Demonstrations
```
🎤 === STT Streaming Demo ===
📡 Created STT stream
📤 Sending audio frame 1: 4800 bytes
🎙️ Buffered audio frame: 4800 bytes (total frames: 1)
[... sends 4 frames totaling ~200ms of audio ...]
🔚 CloseSend called - processing final audio buffer (4 frames)
📥 Receiving streaming results...
🔄 Processing 4 buffered audio frames (final: true)
✅ Stream result 1: 'you' (confidence: 95.00%)
📊 Received 1 streaming results

🔊 === TTS Streaming Demo ===
📡 Created TTS stream
📤 Sending text chunk 1: 'Hello there!'
📝 Received text for TTS streaming: 'Hello there!'
[... processes each text chunk ...]
✅ Audio chunk 1: 15600 bytes, duration: 650ms
📊 Received 4 audio chunks, 67200 total bytes
```

## How to Run

### Development Mode (Mock Services)
```bash
go run . console
```
**Expected**: Fast execution (~1 second), predictable mock responses

### Production Mode (Real AI Services)
```bash
export OPENAI_API_KEY="your-api-key-here"
go run . console
```
**Expected**: Slower execution (~30 seconds), real OpenAI API calls

### Other CLI Commands
- `console` - Interactive testing mode (recommended)
- `dev` - Development mode with monitoring
- `start` - Production deployment mode

## Understanding the Results

### STT Results Interpretation

#### With Mock Services
```
STT result: 'Hello, this is a speech recognition test.'
Confidence: 95.00%, Processing time: 50ms
```
- **Text**: Predefined mock responses
- **Confidence**: Always high (0.95)
- **Time**: Very fast (< 100ms)

#### With OpenAI Whisper  
```
STT result: 'you'
Confidence: 95.00%, Processing time: 991ms
```
- **Text**: 'you' is common for silent audio (expected)
- **Confidence**: Whisper's actual confidence score
- **Time**: Real API latency (~1-2 seconds)

### TTS Results Interpretation

#### Audio Size vs Duration
- **"Hello world!"**: 32KB → 675ms audio
- **Longer sentence**: 191KB → 3.99s audio
- **Ratio**: ~48KB per second (24kHz 16-bit mono)

#### Processing Performance
- **Mock TTS**: ~100ms processing time
- **OpenAI TTS**: 2-4 seconds processing time
- **Network factor**: Real APIs include network latency

### Streaming vs Single-Shot

#### STT Streaming Benefits
- **Latency**: Lower latency for long audio
- **Real-time**: Suitable for live conversations
- **Buffering**: Handles network interruptions

#### TTS Streaming Benefits  
- **Start playback early**: Audio chunks available immediately
- **Memory efficiency**: Process long text without buffering all audio
- **Responsiveness**: Better user experience

## Audio Format Details

### Test Configurations
| Test | Duration | Sample Rate | Channels | Bit Depth | Bytes |
|------|----------|-------------|----------|-----------|-------|
| Short | 0.15s | 16kHz | Mono | 16-bit | 4,800 |
| Medium | 0.2s | 48kHz | Mono | 16-bit | 19,200 |
| Long | 0.25s | 48kHz | Stereo | 16-bit | 48,000 |

### Why These Sizes?
- **Minimum Duration**: OpenAI Whisper requires ≥100ms audio
- **Realistic Sizes**: Match typical voice interaction chunks
- **Format Variety**: Tests different audio configurations

## What Success Looks Like

✅ **Perfect Demo Run**:
- All 4 demo sections complete without errors
- STT tests show transcription results (even if silent)
- TTS tests generate substantial audio data (>30KB per test)
- Streaming demos show chunk-by-chunk processing
- No panics or "close of closed channel" errors
- Processing times are reasonable for the service type

✅ **Service Health Indicators**:
- **Mock services**: Complete in <5 seconds total
- **Real services**: Complete in 30-60 seconds total  
- **Audio generation**: TTS produces audio bytes matching text length
- **Error handling**: Graceful failures with informative messages

❌ **Issues to Watch For**:
- Consistent failures across all tests (service integration problem)
- "Audio file too short" errors (audio duration validation issue)
- Stream panics (channel synchronization problem)
- Zero-byte audio output (TTS service failure)

## Technical Architecture

### Service Integration Pattern
```go
// Smart service selection
services, err := plugins.CreateSmartServices()

// Graceful degradation
if services.STT != nil {
    // Run STT tests
} else {
    log.Println("⚠️ No STT service available, skipping STT demos")
}
```

### Audio Processing Pipeline
1. **Frame Creation**: Generate test audio with realistic durations
2. **Service Call**: STT/TTS processing with timing measurement
3. **Result Validation**: Check audio/text output quality
4. **Performance Logging**: Track processing times and data sizes

### Streaming Implementation
- **Buffered Processing**: Audio/text accumulated before processing
- **Asynchronous Results**: Non-blocking send/receive operations
- **Resource Management**: Proper stream lifecycle and cleanup

## Use Cases

### 🧪 Service Development
Perfect for testing new STT/TTS integrations:
```bash
# Test new provider
export NEW_PROVIDER_API_KEY="key"
go run . console
```

### 📊 Performance Benchmarking
Compare service performance across providers:
- Processing time analysis
- Audio quality assessment
- Cost per operation calculation

### 🔍 Audio Format Validation
Ensure services handle various audio formats:
- Different sample rates (16kHz, 48kHz)
- Channel configurations (mono, stereo)
- Bit depths and encodings

### 🐛 Integration Debugging
Isolate issues in audio processing:
- Service connectivity problems
- Audio format compatibility
- Streaming synchronization issues

## Configuration Options

### Environment Variables
- `OPENAI_API_KEY` - Enables real OpenAI services
- `LIVEKIT_*` - LiveKit room integration settings
- `AUDIO_FORMAT_OVERRIDE` - Custom audio format testing

### Demo Customization
Modify test parameters in the code:
```go
// Custom test cases
testCases := []struct {
    name   string
    size   int
    format media.AudioFormat
}{
    {"Custom test", 9600, customFormat},
}
```

## Troubleshooting

### Common Issues

**"Audio file too short" errors?**
- Indicates audio duration < 100ms
- Check audio frame size calculations
- Verify format sample rate configuration

**No audio output in TTS tests?**
- This is expected - demo tests services, doesn't play audio
- Look for audio byte counts in logs (should be >0)
- Consider adding WAV file output for verification

**Streaming tests hanging?**
- Check for channel deadlocks in stream implementation
- Verify CloseSend() is called properly
- Monitor for goroutine leaks

**Inconsistent Whisper results?**
- Silent audio often returns "you" or empty results
- This is normal OpenAI Whisper behavior
- Focus on successful completion, not transcription accuracy

## Next Steps

After validating the STT/TTS demo:
1. Try different API providers by changing environment variables
2. Experiment with the other examples that combine STT/TTS with LLM
3. Add custom audio formats to test specific use cases
4. Integrate real audio file processing for more realistic testing
5. Use the streaming patterns in your own voice applications

## Audio Playback Note

💡 **Why no audio playback?** This demo focuses on **service validation**, not audio playback. The TTS generates real audio data (visible in byte counts), but doesn't play it to speakers. This design keeps the example focused on testing the AI services themselves without requiring OS-specific audio libraries.