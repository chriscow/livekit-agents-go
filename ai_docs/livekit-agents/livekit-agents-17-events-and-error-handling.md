LiveKit Docs › Building voice agents › Events & error handling

---

# Events and error handling

> Guides and reference for events and error handling in LiveKit Agents.

## Events

`AgentSession` emits events to notify you of state changes. Each event is emitted with an event object as its sole argument.

### user_input_transcribed

A `UserInputTranscribedEvent` is emitted when user transcription is available.

#### Properties

- `transcript`: str
- `is_final`: bool
- `speaker_id`: str | None - Only available if speaker diarization is supported in your STT plugin.

#### Example

```python
from livekit.agents import UserInputTranscribedEvent

@session.on("user_input_transcribed")
def on_user_input_transcribed(event: UserInputTranscribedEvent):
    print(f"User input transcribed: {event.transcript}, final: {event.is_final}, speaker id: {event.speaker_id}")

```

### conversation_item_added

A `ConversationItemAddedEvent` is emitted when a item is committed to the chat history. This event is emitted for both user and agent items.

#### Properties

- `item`: [ChatMessage](https://github.com/livekit/agents/blob/3ee369e7783a2588cffecc0725e582cac10efa39/livekit-agents/livekit/agents/llm/chat_context.py#L105)

#### Example

```python
from livekit.agents import ConversationItemAddedEvent
from livekit.agents.llm import ImageContent, AudioContent

...

@session.on("conversation_item_added")
def on_conversation_item_added(event: ConversationItemAddedEvent):
    print(f"Conversation item added from {event.item.role}: {event.item.text_content}. interrupted: {event.item.interrupted}")
    # to iterate over all types of content:
    for content in event.item.content:
        if isinstance(content, str):
            print(f" - text: {content}")
        elif isinstance(content, ImageContent):
            # image is either a rtc.VideoFrame or URL to the image
            print(f" - image: {content.image}")
        elif isinstance(content, AudioContent):
            # frame is a list[rtc.AudioFrame]
            print(f" - audio: {content.frame}, transcript: {content.transcript}")

```

### function_tools_executed

`FunctionToolsExecutedEvent` is emitted after all function tools have been executed for a given user input.

#### Methods

- `zipped()` returns a list of tuples of function calls and their outputs.

#### Properties

- `function_calls`: list[[FunctionCall](https://github.com/livekit/agents/blob/3ee369e7783a2588cffecc0725e582cac10efa39/livekit-agents/livekit/agents/llm/chat_context.py#L129)]
- `function_call_outputs`: list[[FunctionCallOutput](https://github.com/livekit/agents/blob/3ee369e7783a2588cffecc0725e582cac10efa39/livekit-agents/livekit/agents/llm/chat_context.py#L137)]

### metrics_collected

`MetricsCollectedEvent` is emitted when new metrics are available to be reported. For more information on metrics, see [Capturing metrics](https://docs.livekit.io/agents/build/metrics.md).

#### Properties

- `metrics`: Union[STTMetrics, LLMMetrics, TTSMetrics, VADMetrics, EOUMetrics]

### speech_created

`SpeechCreatedEvent` is emitted when new agent speech is created. Speech could be created for any of the following reasons:

- the user has provided input
- `session.say` is used to create agent speech
- `session.generate_reply` is called to create a reply

#### Properties

- `user_initiated`: str - True if speech was created using public methods like `say` or `generate_reply`
- `source`: str - "say", "generate_reply", or "tool_response"
- `speech_handle`: [SpeechHandle](https://docs.livekit.io/agents/build/audio.md#speechhandle) - handle to track speech playout.

### agent_state_changed

`AgentStateChangedEvent` is emitted when the agent's state changes. The `lk.agent.state` attribute on the agent participant is updated to reflect the new state, allowing frontend code to easily respond to changes.

#### Properties

- `old_state`: AgentState
- `new_state`: AgentState

#### AgentState

The agent could be in one of the following states:

- `initializing` - agent is starting up. this should be brief.
- `listening` - agent is waiting for user input
- `thinking` - agent is processing user input
- `speaking` - agent is speaking

### user_state_changed

`UserStateChangedEvent` is emitted when the user's state changes. This change is driven by the VAD module running on the user's audio input.

#### Properties

- `old_state`: UserState
- `new_state`: UserState

#### UserState

The user's state can be one of the following:

- `speaking` - VAD detected user has started speaking
- `listening` - VAD detected the user has stopped speaking
- `away` - The user hasn't responded for a while (default: 15s). Specify a custom timeout with `AgentSession(user_away_timeout=...)`.

#### Example

- **[Handling idle user](https://github.com/livekit/agents/blob/main/examples/voice_agents/inactive_user.py)**: Check in with the user after they go idle.

### close

The `CloseEvent` is emitted when the AgentSession has closed and the agent is no longer running. This can occur for several reasons:

- The user ended the conversation
- `session.aclose()` was called
- The room was deleted, disconnecting the agent
- An unrecoverable error occurred during the session

#### Properties

- `error`: LLMError | STTError | TTSError | RealtimeModelError | None - The error that caused the session to close, if applicable

## Handling errors

In addition to state changes, it's important to handle errors that may occur during a session. In real-time conversations, inference API failures can disrupt the flow, potentially leaving the agent unable to continue.

### FallbackAdapter

For STT, LLM, and TTS, the Agents framework includes a `FallbackAdapter` that can fall back to secondary providers if the primary one fails.

When in use, `FallbackAdapter` would:

- automatically resubmit the failed request to backup providers when the primary provider fails
- mark the failed provider as unhealthy and stop sending requests to it
- continue to use the backup providers until the primary provider recovers
- periodically checks the primary provider’s status in the background

```python
from livekit.agents import llm, stt, tts
from livekit.plugins import assemblyai, deepgram, elevenlabs, openai, groq

session = AgentSession(
    stt=stt.FallbackAdapter(
        [
            assemblyai.STT(),
            deepgram.STT(),
        ]
    ),
    llm=llm.FallbackAdapter(
        [
            openai.LLM(model="gpt-4o"),
            openai.LLM.with_azure(model="gpt-4o", ...),
        ]
    ),
    tts=tts.FallbackAdapter(
        [
            elevenlabs.TTS(...),
            groq.TTS(...),
        ]
    ),
)

```

### Error event

`AgentSession` emits `ErrorEvent` when errors occur during the session. It includes an `error` object with a `recoverable` field indicating whether the session will retry the failed operation.

- If `recoverable` is `True`, the event is informational, and the session will continue as expected.
- If `recoverable` is `False` (e.g., after exhausting retries), the session requires intervention. You can handle the error—for instance, by using `.say()` to inform the user of an issue.

#### Properties

- `model_config`: dict - a dictionary representing the current model's configuration
- `error`: [LLMError | STTError | TTSError | RealtimeModelError](https://github.com/livekit/agents/blob/db551d2/livekit-agents/livekit/agents/voice/events.py#L138) - the error that occurred. `recoverable` is a field within `error`.
- `source`: LLM | STT | TTS | RealtimeModel - the source object responsible for the error

### Example

- **[Error handling](https://github.com/livekit/agents/blob/main/examples/voice_agents/error_callback.py)**: Handling unrecoverable errors with a presynthesized message.

---

This document was rendered at 2025-07-18T19:15:13.017Z.
For the latest version of this document, see [https://docs.livekit.io/agents/build/events.md](https://docs.livekit.io/agents/build/events.md).

To explore all LiveKit documentation, see [llms.txt](https://docs.livekit.io/llms.txt).