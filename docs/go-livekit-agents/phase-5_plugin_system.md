<!--
Copyright 2024 LiveKit

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->

# Phase 5 – Plugin System

> Timeline target: **Week 9–10**

Allow external packages to register new STT/TTS/LLM/VAD implementations without touching the core.

---

## 1. Goals & Deliverables

| ID | Deliverable | Description |
|----|-------------|-------------|
| P5-D1 | `pkg/plugin` registry | `Register(name string, Factory)` & lookup |
| P5-D2 | Dynamic load | Build-tag-guarded `plugin` package import using Go’s plugin system (linux only) |
| P5-D3 | CLI `lk-go plugin list` | Enumerate available plugins |
| P5-D4 | Core plugins ported | `openai`, `silero` as proof |

---

## 2. Registry API

```go
type Factory func(cfg map[string]any) (any, error) // returns STT/TTS/etc.

func Register(kind, name string, f Factory)
func Get(kind, name string) (Factory, bool)
```

Kind is one of `stt`, `tts`, `llm`, `vad`.

### 2.1 Reference Plugin – Silero VAD

Silero provides state-of-the-art VAD via an **ONNX** model (~1.7 MB).  The Go
reference implementation will live in `plugins/silero` and demonstrate:

* Loading the ONNX graph through `onnxruntime` (CGO).  **Build tag `silero`** will
  guard the dependency so regular builds remain CGO-free.
* Fallback to a lightweight energy-based VAD when the ONNX runtime or model file
  is missing (mirrors Python’s behaviour on constrained environments).
* Self-registration via `init()`, e.g. `plugin.Register("vad", "silero", New)`.

**Distribution note:** the compiled agent **does not embed** the `.onnx` file.
Instead, the plugin looks up `${LK_MODEL_PATH}/silero_vad.onnx` at runtime or
uses the energy fallback.

### 2.2 Dependency & Build-Tag Policy

Official ML plugins (e.g. Silero VAD) MAY rely on heavy runtimes such as
`onnxruntime` provided that:

1. The dependency is isolated behind an **opt-in build tag** (`silero`, `whisper`, …).
2. Go module version is pinned and updated per platform constraints (mac, linux,
   arm64, etc.).
3. The plugin must still build (with degraded functionality) when the tag is
   absent – e.g. energy-based VAD fallback.

### 2.3 Model File Distribution & CLI Support

Plugins choose between two strategies:

| Strategy | When to use | Implementation |
|----------|-------------|----------------|
| Bundled  | &lt; 5 MB, permissive license | Ship model file under `plugins/<name>/models`; embed with `go:embed` for CGO-free builds |
| Download-on-Demand | &gt; 5 MB or restrictive license | Provide `Download()` helper that stores into `${LK_MODEL_PATH}` (default `$HOME/.livekit/models`) |

Add a CLI helper:

```
lk-go plugin download-files   # downloads missing models for all registered plugins
```

### 2.4 Licensing Checklist

* **Source code** → Apache-2.0 (same as core).
* **Model file** → retain upstream license (e.g. CC-BY-NC-SA-4.0 for Silero).
* Plugin `LICENSES/` dir must contain THIRD_PARTY notices & attribution.

---

## 3. Implementation Steps

1. **Create `pkg/plugin`** with maps protected by `sync.RWMutex`.
2. **Silero reference port** demonstrating ONNX load + fallback.
3. **Self-registration** pattern:
   ```go
   func init() { plugin.Register("vad", "silero", New) }
   ```
4. **Dynamic Load** (optional): on Linux with `-tags=plugindyn` attempt
   `plugin.Open()`.
5. **CLI command** `plugin list` reads registry and prints table.
6. **CLI** `plugin download-files` iterates registry, calls `Download()` if implemented.

---

## 4. Testing Strategy

* Unit test that double registration panics (parity with Python).
* Load `fake` plugin dynamically and call Factory.

---

## 5. Acceptance Criteria

* `lk-go plugin list` lists at least `fake` plugin after build.
* Running with `-tags=plugindyn` loads `.so` plugin placed in `${LK_PLUGIN_PATH}`.

---

## 6. Python Reference

| Topic | Python Source |
|-------|---------------|
| Registration | `plugins.base.Plugin.register_plugin()` |
| Discovery | `agents.cli.plugins.load_plugin()` |

---

_Next → [Phase 6 – Production Features](phase-6_production_features.md)._ 