# Scheduler

A Go microservice for scheduling and executing HTTP-based tasks at configurable times and intervals. Tasks can be one-shot or recurring, and the service handles retries, failure alerts, and graceful lifecycle management.

---

## Features

- Schedule tasks at a specific date and time (IST)
- Recurring tasks with a configurable interval (minimum 1 hour)
- Configurable retry attempts per task with exponential backoff
- Slack alerts on task failure
- Force-execute any task immediately via API
- Enable / disable tasks without deleting them
- Graceful HTTP shutdown

---

## Project Structure

```
scheduler/
├── cmd/
│   └── scheduler/
│       └── main.go              # Entrypoint: wires config, logger, server
│
├── internal/                    # Private application code
│   ├── config/
│   │   └── config.go            # App config struct, defaults, validation
│   ├── domain/
│   │   └── task.go              # Core types: TaskModel, TaskQP, Status
│   ├── errors/
│   │   ├── errors.go            # Error type, Kind enum, E() constructor
│   │   ├── common.go            # Shared error constructors
│   │   └── validation.go        # ValidationErrors and builder
│   ├── validate/
│   │   └── validate.go          # Field validation helpers
│   ├── repository/
│   │   └── mongodb/
│   │       ├── connect.go       # MongoDB client wrapper
│   │       └── scheduler_repo.go # Task CRUD operations
│   ├── service/
│   │   ├── scheduler/
│   │   │   ├── scheduler_service.go  # Business logic: Insert, Enable, Disable, etc.
│   │   │   └── scheduler_utils.go    # Scheduling engine: cron, timers, dispatch
│   │   ├── executer/
│   │   │   └── executer.go      # Task runner: HTTP call, retry, status update
│   │   └── health/
│   │       └── health.go        # MongoDB ping health check
│   └── transport/
│       ├── server.go            # HTTP server, router, middleware wiring
│       ├── handler/
│       │   └── scheduler.go     # HTTP handlers for all task routes
│       ├── middleware/
│       │   └── logger.go        # Request logging middleware
│       └── response/
│           └── response.go      # JSON response helpers
│
└── pkg/                         # Reusable packages with no internal dependencies
    ├── httputil/
    │   └── client.go            # HTTP client, CallAPI, HttpRequestType
    ├── notifier/
    │   └── slack.go             # Slack webhook alert sender
    ├── strutil/
    │   └── strings.go           # String and general utility functions
    └── timeutil/
        └── time.go              # Time parsing, formatting, Unix type
```

---

## API

Base path: `/scheduler/v1`

### Task

| Method   | Path                        | Description                       |
|----------|-----------------------------|-----------------------------------|
| `POST`   | `/task`                     | Create and schedule a new task    |
| `GET`    | `/task/{taskId}`            | Get task details                  |
| `PATCH`  | `/task/{taskId}/enable`     | Enable a disabled task            |
| `PATCH`  | `/task/{taskId}/disable`    | Disable a running task            |
| `DELETE` | `/task/{taskId}`            | Delete a task                     |

### Helpers

| Method   | Path                              | Description                          |
|----------|-----------------------------------|--------------------------------------|
| `GET`    | `/helpers/active-tasks`           | List all currently active task IDs   |
| `POST`   | `/helpers/execute-task/{taskId}`  | Force-execute a task immediately     |

### Health

| Method | Path         | Description          |
|--------|--------------|----------------------|
| `GET`  | `/v1/health` | Service health check |

---

## Task Payload

```json
{
  "schedule": "NOW",
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

| Field                  | Type   | Required | Description                                               |
|------------------------|--------|----------|-----------------------------------------------------------|
| `scheduleDate`         | string | yes      | Date in `YYYY-MM-DD` (IST)                                |
| `scheduleTime`         | string | yes      | Time in `HH:MM` (IST)                                    |
| `recur`                | int    | yes      | Interval in seconds. Must be `0` for non-recurring tasks  |
| `isRecurEnabled`       | bool   | yes      | Set `true` for recurring tasks (`recur` must be ≥ 3600)  |
| `numberOfAttempts`     | int    | no       | Retry count on failure (default: 3)                       |
| `expiresAt`            | string | no       | UTC expiry in `YYYY-MM-DDTHH:MM:SS.sssZ` (default: 10 years) |
| `taskData.requestType` | string | yes      | One of: `GET POST PATCH PUT DELETE HEAD OPTIONS`          |
| `taskData.url`         | string | yes      | Target URL the task calls                                 |

---

## Configuration

The service reads config from a YAML file (default: `config.yml`). All defaults are embedded in the binary and overridden by the file.

```yaml
application: "scheduler"

logger:
  encoding: "logfmt"   # logfmt or json
  level: "debug"       # debug, info, warn, error

listen: ":4202"
prefix: "/scheduler"
is_prod_mode: false

mongo:
  uri: "mongodb://localhost:27017"

slack:
  webhook_url: "https://hooks.slack.com/services/your/webhook/url"
  send_alerts_in_dev: false
```

Pass a custom config file with the `-c` flag:

```bash
./scheduler -c /etc/scheduler/config.yml
```

---

## Running

```bash
# Run directly
go run ./cmd/scheduler

# Build and run
go build -o bin/scheduler ./cmd/scheduler && ./bin/scheduler

# With custom config
./bin/scheduler -c config.prod.yml

# Using Make
make run
```

---

## Tech Stack

| Concern         | Library                     |
|-----------------|-----------------------------|
| HTTP router     | `go-chi/chi`                |
| Cron scheduling | `robfig/cron/v3`            |
| Database        | MongoDB (`mongo-driver/v2`) |
| Logging         | `uber-go/zap`               |
| Config          | `knadh/koanf`               |
| Alerts          | Slack Incoming Webhooks     |
