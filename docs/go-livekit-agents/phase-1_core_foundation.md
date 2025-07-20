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

# Phase 1 – Core Foundation

> Timeline target: **Week 1–2** (assuming 1 FTE)

This phase establishes the minimal runnable skeleton: CLI, logging, worker loop, and LiveKit WebSocket connectivity. Every later phase builds on these artefacts.

---

## 1. Goals & Deliverables

| ID | Deliverable | Description |
|----|-------------|-------------|
| P1-D1 | `lk-go` CLI binary | Root command plus `version`, `worker run`, `worker healthz` sub-commands |
| P1-D2 | `internal/worker` package | Goroutine-based worker engine handling WebSocket session & heart-beat |
| P1-D3 | Structured logging | JSON logs via `slog.New(jsonHandler)` with log level from `LK_LOG_LEVEL` env var |
| P1-D4 | Config system | Read env + flags into strongly-typed struct using functional options pattern |
| P1-D5 | CI pipeline | GitHub Action running `go vet`, `golangci-lint`, `go test ./...` |

All deliverables must compile on Linux/macOS/Windows with **Go 1.24**.

---

## 2. Directory Skeleton

```text
livekit-agents-go/              # project root
├── cmd/
│   └── lk-go/
│       └── main.go            # cobra root & sub-cmd registration
├── internal/
│   └── worker/
│       ├── worker.go          # lifecycle, reconnection, graceful shutdown
│       └── websocket.go       # thin ws wrapper for LiveKit signalling
├── pkg/
│   └── version/
│       └── version.go         # populated at build-time via -ldflags
└── docs/go-livekit-agents/    # (this doc set)
```

Use `go work use ./` if you maintain multiple modules locally.

---

## 3. Step-by-Step Implementation Checklist

1. **Bootstrap `go.mod`**  
   ```bash
   go mod init github.com/chriscow/livekit-agents-go && go get github.com/spf13/cobra@v1
   ```

2. **Create CLI Skeleton**  
   * Root command: `lk-go`
   * Sub-command **version** prints `version.Version` string.
   * Sub-command **worker run** parses `--url`, `--token`, `--dry-run` flags.
   * Sub-command **worker healthz** performs a self-diag and exits 0/1.

3. **Logging Middleware**  
   * `slog.Handler` chosen at startup based on `LK_LOG_FORMAT` (json|console).
   * Propagate `context.Context` with a `request_id` value for trace fan-out.

4. **Worker Engine**  
   * `New()` returns `*Worker` with channels:
     * `in <-chan livekit.Signal`
     * `out chan<- livekit.Command`
   * Goroutine reads `in`, handles `Ping`, `StartJob`, `Shutdown`.
   * Use exponential back-off (1 s ‑> 10 s) on socket failure.

5. **Graceful Shutdown Hooks**  
   * `Worker.Run(ctx)` listens for `context.Done()` and drains internal queues.
   * Exit code 0 on normal stop, 1 on unhandled error.

6. **Makefile & CI**  
   * `make test` → `go test ./...`
   * `make lint` → `golangci-lint run`
   * GitHub Action executes both on push & PR.

---

## 4. CLI Commands (Phase 1 scope)

| Command | Purpose | Typical Use |
|---------|---------|-------------|
| `lk-go version` | Print semantic version + git commit | CI smoke test |
| `lk-go worker run --url wss://… --token …` | Start a worker against LiveKit | Local dev |
| `lk-go worker healthz` | Quick connectivity check (pings server once) | K8s liveness probe |

_All commands must exit non-zero on error and produce JSON log lines._

---

## 5. Testing Strategy

### 5.1 Unit Tests

* Use **matryer/is only** + the standard library.
* `worker_test.go` → simulate incoming `Ping` messages, expect outgoing `Pong`.
* Table-driven tests for back-off calculator.

### 5.2 Integration Test (optional offline)

* Spin up a gorilla websocket echo server in‐process (`httptest.NewServer`).
* Verify that `Worker` reconnects after forced close.

### 5.3 CLI Smoke Test

```bash
lk-go version | grep -q "Version:"  # exit status 0
```

---

## 6. Acceptance Criteria

* `go vet ./...` reports **zero** issues.
* `go test ./... -race` passes.
* `golangci-lint run` passes with the default fast preset.
* Running `lk-go worker run --dry-run` exits **0** within 2 s.
* JSON log line includes `"service":"lk-go"` and build commit hash.

---

## 7. GitHub Actions Stub

```yaml
name: CI
on: [push, pull_request]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.24"
      - run: go vet ./...
      - run: go test ./... -race -coverprofile=coverage.out
      - uses: golangci/golangci-lint-action@v3
```

---

## 8. Reference back to Python Implementation

| Concern | Python Location | Go Counterpart |
|---------|-----------------|----------------|
| Worker event loop | `livekit/agents/worker.py` | `internal/worker/worker.go` |
| WebSocket reconnect | `worker._connection_task` | `internal/worker/websocket.go` |
| CLI | `lk` click group | `cmd/lk-go` cobra root |

Follow the same reconnection jitter constants (max 10 s) for parity.

---

> **Next step:** Once all acceptance criteria pass in CI, move to [Phase 2 – Job & Context Management](phase-2_job_context_management.md). 