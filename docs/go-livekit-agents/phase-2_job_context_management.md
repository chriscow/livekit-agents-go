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

# Phase 2 – Job & Context Management

> Timeline target: **Week 3–4**

Build the Go analogue of Python’s Job object, participant/room abstractions, and lifecycle hooks.

---

## 1. Goals & Deliverables

| ID | Deliverable | Description |
|----|-------------|-------------|
| P2-D1 | `pkg/job` package | Strongly-typed `Job`, `JobContext`, `ShutdownInfo` structs |
| P2-D2 | Room wrapper | Thin layer over `livekit-server/pkg/rtc.Room` with auto-subscribe helpers |
| P2-D3 | Graceful shutdown | Context-cancellable cleanup cascade (room → tracks → plugins) |
| P2-D4 | Participant events | Channel-based event fan-out (`Join`, `Leave`, `TrackSubscribed`) |
| P2-D5 | CLI command `lk-go job run-script` | Execute a Go plugin (phase 5) inside a Job container |

---

## 2. Data Types & Interfaces

### 2.1 Job

```go
// pkg/job/job.go
type Job struct {
    ID        string
    RoomName  string
    Context   *JobContext
}
```

### 2.2 JobContext (immutable after creation)

* `Ctx context.Context` – cancelled when job ends
* `Shutdown(reason string)` – idempotent
* `OnShutdown(cb func(reason string))` – register cleanup hooks

### 2.3 Room Abstraction

```go
type Room struct {
    *rtc.Room
    Events chan Event // fan-out of participant & track events
}
```

---

## 3. Implementation Steps

1. **Create `pkg/job/`** with above structs, plus helper `New(ctx context.Context, cfg Config) (*Job, error)`.
2. **Embed context cancellation**: `JobContext` holds a private `cancel func()` executed in `Shutdown()`.
3. **Room Wrapper**: expose `.Join(url, token)` and internally attach LiveKit event handlers -> push onto `Events` channel.
4. **Auto-subscription**: upon remote participant publish, call `room.AutoSubscribe(participant)`.
5. **CLI command** `job run-script <plugin> --room …` that spawns a one-off job using the same code path used by the worker. This serves as an integration test harness.

---

## 4. Testing Strategy

### 4.1 Unit Tests

* **Job lifecycle** – create job, register two `OnShutdown` hooks, call `Shutdown()`, assert hooks executed once.
* **Event fan-out** – push synthetic `TrackSubscribed` into wrapped room, expect message on `Events` channel.

### 4.2 Integration Test (requires LiveKit server)

* Use `docker compose` to boot LiveKit locally (scripts provided in repo root).
* Join a room, publish a dummy audio track, ensure Go wrapper receives `TrackSubscribed`.
* Time-box entire test to 30 s so CI remains fast.

CLI test example:
```bash
lk-go job run-script noop --room test --url ws://127.0.0.1:7880  # exits 0
```

---

## 5. Acceptance Criteria

* On `go test ./... -race` no data race in JobContext cancellation.
* When LiveKit server goes down mid-job, `Shutdown(reason)` fires within 2 s.
* CLI `job run-script` returns exit code mirroring plugin exit code.

---

## 6. Reference back to Python Implementation

| Concern | Python Source | Go location |
|---------|---------------|-------------|
| JobContext cleanup | `agents/ipc/job_proc_lazy_main.py` lines 284-304 | `pkg/job/context.go` |
| Assignment timeout | `worker.ASSIGNMENT_TIMEOUT` | constant `AssignmentTimeout = 7.5 * time.Second` |

---

_Proceed to [Phase 3 – AI Pipeline Interfaces](phase-3_ai_pipeline_interfaces.md) after green CI._ 