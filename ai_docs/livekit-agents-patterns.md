# LiveKit Agents Best Practices & Patterns

*Analysis of 50+ LiveKit Agents examples to identify production-ready patterns and best practices*

## 1. Agent Architecture & Initialization

### Basic Agent Pattern
```python
from livekit.agents import Agent

class MyAgent(Agent):
    def __init__(self, user_data: UserData):
        # Initialize any custom state
        self.user_data = user_data
        
        # Pass instructions to parent
        super().__init__(instructions="Your system instructions here")
    
    # Define function tools as methods
    @function_tool()
    async def my_tool(self, ctx: RunContext, param: str) -> str:
        """Tool description for LLM"""
        return "Tool response"
```

### Multi-Agent Architecture Pattern
```python
# From medical_office_triage example
@dataclass
class UserData:
    """Typed user data for state management"""
    patient_name: str = ""
    appointment_type: str = ""
    agent_type: str = "triage"
    
    def is_valid(self) -> bool:
        return bool(self.patient_name and self.appointment_type)

class BaseAgent(Agent):
    """Shared base class for multiple agents"""
    def __init__(self, user_data: UserData):
        self.user_data = user_data
        super().__init__(instructions=self._get_instructions())
    
    def _get_instructions(self) -> str:
        # Load from external file or build dynamically
        return f"Instructions for {self.user_data.agent_type}"
```

### Environment-Based Configuration
```python
# From multiple examples
def create_agent_config():
    """Environment-based configuration"""
    return {
        "stt": deepgram.STT(model="nova-2", language="en"),
        "llm": openai.LLM(model="gpt-4"),
        "tts": elevenlabs.TTS(voice_id=os.getenv("VOICE_ID")),
        "vad": silero.VAD.load(),
    }
```

## 2. Session Management Patterns

### Standard Session Setup
```python
async def entrypoint(ctx: agents.JobContext):
    # Pre-warm heavy components
    await prewarm()
    
    # Create agent with user data
    user_data = UserData()
    agent = MyAgent(user_data)
    
    # Configure session
    session = AgentSession(
        stt=deepgram.STT(model="nova-2"),
        llm=openai.LLM(model="gpt-4"),
        tts=elevenlabs.TTS(voice_id=VOICE_ID),
        vad=silero.VAD.load(),
        turn_detection="vad",
        user_away_timeout=30.0,
    )
    
    # Start session
    await session.start(ctx.room, agent)
    await ctx.connect()
```

### Session with Room Input Options
```python
# Telephony-specific configuration
await session.start(
    room=ctx.room,
    agent=agent,
    room_input_options=RoomInputOptions(
        noise_cancellation=noise_cancellation.BVCTelephony(),
        # For general use: noise_cancellation.BVC()
    ),
)
```

### Pre-warming Components
```python
async def prewarm():
    """Pre-warm heavy models to reduce first response latency"""
    await asyncio.gather(
        silero.VAD.load(),  # Pre-warm VAD model
        # Add other heavy initializations
    )
```

## 3. Function Tools Best Practices

### Comprehensive Tool Documentation
```python
@function_tool()
async def transfer_to_human(
    self,
    ctx: RunContext,
    reason: str,
    urgency: Literal["low", "medium", "high"] = "medium"
) -> str:
    """Transfer the patient to a human agent.
    
    Args:
        reason: Detailed reason for the transfer (required)
        urgency: Priority level for the transfer
        
    Returns:
        Confirmation message of transfer initiation
    """
    try:
        # Implementation with error handling
        transfer_id = await self._initiate_transfer(reason, urgency)
        return f"Transfer initiated with ID: {transfer_id}"
    except Exception as e:
        logger.error(f"Transfer failed: {e}")
        return f"Transfer failed: {str(e)}"
```

