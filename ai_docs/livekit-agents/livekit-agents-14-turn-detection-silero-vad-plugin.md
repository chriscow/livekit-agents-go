LiveKit Docs › Building voice agents › Turn detection & interruptions › Silero VAD plugin

---

# Silero VAD plugin

> High-performance voice activity detection for LiveKit Agents.

## Overview

The Silero VAD plugin provides voice activity detection (VAD) that contributes to accurate [turn detection](https://docs.livekit.io/agents/build/turns.md) in voice AI applications.

VAD is a crucial component for voice AI applications as it helps determine when a user is speaking versus when they are silent. This enables natural turn-taking in conversations and helps optimize resource usage by only performing speech-to-text while the user speaks.

LiveKit recommends using the Silero VAD plugin in combination with the custom [turn detector model](https://docs.livekit.io/agents/build/turns/turn-detector.md) for the best performance.

## Quick reference

The following sections provide a quick overview of the Silero VAD plugin. For more information, see [Additional resources](#additional-resources).

### Requirements

The model runs locally on the CPU and requires minimal system resources.

### Installation

Install the plugin from PyPI:

```bash
pip install "livekit-agents[silero]~=1.0"

```

### Download model weights

You must download the model weights before running your agent for the first time:

```bash
python agent.py download-files

```

### Usage

Initialize your `AgentSession` with the Silero VAD plugin:

```python
from livekit.plugins import silero

session = AgentSession(
    vad=silero.VAD.load(),
    # ... stt, tts, llm, etc.
)

```

## Prewarm

You can [prewarm](https://docs.livekit.io/agents/worker/options.md#prewarm) the plugin to improve load times for new jobs:

```python
async def entrypoint(ctx: agents.JobContext):
    await ctx.connect()

    session = AgentSession(
        vad=ctx.proc.userdata["vad"],
        # ... stt, tts, llm, etc.
    )

    # ... session.start etc ...

def prewarm(proc: agents.JobProcess):
    proc.userdata["vad"] = silero.VAD.load()

if __name__ == "__main__":
    agents.cli.run_app(
        agents.WorkerOptions(entrypoint_fnc=entrypoint, prewarm_fnc=prewarm)
    )


```

## Configuration

The following parameters are available on the `load` method:

- **`min_speech_duration`** _(float)_ (optional) - Default: `0.05`: Minimum duration of speech required to start a new speech chunk.

- **`min_silence_duration`** _(float)_ (optional) - Default: `0.55`: Duration of silence to wait after speech ends to determine if the user has finished speaking.

- **`prefix_padding_duration`** _(float)_ (optional) - Default: `0.5`: Duration of padding to add to the beginning of each speech chunk.

- **`max_buffered_speech`** _(float)_ (optional) - Default: `60.0`: Maximum duration of speech to keep in the buffer (in seconds).

- **`activation_threshold`** _(float)_ (optional) - Default: `0.5`: Threshold to consider a frame as speech. A higher threshold results in more conservative detection but might potentially miss soft speech. A lower threshold results in more sensitive detection, but might identify noise as speech.

- **`sample_rate`** _(Literal[8000, 16000])_ (optional) - Default: `16000`: Sample rate for the inference (only 8KHz and 16KHz are supported).

- **`force_cpu`** _(bool)_ (optional) - Default: `True`: Force the use of CPU for inference.

## Additional resources

The following resources provide more information about using the LiveKit Silero VAD plugin.

- **[Python package](https://pypi.org/project/livekit-plugins-silero/)**: The `livekit-plugins-silero` package on PyPI.

- **[Plugin reference](https://docs.livekit.io/reference/python/v1/livekit/plugins/silero/index.html.md#livekit.plugins.silero.VAD)**: Reference for the LiveKit Silero VAD plugin.

- **[GitHub repo](https://github.com/livekit/agents/tree/main/livekit-plugins/livekit-plugins-silero)**: View the source or contribute to the LiveKit Silero VAD plugin.

- **[Silero VAD project](https://github.com/snakers4/silero-vad)**: The open source VAD model that powers the LiveKit Silero VAD plugin.

- **[Transcriber](https://github.com/livekit-examples/python-agents-examples/tree/main/pipeline-stt/transcriber.py)**: An example using standalone VAD and STT outside of an `AgentSession`.

---

This document was rendered at 2025-07-18T19:14:12.517Z.
For the latest version of this document, see [https://docs.livekit.io/agents/build/turns/vad.md](https://docs.livekit.io/agents/build/turns/vad.md).

To explore all LiveKit documentation, see [llms.txt](https://docs.livekit.io/llms.txt).