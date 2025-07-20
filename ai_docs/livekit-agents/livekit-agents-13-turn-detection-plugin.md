LiveKit Docs › Building voice agents › Turn detection & interruptions › Turn detector plugin

---

# LiveKit turn detector plugin

> Open-weights model for contextually-aware voice AI turn detection.

## Overview

The LiveKit turn detector plugin is a custom, open-weights language model that adds conversational context as an additional signal to voice activity detection (VAD) to improve end of turn detection in voice AI apps.

Traditional VAD models are effective at determining the presence or absence of speech, but without language understanding they can provide a poor user experience. For instance, a user might say "I need to think about that for a moment" and then take a long pause. The user has more to say but a VAD-only system interrupts them anyways. A context-aware model can predict that they have more to say and wait for them to finish before responding.

The LiveKit turn detector plugin is free to use with the Agents SDK and includes both English-only and multilingual models.

- **[Turn detector demo](https://youtu.be/EYDrSSEP0h0)**: A video showcasing the improvements provided by the LiveKit turn detector.

## Quick reference

The following sections provide a quick overview of the turn detector plugin. For more information, see [Additional resources](#additional-resources).

### Requirements

The LiveKit turn detector is designed for use inside an `AgentSession` and also requires an [STT plugin](https://docs.livekit.io/agents/integrations/stt.md) be provided. If you're using a realtime LLM you must include a separate STT plugin to use the LiveKit turn detector plugin.

LiveKit recommends also using the [Silero VAD plugin](https://docs.livekit.io/agents/build/turns/vad.md) for maximum performance, but you can rely on your STT plugin's endpointing instead if you prefer.

The model runs locally on the CPU and requires <500 MB of RAM even with multiple concurrent jobs with a shared inference server.

### Installation

Install the plugin from PyPI:

```bash
pip install "livekit-agents[turn-detector]~=1.0"

```

### Download model weights

You must download the model weights before running your agent for the first time:

```bash
python agent.py download-files

```

### Usage

Initialize your `AgentSession` with the turn detector and initialize your STT plugin with matching language settings. These examples use the Deepgram STT plugin, but more than 10 other STT plugins [are available](https://docs.livekit.io/agents/integrations/stt.md).

#### English-only model

Use the `EnglishModel` and ensure your STT plugin configuration matches:

```python
from livekit.plugins.turn_detector.english import EnglishModel
from livekit.plugins import deepgram

session = AgentSession(
    turn_detection=EnglishModel(),
    stt=deepgram.STT(model="nova-3", language="en"),
    # ... vad, stt, tts, llm, etc.
)

```

#### Multilingual model

Use the `MultilingualModel` and ensure your STT plugin configuration matches. In this example, Deepgram performs automatic language detection and passes that value to the turn detector.

```python
from livekit.plugins.turn_detector.multilingual import MultilingualModel
from livekit.plugins import deepgram

session = AgentSession(
    turn_detection=MultilingualModel(),
    stt=deepgram.STT(model="nova-3", language="multi"),
    # ... vad, stt, tts, llm, etc.
)

```

### Parameters

The turn detector itself has no configuration, but the `AgentSession` that uses it supports the following related parameters:

- **`min_endpointing_delay`** _(float)_ (optional) - Default: `0.5`: The number of seconds to wait before considering the turn complete. The session uses this delay when no turn detector model is present, or when the model indicates a likely turn boundary.

- **`max_endpointing_delay`** _(float)_ (optional) - Default: `6.0`: The maximum time to wait for the user to speak after the turn detector model indicates the user is likely to continue speaking. This parameter has no effect without the turn detector model.

## Supported languages

The `MultilingualModel` supports English and 13 other languages. The model relies on your [STT plugin](https://docs.livekit.io/agents/integrations/stt.md) to report the language of the user's speech. To set the language to a fixed value, configure the STT plugin with a specific language. For example, to force the model to use Spanish:

```python
session = AgentSession(
    turn_detection=MultilingualModel(),
    stt=deepgram.STT(model="nova-2", language="es"),
    # ... vad, stt, tts, llm, etc.
)

```

The model currently supports English, Spanish, French, German, Italian, Portuguese, Dutch, Chinese, Japanese, Korean, Indonesian, Turkish, Russian, and Hindi.

## Realtime model usage

Realtime models like the OpenAI Realtime API produce user transcripts after the end of the turn, rather than incrementally while the user speaks. The turn detector model requires live STT results to operate, so you must provide an STT plugin to the `AgentSession` to use it with a realtime model. This incurs extra cost for the STT model.

## Benchmarks

The following data shows the expected performance of the turn detector model.

### Runtime performance

The size on disk and typical CPU inference time for the turn detector models is as follows:

| Model | Base Model | Size on Disk | Per Turn Latency |
| English-only | [SmolLM2-135M](https://huggingface.co/HuggingFaceTB/SmolLM2-135M-Instruct) | 66 MB | ~15-45 ms |
| Multilingual | [Qwen2.5-0.5B](https://huggingface.co/Qwen/Qwen2.5-0.5B) | 281 MB | ~50-160 ms |

### Detection accuracy

The following tables show accuracy metrics for the turn detector models in each supported language.

- **True positive** means the model correctly identifies the user has finished speaking.
- **True negative** means the model correctly identifies the user will continue speaking.

#### English-only model

Accuracy metrics for the English-only model:

| Language | True Positive Rate | True Negative Rate |
| English | 98.8% | 87.5% |

#### Multilingual model

Accuracy metrics for the multilingual model, when configured with the correct language:

| Language | True Positive Rate | True Negative Rate |
| Hindi | 99.4% | 93.6% |
| Korean | 99.3% | 88.7% |
| French | 99.3% | 84.9% |
| Portuguese | 99.4% | 82.8% |
| Indonesian | 99.3% | 80.3% |
| Russian | 99.3% | 80.2% |
| English | 99.3% | 80.2% |
| Chinese | 99.3% | 80.0% |
| Japanese | 99.3% | 79.8% |
| Italian | 99.3% | 79.8% |
| Spanish | 99.3% | 79.7% |
| German | 99.3% | 77.6% |
| Turkish | 99.3% | 74.3% |
| Dutch | 99.3% | 73.4% |

## Additional resources

The following resources provide more information about using the LiveKit turn detector plugin.

- **[Python package](https://pypi.org/project/livekit-plugins-turn-detector/)**: The `livekit-plugins-turn-detector` package on PyPI.

- **[Plugin reference](https://docs.livekit.io/reference/python/v1/livekit/plugins/turn_detector/index.html.md#livekit.plugins.turn_detector.TurnDetector)**: Reference for the LiveKit turn detector plugin.

- **[GitHub repo](https://github.com/livekit/agents/tree/main/livekit-plugins/livekit-plugins-turn-detector)**: View the source or contribute to the LiveKit turn detector plugin.

- **[LiveKit Model License](https://huggingface.co/livekit/turn-detector/blob/main/LICENSE)**: LiveKit Model License used for the turn detector model.

---

This document was rendered at 2025-07-18T19:13:29.144Z.
For the latest version of this document, see [https://docs.livekit.io/agents/build/turns/turn-detector.md](https://docs.livekit.io/agents/build/turns/turn-detector.md).

To explore all LiveKit documentation, see [llms.txt](https://docs.livekit.io/llms.txt).