### Error Handling in Tools
```python
@function_tool()
async def book_appointment(self, ctx: RunContext, date: str, time: str) -> str:
    """Book an appointment with proper error handling"""
    try:
        # Validate inputs
        parsed_date = datetime.strptime(date, "%Y-%m-%d")
        if parsed_date < datetime.now():
            return "Error: Cannot book appointments in the past"
        
        # Business logic
        result = await self.booking_service.create_appointment(date, time)
        return f"Appointment booked for {date} at {time}"
        
    except ValueError as e:
        return f"Invalid date format: {str(e)}"
    except BookingError as e:
        return f"Booking failed: {str(e)}"
    except Exception as e:
        logger.error(f"Unexpected booking error: {e}")
        return "Booking temporarily unavailable. Please try again."
```

### Async Tool Patterns
```python
@function_tool()
async def check_inventory(self, ctx: RunContext, product_id: str) -> str:
    """Check product inventory with timeout handling"""
    try:
        # Use timeout for external API calls
        async with asyncio.timeout(10):
            inventory = await self.inventory_api.get_stock(product_id)
            return f"Product {product_id}: {inventory} units available"
    except asyncio.TimeoutError:
        return "Inventory check timed out. Please try again."
```

## 4. Error Handling & Resilience

### Session-Level Error Handling
```python
@session.on("agent_error")
def on_agent_error(event):
    """Handle agent-level errors"""
    logger.error(f"Agent error: {event.error}")
    # Implement fallback behavior
    asyncio.create_task(handle_agent_fallback(event))

async def handle_agent_fallback(error_event):
    """Graceful degradation when agent fails"""
    fallback_message = "I'm experiencing technical difficulties. Let me transfer you to a human agent."
    await session.generate_reply(instructions=f"Say exactly: '{fallback_message}'")
```

### TTS Fallback Pattern
```python
# From multiple examples
class ResilientTTS:
    def __init__(self):
        self.primary_tts = elevenlabs.TTS(voice_id=PRIMARY_VOICE)
        self.fallback_tts = openai.TTS(voice="alloy")
    
    async def synthesize(self, text: str):
        try:
            return await self.primary_tts.synthesize(text)
        except Exception as e:
            logger.warning(f"Primary TTS failed, using fallback: {e}")
            return await self.fallback_tts.synthesize(text)
```

### Database Connection Resilience
```python
class ResilientDatabase:
    """Database with automatic reconnection"""
    def __init__(self):
        self.connection_pool = None
        self.max_retries = 3
    
    async def execute_query(self, query: str, retries: int = 0):
        try:
            return await self._execute(query)
        except ConnectionError as e:
            if retries < self.max_retries:
                await asyncio.sleep(2 ** retries)  # Exponential backoff
                return await self.execute_query(query, retries + 1)
            raise e
```

## 5. Audio/VAD Configuration Patterns

### Telephony-Optimized Configuration
```python
# For phone calls
session = AgentSession(
    stt=deepgram.STT(model="nova-2", language="multi"),
    llm=openai.LLM(model="gpt-4"),
    tts=elevenlabs.TTS(voice_id=TELEPHONY_VOICE_ID),
    vad=silero.VAD.load(),
    turn_detection="vad",
    room_input_options=RoomInputOptions(
        noise_cancellation=noise_cancellation.BVCTelephony(),  # Key difference
    ),
)
```

### Multilingual VAD Configuration
```python
# For multilingual support
from livekit.plugins.silero import MultilingualModel

session = AgentSession(
    vad=silero.VAD.load(model=MultilingualModel()),
    turn_detection="vad",
    # Other components...
)
```

### Realtime API Configuration
```python
# When using OpenAI Realtime API
session = AgentSession(
    llm=openai_realtime.RealtimeAPI(),
    # Note: Don't use separate STT/TTS with Realtime API
    turn_detection="manual",  # Realtime API handles its own detection
)
```

## 6. Event Handling & Lifecycle

### User State Monitoring
```python
@session.on("user_state_changed")
def on_user_state_changed(event):
    """Handle user presence changes"""
    if event.new_state == "away":
        # User has been quiet for configured timeout
        asyncio.create_task(handle_user_silence())
    elif event.new_state == "present":
        # User is back
        logger.info("User returned to conversation")

async def handle_user_silence():
    """Respond to user silence"""
    await session.generate_reply(
        instructions="The user has been quiet. Gently check if they need help."
    )
```

