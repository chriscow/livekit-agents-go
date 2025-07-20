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

# Phase 6 – Production Features

> Timeline target: **Week 11–12**

Harden the runtime with observability, limits, and graceful degradation.

---

## 1. Goals & Deliverables

| ID | Deliverable | Description |
|----|-------------|-------------|
| P6-D1 | Metrics exporter | Prometheus `/metrics` using `promhttp` |
| P6-D2 | Tracing | OpenTelemetry HTTP + gRPC spans |
| P6-D3 | Rate limiting | `golang.org/x/time/rate` per provider |
| P6-D4 | Circuit breaker | `sony/gobreaker` around STT/TTS RPCs |
| P6-D5 | Health endpoints | `/healthz` and `/readyz` |

---

## 2. Implementation Steps

1. **Integrate Prometheus** – register default Go metrics + custom counters for `stt_requests_total` etc.
2. **Add OpenTelemetry** – sample 1 % traces by default, configurable via env.
3. **Wrap provider calls** with `rate.Limiter` and `gobreaker.CircuitBreaker`.
4. **Expose HTTP server** inside worker on `:8080` (configurable).
5. **Add liveness/readiness**: readiness waits until WebSocket connected & plugins loaded.

---

## 3. Testing Strategy

* Unit tests feed 1000 concurrent STT calls, verify breaker trips after 5 failures.
* Integration test scrapes `/metrics` and asserts presence of `go_gc_duration_seconds`.

---

## 4. Acceptance Criteria

* CPU + memory usage reported in Prom metrics.
* When breaker open, `ErrServiceUnavailable` returned within 50 ms.
* `docker-compose up` + `curl :8080/healthz` returns 200.

---

## 5. Python Reference

| Feature | Python Source |
|---------|---------------|
| Metrics | `agents.metrics.collector` |
| Retry/backoff | `stt._main_task` retry loop |

---

_When stable proceed to [Phase 7 – Example Applications](phase-7_example_applications.md)._ 