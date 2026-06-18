# Scheduler

A Go microservice for scheduling and executing HTTP-based tasks at configurable times and intervals. Tasks can be one-shot or recurring, and the service handles retries, failure alerts, and graceful lifecycle management.

---

## Features

- Schedule tasks at a specific date and time (IST)
- Recurring tasks with a configurable interval (minimum 1 hour)
- Configurable retry attempts per task with exponential backoff and jitter
- Slack alerts on task failure
- Force-execute any task immediately via API
- Enable / disable tasks without deleting them
- Graceful shutdown — cron stopped, in-flight executors drained before DB closes
- Build-time version stamping via `ldflags`

---

## Project Structure

```
scheduler/
├── cmd/
│   └── scheduler/
│       └── main.go                      # Entrypoint: wires config, logger, DB, server
│
├── config/
│   └── config.go                        # App config struct, embedded defaults, validation
│
├── errors/
│   ├── errors.go                        # Error type, Kind enum, NewError() constructor
│   ├── common.go                        # Shared error constructors (InvalidBody, EmptyParam …)
│   └── validation.go                    # ValidationErrors slice and builder
│
├── http/
│   ├── server.go                        # HTTP server, router, middleware wiring, ToHTTPHandlerFunc
│   ├── handlers/
│   │   └── scheduler_handlers.go        # HTTP handlers for all task routes
│   ├── middleware/
│   │   └── request_logger.go            # Per-request structured zap logging
│   └── response/
│       └── response.go                  # RespondJSON / RespondMessage / RespondError helpers
│
├── models/
│   └── task.go                          # Task, CreateRequest, Status, ActiveList types
│
├── repositories/
│   └── mongodb/
│       ├── connect.go                   # MongoDB client wrapper (connect, ping, close)
│       └── scheduler_repo.go            # Task CRUD: GetOne, GetActive, Insert, Update, Delete
│
├── services/
│   ├── executer/
│   │   └── executer.go                  # Task runner: HTTP call, retry with backoff, status update
│   ├── health/
│   │   └── health.go                    # MongoDB ping health check
│   └── scheduler/
│       ├── scheduler_service.go         # Public API: Insert, Enable, Disable, Delete, ExecuteNow
│       └── scheduler_utils.go           # Scheduling engine: cron, timers, dispatch logic
│
├── utils/
│   ├── helpers/
│   │   ├── strings.go                   # MD5, PrintStruct, UnmarshalInterface
│   │   ├── time.go                      # Unix type, IST/UTC parsing, time helpers
│   │   └── validate.go                  # Field validation helpers (required, date, time …)
│   ├── httpclient/
│   │   └── client.go                    # Shared HTTP client with connection pooling
│   ├── notifications/
│   │   ├── sender.go                    # Sender interface
│   │   └── slack.go                     # Slack Incoming Webhook implementation
│   └── version/
│       └── info.go                      # Build-time Version and BuildTime variables
│
├── deployments/
│   ├── Dockerfile                       # Multi-stage build (golang:1.26.1-alpine → alpine)
│   └── docker-compose.yml               # MongoDB for local development
│
├── Makefile
├── go.mod
└── go.sum
```

---

## API

Base path: `/scheduler/v1`

### Task

| Method   | Path                          | Description                      |
|----------|-------------------------------|----------------------------------|
| `POST`   | `/task`                       | Create and schedule a new task   |
| `GET`    | `/task/{task_id}`             | Get task details                 |
| `PATCH`  | `/task/{task_id}/enable`      | Enable a disabled task           |
| `PATCH`  | `/task/{task_id}/disable`     | Disable a running task           |
| `DELETE` | `/task/{task_id}`             | Delete a task                    |

### Helpers

| Method | Path                               | Description                        |
|--------|------------------------------------|------------------------------------|
| `GET`  | `/helpers/active-tasks`            | List all currently active task IDs |
| `POST` | `/helpers/execute-task/{task_id}`  | Force-execute a task immediately   |

### System

| Method | Path      | Description                             |
|--------|-----------|-----------------------------------------|
| `GET`  | `/health` | MongoDB ping — returns 200 or 503       |
| `GET`  | `/build`  | Git commit and build timestamp          |

---

## Task Payload

`POST /scheduler/v1/task`

