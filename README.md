# fedey-backend

Strategy-first AI social growth engine backend in Go.

## Product Loop

`observe -> hypothesize -> experiment -> measure -> learn -> update strategy`

## Service Boundaries

- `cmd/api`: HTTP APIs for dashboard and integrations
- `cmd/worker`: async jobs for generation, experiments, analytics ingestion
- `cmd/scheduler`: time-based planning and posting windows
- `internal/agents`: supervisor + specialist agents
- `internal/strategy`: hypothesis and weekly planning engine
- `internal/experiments`: variant generation, assignment, winner logic
- `internal/publishing`: publish orchestration and platform safety checks
- `internal/analytics`: engagement ingestion and normalization
- `internal/learning`: recommendation + policy updates from outcomes
- `events`: event contracts for decoupled workflow

## Initial Milestones

1. Brand + account memory
2. Trend and signal ingestion
3. Hypothesis/experiment planning
4. Multi-variant content pipeline
5. Automated publishing with guardrails
6. Learning and recommendations

## Local Run

```bash
go run ./cmd/api
```

Health endpoints:
- `GET /healthz`
- `GET /v1/health`