### Room Participant Events
```python
@session.on("participant_connected")
def on_participant_connected(participant):
    """Handle new participant joining"""
    logger.info(f"Participant joined: {participant.identity}")

@session.on("participant_disconnected") 
def on_participant_disconnected(participant):
    """Handle participant leaving"""
    logger.info(f"Participant left: {participant.identity}")
    # Cleanup participant-specific state
```

### Custom Event Emitters
```python
from livekit.agents import EventEmitter

class CustomAgent(Agent, EventEmitter):
    def __init__(self):
        super().__init__()
        EventEmitter.__init__(self)
    
    async def process_data(self, data):
        # Process data...
        result = await self._analyze(data)
        
        # Emit custom event
        self.emit("analysis_complete", result)

# Usage
@agent.on("analysis_complete")
def on_analysis_complete(result):
    logger.info(f"Analysis result: {result}")
```

## 7. Memory & State Management

### Redis Integration Pattern
```python
import redis.asyncio as redis
import json

class AgentMemory:
    def __init__(self):
        self.redis = redis.Redis(host='localhost', port=6379, decode_responses=True)
    
    async def store_conversation(self, session_id: str, data: dict):
        """Store conversation data with expiration"""
        key = f"conversation:{session_id}"
        await self.redis.setex(key, 3600, json.dumps(data))  # 1 hour expiry
    
    async def get_conversation(self, session_id: str) -> dict:
        """Retrieve conversation data"""
        key = f"conversation:{session_id}"
        data = await self.redis.get(key)
        return json.loads(data) if data else {}
```

### SQLite Integration Pattern
```python
import sqlite3
import aiofiles.os

class DatabaseManager:
    def __init__(self, db_path: str):
        self.db_path = db_path
    
    async def init_tables(self):
        """Initialize database schema"""
        conn = sqlite3.connect(self.db_path)
        conn.executescript("""
            CREATE TABLE IF NOT EXISTS patients (
                id INTEGER PRIMARY KEY,
                phone TEXT UNIQUE,
                language TEXT,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
            );
            
            CREATE TABLE IF NOT EXISTS appointments (
                id INTEGER PRIMARY KEY,
                patient_id INTEGER,
                appointment_date TEXT,
                status TEXT,
                FOREIGN KEY (patient_id) REFERENCES patients (id)
            );
        """)
        conn.close()
    
    async def store_patient(self, phone: str, language: str):
        """Store patient data safely"""
        conn = sqlite3.connect(self.db_path)
        try:
            conn.execute(
                "INSERT OR REPLACE INTO patients (phone, language) VALUES (?, ?)",
                (phone, language)
            )
            conn.commit()
        finally:
            conn.close()
```

### UserData Patterns with Validation
```python
from dataclasses import dataclass, field
from typing import Optional, List

@dataclass
class PatientData:
    """Validated patient data structure"""
    phone: str = ""
    name: str = ""
    language: str = "English"
    appointments: List[dict] = field(default_factory=list)
    medical_history: List[str] = field(default_factory=list)
    
    def is_complete(self) -> bool:
        """Validate required fields"""
        return bool(self.phone and self.name)
    
    def add_appointment(self, date: str, time: str, type: str):
        """Add appointment with validation"""
        appointment = {
            "date": date,
            "time": time, 
            "type": type,
            "status": "scheduled"
        }
        self.appointments.append(appointment)
```

## 8. Turn Detection & Conversation Flow

### Dynamic Turn Detection
```python
class AdaptiveAgent(Agent):
    def __init__(self):
        super().__init__()
        self.conversation_mode = "interactive"  # or "listening"
    
    async def switch_to_listening_mode(self, ctx: RunContext):
        """Switch to passive listening"""
        self.conversation_mode = "listening"
        # Override turn detection behavior
        ctx.session.turn_detection = "manual"
        
    async def on_user_turn_completed(self, turn_ctx, new_message):
        """Custom turn handling based on mode"""
        if self.conversation_mode == "listening":
            # Process but don't respond automatically
            await self._process_silently(new_message.text_content)
            raise StopResponse()  # Prevent automatic response
        # Normal mode continues with default behavior
```

