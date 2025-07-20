# Minimal Worker Example

A **minimal LiveKit worker** that mirrors the Python `minimal_worker.py` example. This demonstrates the simplest possible worker that just connects to LiveKit rooms without any agent logic or voice processing.

## What This Example Demonstrates

- **Basic Room Connection**: Simple worker that connects to LiveKit rooms
- **Worker Lifecycle**: Proper worker setup and room management
- **Environment Configuration**: LiveKit connection using environment variables
- **CLI Integration**: Three execution modes (dev/console/start)
- **Minimal Setup**: No AI services, voice processing, or complex logic

## 🔌 **Room Connection Only**

**This example provides basic room connectivity** including:
- **LiveKit room connection** using provided credentials
- **Worker registration** with LiveKit server
- **Room lifecycle management** (connect, maintain, disconnect)
- **Environment-based configuration** for different deployment scenarios

This exactly matches the functionality of Python `minimal_worker.py`.

## How It Works

### Worker Functionality
The minimal worker simply:
1. **Connects to LiveKit room** using provided credentials
2. **Logs successful connection** with room name
3. **Maintains connection** until worker is terminated
4. **Handles disconnection** gracefully when context is cancelled

### No Additional Features
Unlike other examples, this worker intentionally does NOT include:
- Voice processing (STT, TTS, VAD)
- AI services (LLM integration)
- Agent logic or personality
- Function calling or complex workflows

## Expected Output

### Successful Connection
```
Starting Minimal Worker: MinimalWorker
LiveKit Host: localhost:7880
LiveKit URL: ws://localhost:7880

connected to the room test-room

[Worker maintains connection until terminated]
```

### With Real LiveKit Server
```
Starting Minimal Worker: MinimalWorker
LiveKit Host: your-server.livekit.cloud
LiveKit URL: wss://your-server.livekit.cloud

connected to the room production-room

[Worker stays connected and ready for participants]
```

## How to Run

### Development Mode (Local Server)
```bash
go run . dev
```

### Console Mode (Local Testing)
```bash
go run . console
```

### Production Mode (Real LiveKit Server)
```bash
export LIVEKIT_API_KEY="your-livekit-key"
export LIVEKIT_API_SECRET="your-livekit-secret" 
export LIVEKIT_URL="wss://your-server.livekit.cloud"
go run . start
```

### Available Commands (Matching Python CLI)
- `console` - Local testing mode
- `dev` - Development mode with debug logging
- `start` - Production mode (connects to real LiveKit server)

## What Success Looks Like

✅ **Successful Run Indicators**:
- Worker starts without errors
- Successfully connects to LiveKit room
- Logs "connected to the room {room-name}" message
- Worker maintains connection until terminated
- No crashes or connection errors

❌ **Common Issues**:
- **Connection failed** → Check LIVEKIT_API_KEY and LIVEKIT_API_SECRET
- **Room not found** → Ensure room exists or worker can create rooms
- **Network errors** → Check internet connection and server URL
- **Authentication errors** → Verify API credentials are valid

## Configuration

### Required Environment Variables (Production)
```bash
export LIVEKIT_API_KEY="your-livekit-key"
export LIVEKIT_API_SECRET="your-livekit-secret"
export LIVEKIT_URL="wss://your-server.livekit.cloud"
```

### Optional Environment Variables
```bash
export LIVEKIT_HOST="your-server.livekit.cloud"  # Alternative to LIVEKIT_URL
```

### Development Defaults
If no environment variables are set, the worker defaults to:
- **Host**: `localhost:7880`
- **URL**: `ws://localhost:7880`
- **Room**: Auto-assigned by LiveKit server

## Use Cases

### 1. **Testing LiveKit Connectivity**
Verify your LiveKit server credentials and network connectivity before building complex agents.

### 2. **Worker Framework Validation**
Test the Go agents framework worker lifecycle and CLI integration.

### 3. **Deployment Testing**
Validate worker deployment and configuration in production environments.

### 4. **Base Template**
Use as a starting point for building more complex agents with voice and AI capabilities.

## Next Steps

After validating this minimal worker:
1. **Try basic-agent** for full voice interaction with AI
2. **Test weather-agent** for function calling examples
3. **Explore multi-agent** for complex workflow patterns
4. **Build custom agents** using this as a foundation

## Troubleshooting

**Worker won't start?** Ensure Go 1.21+ is installed and all dependencies are available.

**Connection timeout?** Check that LiveKit server is running and accessible at the specified URL.

**Authentication failed?** Verify your LIVEKIT_API_KEY and LIVEKIT_API_SECRET are correct and have proper permissions.

**Room errors?** Ensure the worker has permission to join rooms on your LiveKit server.