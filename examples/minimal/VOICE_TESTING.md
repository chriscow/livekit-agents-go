# Voice Assistant Testing Guide

## Phase 1.4 Testing: Audio Track Subscription

### Test Setup
1. **Start the Agent**:
   ```bash
   cd examples/minimal
   LIVEKIT_API_KEY=your-key LIVEKIT_API_SECRET=your-secret OPENAI_API_KEY=your-openai-key go run main.go dev
   ```

2. **Generate Test Token**:
   ```bash
   go run main.go -generate-token
   ```

3. **Open Browser Client**:
   - Open `test-client.html` in browser
   - Paste the generated token
   - Click "Connect"
   - Click "Enable Microphone"

### Expected Logs (Phase 1.4 Success Criteria)

When a user enables their microphone, you should see these logs in sequence:

```
🎯 Participant connected: test-user (identity: test-user)
📡 Track published: track-sid from test-user (kind: audio, source: microphone)
🔔 Auto-subscribing to audio track from test-user
🎵 Track subscribed: track-sid from test-user (kind: audio)
🎤 Audio track detected, starting audio processing...
🎤 Starting audio processing for track track-sid from test-user
📊 Received 100 audio packets from test-user (payload type: 111, sequence: 12345)
📊 Received 200 audio packets from test-user (payload type: 111, sequence: 12445)
... (continuing every 100 packets)
```

### Validation Checklist

#### ✅ Track Subscription Working When You See:
- [x] "Track published" log when user enables microphone
- [x] "Auto-subscribing to audio track" log
- [x] "Track subscribed" log immediately after
- [x] "Audio track detected, starting audio processing" log
- [x] "Starting audio processing for track" log

#### ✅ Audio Data Flow Working When You See:
- [x] Regular "Received X audio packets" logs (every 100 packets)
- [x] Incrementing packet counts
- [x] Valid payload types (typically 111 for Opus audio)
- [x] Incrementing sequence numbers

#### ❌ Issues to Watch For:
- No "Track published" log → Browser not publishing audio
- "Failed to subscribe to audio track" error → Subscription permission issue
- No "Track subscribed" log → LiveKit connection issue
- No audio packet logs → RTP reading not working
- "Error reading RTP packet" → Audio processing failure

### Testing Commands

#### Test Agent Startup:
```bash
go run main.go dev
# Should see: "Starting LiveKit Agent: GreetingAgent"
```

#### Test Token Generation:
```bash
go run main.go -generate-token -room "voice-test" -identity "tester"
# Should output a valid JWT token
```

#### Test Compilation:
```bash
go build .
# Should compile without errors
```

### Next Phase Testing

Once Phase 1.4 passes (audio packets received), proceed to:
- **Phase 1.2**: Audio format conversion (RTP → AudioFrame)
- **Phase 1.3**: Audio buffering for STT processing  
- **Phase 2**: STT integration with OpenAI Whisper

### Current Status
- **Phase 1.1**: ✅ COMPLETED - Track subscription handlers added
- **Phase 1.4**: ✅ COMPLETED - Audio packet monitoring (TESTED)
- **Phase 1.2**: ✅ COMPLETED - RTP to AudioFrame conversion
- **Phase 1.3**: ✅ COMPLETED - Audio buffering and streaming  
- **Phase 1.5**: ✅ COMPLETED - AudioFrame creation pipeline (TESTED)
- **Phase 2.1**: ✅ COMPLETED - OpenAI Whisper STT implementation
- **Phase 2.2**: 🔄 READY FOR TESTING - STT speech recognition

## Phase 2.2 Testing: Speech-to-Text Integration

### Expected New Logs (STT Success Criteria)

When you speak into the microphone, you should now see:

```bash
🎙️ Whisper STT service initialized
🎙️ Processing STT batch: 50 frames (~1s) from test-user
🎙️ Starting Whisper transcription for 96000 bytes of audio
🎯 Whisper transcription result: 'Hello, how are you today?'
🎯 STT Result from test-user: 'Hello, how are you today?' (confidence: 0.92)
```

### Testing Steps for STT

1. **Ensure OpenAI API Key**: Make sure OPENAI_API_KEY environment variable is set
2. **Start Agent**: Run with STT enabled
3. **Connect Browser**: Use test-client.html  
4. **Enable Microphone**: Click "Enable Microphone"
5. **Speak Clearly**: Say something like "Hello, can you hear me?"
6. **Wait 1-2 seconds**: STT processes in ~1 second batches

### Validation Checklist for STT

#### ✅ STT Integration Working When You See:
- [x] "Whisper STT service initialized" on startup
- [x] "Processing STT batch: X frames" every ~1 second of speech
- [x] "Starting Whisper transcription for X bytes" 
- [x] "Whisper transcription result: '...'" with actual text
- [x] "STT Result from test-user: '...' (confidence: 0.XX)"

#### ❌ STT Issues to Watch For:
- No "Whisper STT service initialized" → Missing OPENAI_API_KEY
- "STT recognition failed" → OpenAI API error or invalid audio format
- No transcription results → Audio not reaching minimum batch size
- Low confidence scores → Poor audio quality or background noise