# Rooster v2 architecture

Rooster is a local task supervisor. The product has four core workflows:

1. Define and edit a task.
2. Enable a resident task or a scheduled task.
3. Run and stop executions without leaving child processes behind.
4. Inspect current state, execution history, and logs.

## Domain model

- `Task` is durable user intent. It never contains locks, process handles, or cron IDs.
- `Execution` is one attempt to run a task. It owns status, PID, timestamps, exit data, and log path.
- `RuntimeState` is ephemeral process state derived from the active execution.
- `Supervisor` reconciles enabled resident tasks with the processes that should exist.
- `Scheduler` turns enabled schedules into execution requests.
- `Runner` owns exactly one child process from start through `Wait`.
- `Store` persists tasks and execution records transactionally.

`Task.Enabled` means the user wants automation active. It is deliberately separate from
the current execution state.

## Process lifecycle

Every successfully started process has exactly one goroutine that calls `Wait`. Stopping
an execution signals the complete process group/tree, waits for a grace period, and then
forces termination if required. The wait goroutine remains responsible for reaping the
direct child. Rooster shutdown first disables new dispatch, stops the scheduler, requests
all active executions to stop, and waits for every runner goroutine.

## Storage

SQLite stores task definitions and execution metadata. Task logs remain append-only files.
Schema changes use ordered migrations recorded in `schema_migrations`. A corrupt legacy
JSON file is never overwritten; legacy import will be an explicit operation.

## Packages

```text
cmd/roosterd            composition root and signals
internal/domain         durable model and validation
internal/store          persistence contracts
internal/store/sqlite   SQLite implementation and migrations
internal/runner         environment, command, and process lifecycle
internal/engine         scheduling and supervision
internal/httpapi        REST API and embedded web application
web                     React + Vite + shadcn dashboard
```