### Manual Turn Control
```python
@function_tool()
async def enable_passive_mode(self, ctx: RunContext):
    """Switch to manual turn control"""
    # Store original configuration
    self._original_turn_detection = ctx.session.turn_detection
    
    # Switch to manual control
    ctx.session.turn_detection = "manual"
    
    return "Switched to passive listening mode"

@function_tool() 
async def resume_interactive_mode(self, ctx: RunContext):
    """Resume automatic turn detection"""
    ctx.session.turn_detection = self._original_turn_detection
    return "Resumed interactive mode"
```

## 9. Translation & Multilingual Support

### Pipeline Translation Pattern
```python
class TranslationAgent(Agent):
    def __init__(self, target_language: str = "Spanish"):
        self.target_language = target_language
        self.translator_llm = openai.LLM(model="gpt-4")
        super().__init__()
    
    async def translate_and_speak(self, ctx: RunContext, text: str):
        """Translate text and generate TTS"""
        if self.target_language.lower() != "english":
            translation_prompt = f"Translate this to {self.target_language}: '{text}'"
            translated = await self.translator_llm.chat(translation_prompt)
            await ctx.session.generate_reply(instructions=f"Say exactly: '{translated}'")
        else:
            await ctx.session.generate_reply(instructions=f"Say exactly: '{text}'")
```

### Dynamic Language Switching
```python
@function_tool()
async def switch_language(self, ctx: RunContext, language: str):
    """Switch conversation language"""
    self.current_language = language
    
    # Update TTS voice if needed
    voice_map = {
        "Spanish": "spanish_voice_id",
        "French": "french_voice_id",
        "English": "english_voice_id"
    }
    
    if language in voice_map:
        ctx.session.tts.voice_id = voice_map[language]
    
    return f"Language switched to {language}"
```

## 10. Telephony-Specific Patterns

### SIP Lifecycle Management
```python
@session.on("sip_participant_connected")
def on_sip_connected(participant):
    """Handle SIP participant joining"""
    logger.info(f"SIP call connected: {participant.identity}")
    # Store call metadata
    call_data = {
        "start_time": datetime.now().isoformat(),
        "caller_id": participant.identity,
        "call_type": "inbound"
    }
    asyncio.create_task(store_call_data(call_data))

@session.on("sip_participant_disconnected")
def on_sip_disconnected(participant):
    """Handle SIP participant leaving"""
    logger.info(f"SIP call ended: {participant.identity}")
    # Cleanup and analytics
    asyncio.create_task(finalize_call_session(participant.identity))
```

### Call Transfer with Context
```python
@function_tool()
async def transfer_to_human(self, ctx: RunContext, reason: str, context: str = ""):
    """Transfer call to human agent with context preservation"""
    transfer_data = {
        "reason": reason,
        "conversation_context": context,
        "patient_data": self.user_data.__dict__,
        "timestamp": datetime.now().isoformat()
    }
    
    # Store context for human agent
    await self.memory.store_transfer_context(
        ctx.room.name, 
        transfer_data
    )
    
    # Initiate transfer
    await ctx.session.generate_reply(
        instructions="I'm transferring you to a specialist who can better help you. Please hold."
    )
    
    return f"Transfer initiated: {reason}"
```

### Outbound Calling Pattern
```python
async def initiate_outbound_call(phone_number: str, agent_data: dict):
    """Initiate outbound call with LiveKit SIP"""
    # Create outbound SIP participant
    sip_request = CreateSIPParticipantRequest(
        sip_trunk_id=SIP_TRUNK_ID,
        sip_call_to=phone_number,
        room_name=f"outbound-{phone_number}-{int(time.time())}",
        # Additional configuration
    )
    
    # Start the call
    participant = await livekit_api.create_sip_participant(sip_request)
    logger.info(f"Outbound call initiated to {phone_number}")
    
    return participant
```

