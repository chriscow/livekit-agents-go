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

---

## 3. Implementation Steps

1. **Create `pkg/plugin`** with maps protected by `sync.RWMutex`.
2. **Self-registration** pattern:
   ```go
   func init() { plugin.Register("stt", "openai", New) }
   ```
3. **Dynamic Load** (optional): on Linux with `-tags=plugindyn` attempt `plugin.Open()`.
4. **CLI command** `plugin list` reads registry and prints table.

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