```json
{
  "scheduleDate": "2026-06-15",
  "scheduleTime": "14:30",
  "recur": 0,
  "isRecurEnabled": false,
  "numberOfAttempts": 3,
  "expiresAt": "2026-12-31T18:30:00.000Z",
  "taskData": {
    "taskType": "api-call",
    "requestType": "POST",
    "url": "https://example.com/webhook",
    "headers": { "Authorization": "Bearer token" },
    "queryParams": {},
    "requestBody": { "key": "value" }
  }
}
```

**Fields:**

| Field                  | Type   | Required | Description                                                       |
|------------------------|--------|----------|-------------------------------------------------------------------|
| `scheduleDate`         | string | yes      | Date in `YYYY-MM-DD` (IST)                                        |
| `scheduleTime`         | string | yes      | Time in `HH:MM` 24-hour (IST)                                     |
| `recur`                | int    | yes      | Repeat interval in seconds. Must be `0` for non-recurring tasks   |
| `isRecurEnabled`       | bool   | yes      | `true` for recurring tasks — `recur` must be ≥ `3600`             |
| `numberOfAttempts`     | int    | no       | Retry count on failure (default: `3`)                             |
| `expiresAt`            | string | no       | UTC expiry timestamp `YYYY-MM-DDTHH:MM:SS.sssZ` (default: 10 yr) |
| `taskData.taskType`    | string | yes      | Arbitrary label for the task category                             |
| `taskData.requestType` | string | yes      | One of: `GET POST PATCH PUT DELETE HEAD OPTIONS`                  |
| `taskData.url`         | string | yes      | Target URL the executor calls                                     |
| `taskData.headers`     | object | no       | HTTP headers forwarded with each attempt                          |
| `taskData.queryParams` | object | no       | Query parameters appended to the URL                              |
| `taskData.requestBody` | object | no       | JSON body sent with the request                                   |

> **Scheduling note:** `scheduleDate` + `scheduleTime` are interpreted as IST and
> converted to UTC Unix timestamps at insert time. `expiresAt` is UTC. The
> scheduler will not execute a task whose start time is in the past or whose
> expiry has already elapsed.

---

## Configuration

All defaults are embedded in the binary. Provide a YAML file to override any field.

```yaml
application: "scheduler"

logger:
  encoding: "logfmt"   # logfmt | json
  level: "debug"       # debug | info | warn | error

listen: ":4202"
prefix: "/scheduler"
is_prod_mode: false

mongo:
  uri: "mongodb://localhost:27017"

slack:
  webhook_url: "https://hooks.slack.com/services/your/webhook/url"
  send_alerts_in_dev: false   # set true to send Slack alerts in non-prod mode
```

Pass a config file with the `-c` flag:

```bash
./scheduler -c /etc/scheduler/config.yml
```

---

## Running

### Prerequisites

- Go 1.26+
- MongoDB (local or remote)

Start MongoDB locally using Docker Compose:

```bash
docker compose -f deployments/docker-compose.yml up -d
```

### Make targets

```bash
make run                          # Run with embedded defaults (no config file)
make run/config CONFIG=config.yml # Run with a custom config file

make build                        # Compile → .bin/scheduler (version-stamped)
make all                          # fmt + vet + build

make fmt                          # go fmt ./...
make vet                          # go vet ./...
make tidy                         # go mod tidy && go mod verify
make lint                         # golangci-lint run ./...

make test                         # go test -race ./...
make test/cover                   # Race + coverage → coverage.html

make docker/build                 # Build image as scheduler:<commit>
make docker/run                   # Run image, mount config, forward :4202
make docker/all                   # Build + run in one step

make help                         # List all targets with descriptions
```

### Direct

```bash
# Run with defaults
go run ./cmd/scheduler

# Run with config file
go run ./cmd/scheduler -c config.yml

# Build and run
make build && .bin/scheduler -c config.yml
```

---

## Tech Stack

| Concern         | Library                     |
|-----------------|-----------------------------|
| HTTP router     | `go-chi/chi/v5`             |
| Cron scheduling | `robfig/cron/v3`            |
| Database        | MongoDB (`mongo-driver/v2`) |
| Logging         | `uber-go/zap` + logfmt      |
| Config          | `knadh/koanf`               |
| CLI flags       | `alecthomas/kingpin/v2`     |
| Alerts          | Slack Incoming Webhooks     |