### Call Recording Pattern
```python
from livekit import egress

@function_tool()
async def start_call_recording(self, ctx: RunContext):
    """Start recording the call"""
    recording_request = RoomCompositeEgressRequest(
        room_name=ctx.room.name,
        layout="speaker-light",
        audio_only=True,  # For telephony
        file_outputs=[
            EncodedFileOutput(
                file_type=EncodedFileType.MP3,
                filepath=f"recordings/call-{ctx.room.name}.mp3"
            )
        ]
    )
    
    egress_client = egress.EgressServiceClient()
    recording = await egress_client.start_room_composite_egress(recording_request)
    
    return f"Recording started: {recording.egress_id}"
```

## 11. Production Deployment Patterns

### Environment Configuration Management
```python
import os
from typing import Optional

class Config:
    """Centralized configuration management"""
    
    # LiveKit Configuration
    LIVEKIT_URL: str = os.getenv("LIVEKIT_URL", "")
    LIVEKIT_API_KEY: str = os.getenv("LIVEKIT_API_KEY", "")
    LIVEKIT_API_SECRET: str = os.getenv("LIVEKIT_API_SECRET", "")
    
    # AI Services
    OPENAI_API_KEY: str = os.getenv("OPENAI_API_KEY", "")
    DEEPGRAM_API_KEY: str = os.getenv("DEEPGRAM_API_KEY", "")
    ELEVENLABS_API_KEY: str = os.getenv("ELEVENLABS_API_KEY", "")
    
    # Voice Configuration
    VOICE_ID: str = os.getenv("VOICE_ID", "default_voice")
    TELEPHONY_VOICE_ID: str = os.getenv("TELEPHONY_VOICE_ID", "phone_optimized_voice")
    
    # Application Settings
    REDIS_URL: str = os.getenv("REDIS_URL", "redis://localhost:6379")
    DATABASE_URL: str = os.getenv("DATABASE_URL", "sqlite:///app.db")
    LOG_LEVEL: str = os.getenv("LOG_LEVEL", "INFO")
    
    @classmethod
    def validate(cls) -> bool:
        """Validate required configuration"""
        required = ["LIVEKIT_URL", "LIVEKIT_API_KEY", "LIVEKIT_API_SECRET"]
        missing = [key for key in required if not getattr(cls, key)]
        if missing:
            raise ValueError(f"Missing required config: {missing}")
        return True
```

### Monitoring and Metrics
```python
from livekit.agents.metrics import UsageCollector, UsageMetrics
import logging

# Setup logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class MonitoredAgent(Agent):
    def __init__(self):
        super().__init__()
        self.usage_collector = UsageCollector()
    
    async def on_session_start(self, ctx):
        """Track session start"""
        self.usage_collector.collect_agent_metrics(
            UsageMetrics(
                event_type="session_start",
                agent_id=self.__class__.__name__,
                room_name=ctx.room.name
            )
        )
    
    @function_tool()
    async def monitored_function(self, ctx: RunContext, param: str):
        """Function with monitoring"""
        start_time = time.time()
        try:
            result = await self._do_work(param)
            
            # Track successful execution
            self.usage_collector.collect_llm_metrics(
                execution_time=time.time() - start_time,
                tokens_used=len(result),
                success=True
            )
            return result
            
        except Exception as e:
            # Track errors
            self.usage_collector.collect_llm_metrics(
                execution_time=time.time() - start_time,
                success=False,
                error=str(e)
            )
            logger.error(f"Function failed: {e}")
            raise
```

