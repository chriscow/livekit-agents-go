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

# Phase 7 – Example Applications

> Timeline target: **Week 13–14**

Ship runnable demos that mirror Python examples plus a new Go-native one.

---

## 1. Goals & Deliverables

| ID | Deliverable | Description |
|----|-------------|-------------|
| P7-D1 | `examples/echo_bot` | Voice agent that echoes user speech |
| P7-D2 | `examples/drive_thru` | Port of Python drive-thru demo |
| P7-D3 | `examples/cli_tool_call` | Demonstrate function-calling with tools |
| P7-D4 | README docs | Step-by-step run instructions |

---

## 2. Implementation Steps

1. **Echo Bot** – instantiate Agent with `fake` STT/TTS and run locally.
2. **Drive-Thru** – reuse menu logic from Python, translate to Go.
3. **Tool-Call Demo** – expose `weather.today` function, call OpenAI.
4. **Add `make examples`** – builds and runs Echo Bot in headless mode.

---

## 3. Testing & Acceptance

* CI job runs Echo Bot for 5 s with prerecorded audio, asserts echoed packet count.
* Drive-Thru passes order string regression test (`2x burger`).
* All READMEs render without broken links (link checker action).

---

## 4. Python Reference Map

| Example | Python location | Go folder |
|---------|-----------------|-----------|
| Echo agent | `examples/primitives/echo-agent.py` | `examples/echo_bot` |
| Drive-thru | `examples/drive-thru` | `examples/drive_thru` |

---

🎉 **Project complete!** Merge to main and tag `v0.1.0`. 