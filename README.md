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

Automation worker:

```bash
go run ./cmd/scheduler
```

Health endpoints:
- `GET /healthz`
- `GET /v1/health`

Strategy endpoint:
- `GET /v1/strategy/snapshot`

Experiments endpoints:
- `POST /v1/experiments`
- `GET /v1/experiments`
- `PATCH /v1/experiments/{id}/status`

Analytics endpoint:
- `POST /v1/analytics/events`

Brand memory endpoints:
- `GET /v1/brand-memory`
- `PUT /v1/brand-memory`

Trend endpoints:
- `GET /v1/trends`
- `POST /v1/trends`

Content endpoints:
- `GET /v1/content/drafts`
- `POST /v1/content/drafts/generate`
- `POST /v1/content/drafts/{id}/variants/generate`

Publishing endpoints:
- `GET /v1/publishing/schedules`
- `POST /v1/publishing/schedules`
- `PATCH /v1/publishing/schedules/{id}/publish`

Community endpoints:
- `GET /v1/community/inbox`
- `POST /v1/community/inbox`
- `POST /v1/community/inbox/{id}/draft-reply`
- `PATCH /v1/community/inbox/{id}/reply`

Automation endpoints:
- `GET /v1/automation/runs`
- `POST /v1/automation/run`

Copy `.env.example` to `.env` and adjust if needed.
If `FEDEY_DATABASE_URL` is unset, experiments use in-memory storage.
Set `FEDEY_AUTOMATION_INTERVAL` to control scheduler cadence. Default: `1h`.
Set `FEDEY_X_ACCESS_TOKEN` and `FEDEY_X_USER_ID` to enable real X publishing and mention ingestion.
Set `FEDEY_X_CLIENT_ID`, `FEDEY_X_REDIRECT_URI`, and `FEDEY_WEB_APP_URL` to enable real X OAuth account connection.
