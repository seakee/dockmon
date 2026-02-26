# Dockmon

[中文文档](./README_ZH.md)

Dockmon is a Go service that collects Docker container logs and stores them in MySQL.
It supports both structured and unstructured logs, watches Docker events, and starts/stops collectors dynamically as containers change state.

## Features

- Collect logs from configured containers.
- Parse JSON logs and merge multiline unstructured logs.
- Persist logs to MySQL with container metadata and extra fields.
- Track Docker events (`start`, `stop`, `die`, `destroy`) to manage collectors dynamically.
- Resume collection with Redis-based timestamp checkpoints.
- Provide internal auth APIs for app registration and JWT token issuance.

## Architecture

At startup, Dockmon initializes dependencies, then runs three concurrent components:

- HTTP server (`Gin`)
- In-process scheduler
- Docker log collector

Key flow for log collection:

1. Resolve target container names to container IDs.
2. Start log stream with Docker SDK (`Follow + Timestamps`).
3. Parse each line:
   - JSON logs: map known fields (`L`, `T`, `C`, `M`, `TraceID`) and keep unknown keys in `extra`.
   - Unstructured logs: buffer multiline blocks and infer level.
4. Sanitize message content (strip ANSI escape sequences, control chars, invalid UTF-8, and truncate safely).
5. Store into MySQL table `log`.
6. Save last timestamp in Redis for incremental resume.

## Project Layout

```text
.
├── app/
│   ├── http/                # API handlers, middleware, router
│   ├── monitor/             # Docker watcher and log parser
│   ├── job/                 # Scheduler jobs
│   ├── model/               # GORM models
│   ├── repository/          # Data access layer
│   ├── service/             # Business services
│   └── pkg/                 # Shared packages (jwt, trace, schedule, error code)
├── bootstrap/               # App bootstrap and component startup
├── bin/
│   ├── configs/             # local/dev/prod config files
│   ├── data/sql/            # schema and migration SQL
│   └── lang/                # i18n message files
├── scripts/                 # helper scripts
├── Dockerfile
├── Makefile
└── main.go
```

## Requirements

- Go `1.24+` (from `go.mod`)
- Docker daemon (Dockmon uses Docker socket/API)
- MySQL 8+
- Redis

## Quick Start (Local)

1. Clone the repository.

```bash
git clone https://github.com/seakee/dockmon.git
cd dockmon
```

2. Create local config and update required fields.

```bash
cp bin/configs/local.json.default bin/configs/local.json
```

You must set:

- `system.jwt_secret` (required, use a strong random value, at least 32 chars)
- `databases[0]` connection values
- `redis[0]` connection values
- `collector.container_name` (containers to monitor)

3. Initialize database schema.

```bash
mysql -u <user> -p <database> < bin/data/sql/auth_app.sql
mysql -u <user> -p <database> < bin/data/sql/log.sql
```

If you are upgrading an existing deployment, also apply:

```bash
mysql -u <user> -p <database> < bin/data/sql/migration/20260226_log_utf8mb4_fix.sql
```

4. Build and run.

```bash
make build
RUN_ENV=local make run
```

Equivalent script command:

```bash
./scripts/dockmon.sh build
RUN_ENV=local ./scripts/dockmon.sh run
```

## Run with Docker

Build:

```bash
make docker-build
```

Run:

```bash
RUN_ENV=local make docker-run
```

Manual example:

```bash
docker run -d --name dockmon \
  -p 8085:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$(pwd)/bin/configs":/bin/configs \
  -e APP_NAME=dockmon \
  -e RUN_ENV=local \
  dockmon:latest
```

## Configuration

Config files are loaded from `bin/configs/<RUN_ENV>.json`.

Environment variables:

- `RUN_ENV`: selects config file (`local`, `dev`, `prod`), default is `local`.
- `APP_NAME`: overrides `system.name` at runtime.

Important config blocks:

- `system`: run mode, HTTP port, JWT, language.
- `databases`: MySQL setup and retry policy.
- `redis`: redis clients (Dockmon expects a `dockmon` entry).
- `collector`:
  - `monitor_self`: auto-add current app container when running in Docker.
  - `container_name`: monitored container names.
  - `time_layout`: accepted timestamp formats.
  - `unstructured_log_line_flags`: line prefixes treated as new unstructured blocks.

## HTTP API

Base group: `/dockmon`

Health endpoints:

- `GET /dockmon/internal/ping`
- `GET /dockmon/internal/admin/ping`
- `GET /dockmon/internal/service/ping`
- `GET /dockmon/external/ping`
- `GET /dockmon/external/app/ping`
- `GET /dockmon/external/service/ping`

Auth endpoints:

- `POST /dockmon/internal/service/server/auth/token`
  - Body type: `application/x-www-form-urlencoded`
  - Fields: `app_id`, `app_secret`
  - Returns JWT token and `expires_in`

- `POST /dockmon/internal/service/server/auth/app`
  - Protected by `Authorization` header token (raw JWT string)
  - Body type: JSON
  - Fields: `app_name`, `description`, `redirect_uri`
  - Returns generated `app_id` and `app_secret`

Get token example:

```bash
curl -X POST "http://127.0.0.1:8080/dockmon/internal/service/server/auth/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "app_id=<app_id>&app_secret=<app_secret>"
```

Create app example:

```bash
curl -X POST "http://127.0.0.1:8080/dockmon/internal/service/server/auth/app" \
  -H "Authorization: <jwt_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "app_name": "my-service",
    "description": "internal service",
    "redirect_uri": "https://example.com/callback"
  }'
```

## Testing

Run all tests:

```bash
go test ./...
```

Run monitor package tests only:

```bash
go test ./app/monitor -v
```

## Operational Notes

- Docker socket mount is required (`/var/run/docker.sock`).
- The collector expects Redis key space for timestamp checkpoints and scheduler locks.
- `auth_app` table must exist before calling auth endpoints.
- Config samples use stdout logging by default. If switching to file logging, ensure the log path key matches code expectations (`log_path`).

## License

MIT License. See [LICENSE](./LICENSE).
