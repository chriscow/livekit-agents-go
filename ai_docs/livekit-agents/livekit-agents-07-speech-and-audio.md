LiveKit Docs › Building voice agents › Speech & audio

---

# Agent speech and audio

> Speech and audio capabilities for LiveKit agents.

## Overview

Speech capabilities are a core feature of LiveKit agents, enabling them to interact with users through voice. This guide covers the various speech features and functionalities available for agents.

LiveKit Agents provide a unified interface for controlling agents using both the STT-LLM-TTS pipeline and realtime models.

To learn more and see usage examples, see the following topics:

- **[Text-to-speech (TTS)](https://docs.livekit.io/agents/integrations/tts.md)**: TTS is a synthesis process that converts text into audio, giving AI agents a "voice."

- **[Speech-to-speech](https://docs.livekit.io/agents/integrations/realtime.md)**: Multimodal, realtime APIs can understand speech input and generate speech output directly.

## Preemptive speech generation

**Preemptive generation** allows the agent to begin generating a response before the user's end of turn is committed. The response is based on partial transcription or early signals from user input, helping reduce perceived response delay and improving conversational flow.

When enabled, the agent starts generating a response as soon as the final transcript is available. If the chat context or tools change in the `on_user_turn_completed` [node](https://docs.livekit.io/agents/build/nodes.md#on_user_turn_completed), the preemptive response is canceled and replaced with a new one based on the final transcript.

This feature reduces latency when the following are true:

- [STT node](https://docs.livekit.io/agents/build/nodes.md#stt-node) returns the final transcript faster than [VAD](https://docs.livekit.io/agents/build/turns/vad.md) emits the `end_of_speech` event.
- [Turn detection model](https://docs.livekit.io/agents/build/turns/turn-detector.md) is enabled.

You can enable this feature for STT-LLM-TTS pipeline agents using the `preemptive_generation` parameter for AgentSession:

```python
session = AgentSession(
   preemptive_generation=True,
   ... # STT, LLM, TTS, etc.
)

```

> ℹ️ **Note**
> 
> Preemptive generation doesn't guarantee reduced latency. Use [logging, metrics, and telemetry](https://docs.livekit.io/agents/build/metrics.md)  to validate and fine tune agent performance.

### Example

- **[Preemptive generation example](https://github.com/livekit/agents/blob/main/examples/voice_agents/preemptive_generation.py)**: An example of an agent using preemptive generation.

## Initiating speech

By default, the agent waits for user input before responding—the Agents framework automatically handles response generation.

In some cases, though, the agent might need to initiate the conversation. For example, it might greet the user at the start of a session or check in after a period of silence.

### session.say

To have the agent speak a predefined message, use `session.say()`. This triggers the configured TTS to synthesize speech and play it back to the user.

You can also optionally provide pre-synthesized audio for playback. This skips the TTS step and reduces response time.

> 💡 **Realtime models and TTS**
> 
> The `say` method requires a TTS plugin. If you're using a realtime model, you need to add a TTS plugin to your session or use the [`generate_reply()`](#manually-interrupt-and-generate-responses) method instead.

```python
await session.say(
   "Hello. How can I help you today?",
   allow_interruptions=False,
)

```

#### Parameters

- **`text`** _(str | AsyncIterable[str])_: The text to speak.

 - **`audio`** _(AsyncIterable[rtc.AudioFrame])_ (optional): Pre-synthesized audio to play.

 - **`allow_interruptions`** _(boolean)_ (optional): If `True`, allow the user to interrupt the agent while speaking. (default `True`)

 - **`add_to_chat_ctx`** _(boolean)_ (optional): If `True`, add the text to the agent's chat context after playback. (default `True`)

#### Returns

Returns a [`SpeechHandle`](#speechhandle) object.

#### Events

This method triggers a [`speech_created`](https://docs.livekit.io/agents/build/events.md#speech_created) event.

### generate_reply

To make conversations more dynamic, use `session.generate_reply()` to prompt the LLM to generate a response.

There are two ways to use `generate_reply`:

1. give the agent instructions to generate a response

```python
session.generate_reply(
   instructions="greet the user and ask where they are from",
)

```
2. provide the user's input via text

```python
session.generate_reply(
   user_input="how is the weather today?",
)

```

> ℹ️ **Impact to chat history**
> 
> When using `generate_reply` with `instructions`, the agent uses the instructions to generate a response, which is added to the chat history. The instructions themselves are not recorded in the history.
> 
> In contrast, `user_input` is directly added to the chat history.

#### Parameters

- **`user_input`** _(string)_ (optional): The user input to respond to.

 - **`instructions`** _(string)_ (optional): Instructions for the agent to use for the reply.

 - **`allow_interruptions`** _(boolean)_ (optional): If `True`, allow the user to interrupt the agent while speaking. (default `True`)

#### Returns

Returns a [`SpeechHandle`](#speechhandle) object.

#### Events

This method triggers a [`speech_created`](https://docs.livekit.io/agents/build/events.md#speech_created) event.

## Controlling agent speech

You can control agent speech using the `SpeechHandle` object returned by the `say()` and `generate_reply()` methods, and allowing user interruptions.

### SpeechHandle

The `say()` and `generate_reply()` methods return a `SpeechHandle` object, which lets you track the state of the agent's speech. This can be useful for coordinating follow-up actions—for example, notifying the user before ending the call.

```python

await session.say("Goodbye for now.", allow_interruptions=False)

# the above is a shortcut for 
# handle = session.say("Goodbye for now.", allow_interruptions=False)
# await handle.wait_for_playout()

```

You can wait for the agent to finish speaking before continuing:

```python
handle = session.generate_reply(instructions="Tell the user we're about to run some slow operations.")

# perform an operation that takes time
...

await handle # finally wait for the speech

```

The following example makes a web request for the user, and cancels the request when the user interrupts:

```python
async with aiohttp.ClientSession() as client_session:
    web_request = client_session.get('https://api.example.com/data')
    handle = await session.generate_reply(instructions="Tell the user we're processing their request.")
    if handle.interrupted:
        # if the user interrupts, cancel the web_request too
        web_request.cancel()

```

`SpeechHandle` has an API similar to `ayncio.Future`, allowing you to add a callback:

```python
handle = session.say("Hello world")
handle.add_done_callback(lambda _: print("speech done"))

```

### Getting the current speech handle

The agent session's active speech handle, if any, is available with the `current_speech` property. If no speech is active, this property returns `None`. Otherwise, it returns the active `SpeechHandle`.

Use the active speech handle to coordinate with the speaking state. For instance, you can ensure that a hang up occurs only after the current speech has finished, rather than mid-speech:

```python
# to hang up the call as part of a function call
@function_tool
async def end_call(self, ctx: RunContext):
   """Use this tool when the user has signaled they wish to end the current call. The session ends automatically after invoking this tool."""
   # let the agent finish speaking
   current_speech = ctx.session.current_speech
   if current_speech:
      await current_speech.wait_for_playout()

   # call API to delete_room
   ...

```

### Interruptions

By default, the agent stops speaking when it detects that the user has started speaking. This behavior can be disabled by setting  `allow_interruptions=False` when scheduling speech.

To explicitly interrupt the agent, call the `interrupt()` method on the handle or session at any time. This can be performed even when `allow_interruptions` is set to `False`.

```python
handle = session.say("Hello world")
handle.interrupt()

# or from the session
session.interrupt()

```

## Customizing pronunciation

Most TTS providers allow you to customize pronunciation of words using Speech Synthesis Markup Language (SSML). The following example uses the [tts_node](https://docs.livekit.io/agents/build/nodes.md#tts_node) to add custom pronunciation rules:

** Filename: `agent.py`**

```python
async def tts_node(
    self,
    text: AsyncIterable[str],
    model_settings: ModelSettings
) -> AsyncIterable[rtc.AudioFrame]:
    # Pronunciation replacements for common technical terms and abbreviations.
    # Support for custom pronunciations depends on the TTS provider.
    pronunciations = {
        "API": "A P I",
        "REST": "rest",
        "SQL": "sequel",
        "kubectl": "kube control",
        "AWS": "A W S",
        "UI": "U I",
        "URL": "U R L",
        "npm": "N P M",
        "LiveKit": "Live Kit",
        "async": "a sink",
        "nginx": "engine x",
    }
    
    async def adjust_pronunciation(input_text: AsyncIterable[str]) -> AsyncIterable[str]:
        async for chunk in input_text:
            modified_chunk = chunk
            
            # Apply pronunciation rules
            for term, pronunciation in pronunciations.items():
                # Use word boundaries to avoid partial replacements
                modified_chunk = re.sub(
                    rf'\b{term}\b',
                    pronunciation,
                    modified_chunk,
                    flags=re.IGNORECASE
                )
            
            yield modified_chunk
    
    # Process with modified text through base TTS implementation
    async for frame in Agent.default.tts_node(
        self,
        adjust_pronunciation(text),
        model_settings
    ):
        yield frame

```

** Filename: `Required imports`**

```python
import re
from livekit import rtc
from livekit.agents.voice import ModelSettings
from livekit.agents import tts
from typing import AsyncIterable

```

The following table lists the SSML tags supported by most TTS providers:

| SSML Tag | Description |
| `phoneme` | Used for phonetic pronunciation using a standard phonetic alphabet. These tags provide a phonetic pronunciation for the enclosed text. |
| `say as` | Specifies how to interpret the enclosed text. For example, use `character` to speak each character individually, or `date` to specify a calendar date. |
| `lexicon` | A custom dictionary that defines the pronunciation of certain words using phonetic notation or text-to-pronunciation mappings. |
| `emphasis` | Speak text with an emphasis. |
| `break` | Add a manual pause. |
| `prosody` | Controls pitch, speaking rate, and volume of speech output. |

## Adjusting speech volume

To adjust the volume of the agent's speech, add a processor to the `tts_node` or the `realtime_audio_output_node`.  Alternative, you can also [adjust the volume of playback](https://docs.livekit.io/home/client/tracks/subscribe.md#volume) in the frontend SDK.

The following example agent has an adjustable volume between 0 and 100, and offers a [tool call](https://docs.livekit.io/agents/build/tools.md) to change it.

** Filename: `agent.py`**

```python
class Assistant(Agent):
    def __init__(self) -> None:
        self.volume: int = 50
        super().__init__(
            instructions=f"You are a helpful voice AI assistant. Your starting volume level is {self.volume}."
        )

    @function_tool()
    async def set_volume(self, volume: int):
        """Set the volume of the audio output.

        Args:
            volume (int): The volume level to set. Must be between 0 and 100.
        """
        self.volume = volume

    # Audio node used by STT-LLM-TTS pipeline models
    async def tts_node(self, text: AsyncIterable[str], model_settings: ModelSettings):
        return self._adjust_volume_in_stream(
            Agent.default.tts_node(self, text, model_settings)
        )

    # Audio node used by realtime models
    async def realtime_audio_output_node(
        self, audio: AsyncIterable[rtc.AudioFrame], model_settings: ModelSettings
    ) -> AsyncIterable[rtc.AudioFrame]:
        return self._adjust_volume_in_stream(
            Agent.default.realtime_audio_output_node(self, audio, model_settings)
        )

    async def _adjust_volume_in_stream(
        self, audio: AsyncIterable[rtc.AudioFrame]
    ) -> AsyncIterable[rtc.AudioFrame]:
        stream: utils.audio.AudioByteStream | None = None
        async for frame in audio:
            if stream is None:
                stream = utils.audio.AudioByteStream(
                    sample_rate=frame.sample_rate,
                    num_channels=frame.num_channels,
                    samples_per_channel=frame.sample_rate // 10,  # 100ms
                )
            for f in stream.push(frame.data):
                yield self._adjust_volume_in_frame(f)

        if stream is not None:
            for f in stream.flush():
                yield self._adjust_volume_in_frame(f)

    def _adjust_volume_in_frame(self, frame: rtc.AudioFrame) -> rtc.AudioFrame:
        audio_data = np.frombuffer(frame.data, dtype=np.int16)
        audio_float = audio_data.astype(np.float32) / np.iinfo(np.int16).max
        audio_float = audio_float * max(0, min(self.volume, 100)) / 100.0
        processed = (audio_float * np.iinfo(np.int16).max).astype(np.int16)

        return rtc.AudioFrame(
            data=processed.tobytes(),
            sample_rate=frame.sample_rate,
            num_channels=frame.num_channels,
            samples_per_channel=len(processed) // frame.num_channels,
        )

```

** Filename: `Required imports`**

```python
import numpy as np
from typing import AsyncIterable
from livekit.agents import Agent, function_tool, utils
from livekit.plugins import rtc

```

## Adding background audio

To add more realism to your agent, or add additional sound effects, publish background audio. This audio is played on a separate audio track. The `BackgroundAudioPlayer` class supports on-demand playback of custom audio as well as automatic ambient and thinking sounds synchronized to the agent lifecycle.

For a complete example, see the following recipe:

- **[Background Audio](https://github.com/livekit/agents/blob/main/examples/voice_agents/background_audio.py)**: A voice AI agent with background audio for thinking states and ambiance.

### Create the player

The `BackgroundAudioPlayer` class manages audio playback to a room. It can also play ambient and thinking sounds automatically during the lifecycle of the agent session, if desired.

- **`ambient_sound`** _(AudioSource | AudioConfig | list[AudioConfig])_ (optional): Ambient sound plays on a loop in the background during the agent session. See [Supported audio sources](#audio-sources) and [Multiple audio clips](#multiple-audio-clips) for more details.

- **`thinking_sound`** _(AudioSource | AudioConfig | list[AudioConfig])_ (optional): Thinking sound plays while the agent is in the "thinking" state. See [Supported audio sources](#audio-sources) and [Multiple audio clips](#multiple-audio-clips) for more details.

Create the player within your `entrypoint` function:

```python
from livekit.agents import BackgroundAudioPlayer, AudioConfig, BuiltinAudioClip

# An audio player with automated ambient and thinking sounds
background_audio = BackgroundAudioPlayer(
    ambient_sound=AudioConfig(BuiltinAudioClip.OFFICE_AMBIENCE, volume=0.8),
    thinking_sound=[
        AudioConfig(BuiltinAudioClip.KEYBOARD_TYPING, volume=0.8),
        AudioConfig(BuiltinAudioClip.KEYBOARD_TYPING2, volume=0.7),
    ],
)

# An audio player with a custom ambient sound played on a loop
background_audio = BackgroundAudioPlayer(
    ambient_sound="/path/to/my-custom-sound.mp3",
)

# An audio player for on-demand playback only
background_audio = BackgroundAudioPlayer()

```

### Start and stop the player

Call the `start` method after room connection and after starting the agent session. Ambient sounds, if any, begin playback immediately.

- `room`: The room to publish the audio to.
- `agent_session`: The agent session to publish the audio to.

```python
await background_audio.start(room=ctx.room, agent_session=session)

```

To stop and dispose the player, call the `aclose` method. You must create a new player instance if you want to start again.

```python
await background_audio.aclose()

```

### Play audio on-demand

You can play audio at any time, after starting the player, with the `play` method.

- **`audio`** _(AudioSource | AudioConfig | list[AudioConfig])_: The audio source or a probabilistic list of sources to play. To learn more, see [Supported audio sources](#audio-sources) and [Multiple audio clips](#multiple-audio-clips).

- **`loop`** _(boolean)_ (optional) - Default: `False`: Set to `True` to continuously loop playback.

For example, if you created `background_audio` in the [previous example](#publishing-background-audio), you can play an audio file like this:

```python
background_audio.play("/path/to/my-custom-sound.mp3")

```

The `play` method returns a `PlayHandle` which you can use to await or cancel the playback.

The following example uses the handle to await playback completion:

```python
# Wait for playback to complete
await background_audio.play("/path/to/my-custom-sound.mp3")

```

The next example shows the handle's `stop` method, which stops playback early:

```python
handle = background_audio.play("/path/to/my-custom-sound.mp3")
await(asyncio.sleep(1))
handle.stop() # Stop playback early

```

### Multiple audio clips

You can pass a list of audio sources to any of `play`, `ambient_sound`, or `thinking_sound`. The player selects a single entry in the list based on the `probability` parameter. This is useful to avoid repetitive sound effects. To allow for the possibility of no audio at all, ensure the sum of the probabilities is less than 1.

`AudioConfig` has the following properties:

- **`source`** _(AudioSource)_: The audio source to play. See [Supported audio sources](#audio-sources) for more details.

- **`volume`** _(float)_ (optional) - Default: `1`: The volume at which to play the given audio.

- **`probability`** _(float)_ (optional) - Default: `1`: The relative probability of selecting this audio source from the list.

```python
# Play the KEYBOARD_TYPING sound with an 80% probability and the KEYBOARD_TYPING2 sound with a 20% probability
background_audio.play([
    AudioConfig(BuiltinAudioClip.KEYBOARD_TYPING, volume=0.8, probability=0.8),
    AudioConfig(BuiltinAudioClip.KEYBOARD_TYPING2, volume=0.7, probability=0.2),
])

```

### Supported audio sources

The following audio sources are supported:

#### Local audio file

Pass a string path to any local audio file. The player decodes files with FFmpeg via [PyAV](https://github.com/PyAV-Org/PyAV) and supports all common audio formats including MP3, WAV, AAC, FLAC, OGG, Opus, WebM, and MP4.

> 💡 **WAV files**
> 
> The player uses an optimized custom decoder to load WAV data directly to audio frames, without the overhead of FFmpeg. For small files, WAV is the highest-efficiency option.

#### Built-in audio clips

The following built-in audio clips are available by default for common sound effects:

- `BuiltinAudioClip.OFFICE_AMBIENCE`: Chatter and general background noise of a busy office.
- `BuiltinAudioClip.KEYBOARD_TYPING`: The sound of an operator typing on a keyboard, close to their microphone.
- `BuiltinAudioClip.KEYBOARD_TYPING2`: A shorter version of `KEYBOARD_TYPING`.

#### Raw audio frames

Pass an `AsyncIterator[rtc.AudioFrame]` to play raw audio frames from any source.

## Additional resources

To learn more, see the following resources.

- **[Voice AI quickstart](https://docs.livekit.io/agents/start/voice-ai.md)**: Use the quickstart as a starting base for adding audio code.

- **[Speech related event](https://docs.livekit.io/agents/build/events.md#speech_created)**: Learn more about the `speech_created` event, triggered when new agent speech is created.

- **[LiveKit SDK](https://docs.livekit.io/home/client/tracks/publish.md#publishing-audio-tracks)**: Learn how to use the LiveKit SDK to play audio tracks.

- **[Background audio example](https://github.com/livekit/agents/blob/main/examples/voice_agents/background_audio.py)**: An example of using the `BackgroundAudioPlayer` class to play ambient office noise and thinking sounds.

- **[Text-to-speech (TTS)](https://docs.livekit.io/agents/integrations/tts.md)**: TTS usage and examples for pipeline agents.

- **[Speech-to-speech](https://docs.livekit.io/agents/integrations/realtime.md)**: Multimodal, realtime APIs understand speech input and generate speech output directly.

---

This document was rendered at 2025-07-18T19:11:20.024Z.
For the latest version of this document, see [https://docs.livekit.io/agents/build/audio.md](https://docs.livekit.io/agents/build/audio.md).

To explore all LiveKit documentation, see [llms.txt](https://docs.livekit.io/llms.txt).