# AgentBakerSvc load study

This command sends `NodeBootstrappingConfiguration` requests to AgentBakerSvc at a fixed open-loop rate. It records each scheduled arrival independently of earlier response completion, so slow responses do not reduce the offered load.

The built-in fixture is synthetic and derived from AgentBaker's Linux API test. Use `--body` only with an approved, sanitized request. The built-in fixture is a safe functional baseline; it is not yet the maximum-work fixture for the capacity study.

## Local smoke test

Start AgentBakerSvc:

```bash
go run . start --addr 127.0.0.1:8080
```

Start at 1 RPS for 30 seconds with the production client timeout. A smoke test should prove that the endpoint and fixture work before applying enough load to build a queue:

```bash
go run ./hack/agentbakersvc-load \
  --target http://127.0.0.1:8080/getnodebootstrapdata \
  --rps 1 \
  --duration 30s \
  --timeout 3s \
  --output results-smoke.jsonl
```

Verify that `successful` equals `offered`, with zero timeouts, overload responses, transport errors, and client drops. Then find the local capacity knee before running the production traffic points:

```bash
for rate in 1 2 3 4 5 10; do
  go run ./hack/agentbakersvc-load \
    --target http://127.0.0.1:8080/getnodebootstrapdata \
    --rps "$rate" \
    --duration 30s \
    --timeout 3s \
    --output "results-${rate}rps.jsonl" \
    | tee "summary-${rate}rps.json"

  read -rp "Allow AgentBakerSvc to cool down, then press Enter"
done
```

Run the production traffic points as separate processes so a saturated phase cannot contaminate the next phase:

```bash
for rate in 10 30 50 72 100; do
  go run ./hack/agentbakersvc-load \
    --target http://127.0.0.1:8080/getnodebootstrapdata \
    --rps "$rate" \
    --duration 5m \
    --timeout 3s \
    --output "results-${rate}rps.jsonl"
done
```

Replay the fallback incident shape or synchronized limiter waves:

```bash
go run ./hack/agentbakersvc-load \
  --schedule hack/agentbakersvc-load/schedules/incident-fallback.json \
  --output results-incident.jsonl

go run ./hack/agentbakersvc-load \
  --schedule hack/agentbakersvc-load/schedules/saturation-waves.json \
  --output results-saturation.jsonl
```

A schedule contains ordered phases. Each phase must set exactly one of `rps` for open-loop traffic or `concurrency` for a barrier-synchronized wave. For a wave, `duration` is the cooldown before the next phase. Use `repeat` to repeat a phase.

The summary is written to stdout. `completed` and `completedRps` count all terminal client attempts, including timeouts; use `successful`, `successfulRps`, and `successLatencyMs` for delivered service capacity. `httpResponses` counts attempts that received an HTTP response, while `overloaded` counts HTTP 503 responses. Per-request JSONL includes scheduled, start, and completion timestamps; scheduling delay; service and end-to-end latency; status; `Retry-After`; response bytes; error classification; and observed outstanding requests. If the client reaches `--max-outstanding`, it records dropped arrivals rather than converting the open-loop test into a closed-loop test.

## Profiling

The pprof listener is disabled by default and is separate from the API router. Enable it only in an isolated test deployment:

```bash
go run . start --addr :8080 --pprof-addr 127.0.0.1:6060
```

For a pod, port-forward the unexposed debug port:

```bash
kubectl -n agentbakersvc-perf port-forward pod/<pod> 6060:6060
```

Collect profiles sequentially during a stable load phase:

```bash
go tool pprof -proto cpu.pb.gz 'http://127.0.0.1:6060/debug/pprof/profile?seconds=60'
curl -fsS 'http://127.0.0.1:6060/debug/pprof/heap?gc=1' -o heap.pb.gz
curl -fsS 'http://127.0.0.1:6060/debug/pprof/allocs' -o allocs.pb.gz
curl -fsS 'http://127.0.0.1:6060/debug/pprof/goroutine' -o goroutine.pb.gz
curl -fsS 'http://127.0.0.1:6060/debug/pprof/block?seconds=30' -o block.pb.gz
curl -fsS 'http://127.0.0.1:6060/debug/pprof/mutex?seconds=30' -o mutex.pb.gz
```

Enabling pprof also enables block and mutex sampling. Repeat representative runs without pprof to measure observer overhead. Never expose port 6060 through the production Service or an external load balancer.

## Remaining study input

Before executing the incident study, replace the fallback schedule with the measured one-second incident arrival pattern. Validate an approved maximum-work fixture through `GetNodeBootstrapping` and record its request/response sizes and SHA-256.