### External Prompt Management
```python
import yaml
from pathlib import Path

class PromptManager:
    """Load prompts from external files"""
    
    def __init__(self, prompts_dir: str = "prompts"):
        self.prompts_dir = Path(prompts_dir)
        self._prompts_cache = {}
    
    def load_prompt(self, prompt_name: str, **kwargs) -> str:
        """Load and format prompt from file"""
        if prompt_name not in self._prompts_cache:
            prompt_file = self.prompts_dir / f"{prompt_name}.yaml"
            if not prompt_file.exists():
                raise FileNotFoundError(f"Prompt file not found: {prompt_file}")
            
            with open(prompt_file, 'r') as f:
                prompt_data = yaml.safe_load(f)
                self._prompts_cache[prompt_name] = prompt_data
        
        prompt_template = self._prompts_cache[prompt_name]['template']
        return prompt_template.format(**kwargs)

# prompts/medical_agent.yaml
"""
template: |
  You are {agent_name}, a medical assistant.
  
  PATIENT INFORMATION:
  - Name: {patient_name}
  - Language: {patient_language}
  
  INSTRUCTIONS:
  - Be professional and empathetic
  - Ask one question at a time
  - Provide clear instructions
"""

# Usage
prompt_manager = PromptManager()
instructions = prompt_manager.load_prompt(
    "medical_agent",
    agent_name="Dr. Smith",
    patient_name="John Doe", 
    patient_language="English"
)
```

### Graceful Shutdown Handling
```python
import signal
import asyncio

class GracefulAgent:
    def __init__(self):
        self.shutdown_event = asyncio.Event()
        self.active_sessions = set()
    
    async def setup_signal_handlers(self):
        """Setup graceful shutdown"""
        loop = asyncio.get_event_loop()
        
        for sig in [signal.SIGTERM, signal.SIGINT]:
            loop.add_signal_handler(sig, self.shutdown_event.set)
    
    async def run_with_shutdown(self):
        """Run agent with graceful shutdown"""
        shutdown_task = asyncio.create_task(self.shutdown_event.wait())
        agent_task = asyncio.create_task(self.run_agent())
        
        # Wait for either completion or shutdown signal
        done, pending = await asyncio.wait(
            [agent_task, shutdown_task],
            return_when=asyncio.FIRST_COMPLETED
        )
        
        if shutdown_task in done:
            logger.info("Shutdown signal received, cleaning up...")
            await self.cleanup_sessions()
        
        # Cancel any pending tasks
        for task in pending:
            task.cancel()
```

## Assessment of PostOp AI Implementation

### ✅ Strengths (Already Following Best Practices)
1. **Telephony Configuration**: Correctly using `BVCTelephony()` for phone optimization
2. **Redis Integration**: Solid memory management with Redis backend
3. **Function Tools**: Comprehensive set of well-documented function tools
4. **Multi-language Support**: Good foundation for translation capabilities
5. **Console Mode Detection**: Smart environment detection for testing vs. production
6. **Error Handling**: Basic try-catch patterns in place

### 🔄 Areas for Improvement

#### 1. Session Event Handling
**Current**: Custom passive mode with manual state management
**Recommended**: Use built-in session events
```python
@session.on("user_state_changed")
def on_user_state_changed(event):
    if event.new_state == "away":
        asyncio.create_task(agent.handle_silence())
```

#### 2. External Prompt Management
**Current**: Inline instruction strings
**Recommended**: External YAML/JSON prompt files
```python
instructions = prompt_manager.load_prompt(
    "postop_agent",
    agent_name=AGENT_NAME,
    patient_language=self.current_language
)
```

#### 3. Usage Monitoring
**Current**: Basic logging
**Recommended**: Structured metrics collection
```python
self.usage_collector = UsageCollector()
# Track LLM usage, session metrics, etc.
```

#### 4. Error Resilience
**Current**: Basic error handling
**Recommended**: Fallback mechanisms
```python
try:
    await primary_service.call()
except ServiceError:
    await fallback_service.call()
```

#### 5. Configuration Management
**Current**: Direct environment variable access
**Recommended**: Centralized config validation
```python
class Config:
    @classmethod
    def validate(cls) -> bool:
        # Validate all required settings
        pass
```

### 🎯 Quick Wins for PostOp AI

1. **Add session event handlers** for user state monitoring
2. **Implement usage metrics collection** for production monitoring
3. **Extract prompts** to external YAML files
4. **Add TTS fallback** for service reliability
5. **Centralize configuration** with validation
6. **Add graceful shutdown** handling

The current PostOp implementation is already quite solid and follows many LiveKit best practices. The suggested improvements would primarily enhance production readiness, monitoring capabilities, and maintainability rather than fix fundamental architectural issues.