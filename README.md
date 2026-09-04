# Rooster

Rooster is a local task supervisor for long-running processes and scheduled commands.
It combines a native tray application, a local web console, durable execution history,
and process-tree aware shutdown.

## Capabilities

- Resident tasks with restart policies, bounded retries, and exponential backoff.
- Five-field cron schedules with skip or parallel overlap behavior.
- Shell execution with a cached login environment, or direct executable + arguments.
- Per-task environment overrides and working directories.
- SQLite task storage, schema migrations, and interrupted-execution recovery.
- Rotating file logs and execution history in the dashboard.
- Explicit starting, running, stopping, backoff, failed, and idle runtime states.
- macOS, Linux, and Windows process-tree termination implementations.

See [docs/architecture.md](docs/architecture.md) for package ownership and lifecycle rules.

## Development

Requirements: Go 1.25+, Node.js, and pnpm.

```sh
cd web
pnpm install
cd ..
make test
go run ./cmd/roosterd
```

The dashboard listens on `http://127.0.0.1:9090`. If the port is occupied, Rooster tries
the next 100 ports. Set `ROOSTER_LISTEN` to choose the preferred address and
`ROOSTER_DATA_DIR` to use an isolated database and log directory.

```sh
ROOSTER_DATA_DIR=/tmp/rooster-dev ROOSTER_LISTEN=127.0.0.1:19090 go run ./cmd/roosterd
```

Use `go run ./cmd/rooster` for the native tray entry point. Platform packaging still uses
Fyne's packaging toolchain.

## Data

By default, the database and logs live under the operating system's user configuration
directory in a `Rooster` folder. Logs rotate at 20 MB with three compressed backups.
The HTTP server binds to loopback by default and has no remote authentication layer.
