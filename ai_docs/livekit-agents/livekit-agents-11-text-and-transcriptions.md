LiveKit Docs › Building voice agents › Text & transcriptions

---

# Text and transcriptions

> Integrate realtime text features into your agent.

## Overview

LiveKit Agents supports text inputs and outputs in addition to audio, based on the [text streams](https://docs.livekit.io/home/client/data/text-streams.md) feature of the LiveKit SDKs. This guide explains what's possible and how to use it in your app.

## Transcriptions

When an agent performs STT as part of its processing pipeline, the transcriptions are also published to the frontend in realtime. Additionally, a text representation of the agent speech is also published in sync with audio playback when the agent speaks. These features are both enabled by default when using `AgentSession`.

Transcriptions use the `lk.transcription` text stream topic. They include a `lk.transcribed_track_id` attribute and the sender identity is the transcribed participant.

To disable transcription output, set `transcription_enabled=False` in `RoomOutputOptions`.

### Synchronized transcription forwarding

When both voice and transcription are enabled, the agent's speech is synchronized with its transcriptions, displaying text word by word as it speaks. If the agent is interrupted, the transcription stops and is truncated to match the spoken output.

### Accessing from AgentSession

You can be notified within your agent whenever text input or output is committed to the chat history by listening to the [conversation_item_added](https://docs.livekit.io/agents/build/events.md#conversation_item_added) event.

### TTS-aligned transcriptions

If your TTS provider supports it, you can enable TTS-aligned transcription forwarding to improve transcription synchronization to your frontend. This feature synchronizes the transcription output with the actual speech timing, enabling word-level synchronization. When using this feature, certain formatting may be lost from the original text (dependent on the TTS provider).

Currently, only [Cartesia](https://docs.livekit.io/agents/integrations/tts/cartesia.md) and [ElevenLabs](https://docs.livekit.io/agents/integrations/tts/elevenlabs.md) support word-level transcription timing. For other providers, the alignment is applied at the sentence level and still improves synchronization reliability for multi-sentence turns.

To enable this feature, set `use_tts_aligned_transcript=True` in your `AgentSession` configuration:

```python
session = AgentSession(
    # ... stt, llm, tts, vad, etc...
    use_tts_aligned_transcript=True,
)

```

To access timing information in your code, implement a [transcription_node](https://docs.livekit.io/agents/build/nodes.md#transcription-node) method in your agent. The iterator yields a `TimedString` which includes `start_time` and `end_time` for each word, in seconds relative to the start of the agent's current [turn](https://docs.livekit.io/agents/build/turns.md).

> 🔥 **Experimental feature**
> 
> The `transcription_node` and `TimedString` implementations are experimental and may change in a future version of the SDK.

```python
async def transcription_node(
    self, text: AsyncIterable[str | TimedString], model_settings: ModelSettings
) -> AsyncGenerator[str | TimedString, None]:
    async for chunk in text:
        if isinstance(chunk, TimedString):
            logger.info(f"TimedString: '{chunk}' ({chunk.start_time} - {chunk.end_time})")
        yield chunk

```

## Text input

Your agent also monitors the `lk.chat` text stream topic for incoming text messages from its linked participant. The agent interrupts its current speech, if any, to process the message and generate a new response.

To disable text input, set `text_enabled=False` in `RoomInputOptions`.

## Text-only output

To disable audio output entirely and send text only, set `audio_enabled=False` in `RoomOutputOptions`. The agent will publish text responses to the `lk.transcription` text stream topic, without a `lk.transcribed_track_id` attribute and without speech synchronization.

## Usage examples

This section contains small code samples demonstrating how to use the text features.

For more information, see the [text streams](https://docs.livekit.io/home/client/data/text-streams.md) documentation. For more complete examples, see the [recipes](https://docs.livekit.io/recipes.md) collection.

### Frontend integration

Use the `registerTextStreamHandler` method to receive incoming transcriptions or text:

**JavaScript**:

```typescript
room.registerTextStreamHandler('lk.transcription', async (reader, participantInfo) => {
  const message = await reader.readAll();
  if (reader.info.attributes['lk.transcribed_track_id']) {
    console.log(`New transcription from ${participantInfo.identity}: ${message}`);
  } else {
    console.log(`New message from ${participantInfo.identity}: ${message}`);
  }
});

```

---

**Swift**:

```swift
try await room.registerTextStreamHandler(for: "lk.transcription") { reader, participantIdentity in
    let message = try await reader.readAll()
    if let transcribedTrackId = reader.info.attributes["lk.transcribed_track_id"] {
        print("New transcription from \(participantIdentity): \(message)")
    } else {
        print("New message from \(participantIdentity): \(message)")
    }
}

```

Use the `sendText` method to send text messages:

**JavaScript**:

```typescript
const text = 'Hello how are you today?';
const info = await room.localParticipant.sendText(text, {
  topic: 'lk.chat',
});

```

---

**Swift**:

```swift
let text = "Hello how are you today?"
let info = try await room.localParticipant.sendText(text, for: "lk.chat")

```

### Configuring input/output options

The AgentSession constructor accepts configuration for input and output options:

```python
session = AgentSession(
    ..., # STT, LLM, etc.
    room_input_options=RoomInputOptions(
        text_enabled=False # disable text input
    ), 
    room_output_options=RoomOutputOptions(
        audio_enabled=False # disable audio output
    )
)

```

## Manual text input

To insert text input and generate a response, use the `generate_reply` method of AgentSession: `session.generate_reply(user_input="...")`.

## Transcription events

Frontend SDKs can also receive transcription events via `RoomEvent.TranscriptionReceived`.

> 🔥 **Deprecated feature**
> 
> Transcription events will be removed in a future version. Use [text streams](https://docs.livekit.io/home/client/data/text-streams.md) on the `lk.chat` topic instead.

**JavaScript**:

```typescript
room.on(RoomEvent.TranscriptionReceived, (segments) => {
  for (const segment of segments) {
    console.log(`New transcription from ${segment.senderIdentity}: ${segment.text}`);
  }
});

```

---

**Swift**:

```swift
func room(_ room: Room, didReceiveTranscriptionSegments segments: [TranscriptionSegment]) {
  for segment in segments {
    print("New transcription from \(segment.senderIdentity): \(segment.text)")
  }
}

```

---

**Android**:

```kotlin
room.events.collect { event ->
  if (event is RoomEvent.TranscriptionReceived) {
    event.transcriptionSegments.forEach { segment ->
      println("New transcription from ${segment.senderIdentity}: ${segment.text}")
    }
  }
}

```

---

**Flutter**:

```dart
room.createListener().on<TranscriptionEvent>((event) {
  for (final segment in event.segments) {
    print("New transcription from ${segment.senderIdentity}: ${segment.text}");
  }
});

```

---

This document was rendered at 2025-07-18T19:12:42.531Z.
For the latest version of this document, see [https://docs.livekit.io/agents/build/text.md](https://docs.livekit.io/agents/build/text.md).

To explore all LiveKit documentation, see [llms.txt](https://docs.livekit.io/llms.txt).