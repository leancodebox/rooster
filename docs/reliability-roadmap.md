# Reliability Development

## Isolation

- Develop on `codex/reliability-improvements`.
- All engine tests use temporary databases and logs.
- Development servers must use an explicit temporary `ROOSTER_DATA_DIR` and an independent loopback port.
- Do not run `build-macos.sh`, replace the desktop app, or connect tests to the production API.
- Sleep recovery and missed-schedule replay are outside this work.

## First Batch

- Preserve list positions during live refresh and task operations. Sort when initially loading or explicitly changing the sort option; append new tasks.
- Stopping an enabled resident execution disables automation and cancels its supervisor. Scheduled executions retain stop-current-execution behavior.
- A canceled supervisor exits before counting failures or applying restart policy.
- Reserve task startup atomically so concurrent requests cannot bypass skip-overlap policy.
- Track in-flight startup during engine shutdown.
- Acquire an OS-backed data-directory lock before opening the database or starting tasks.
- Retry startup failures with bounded exponential backoff; expose error, failure count, and next retry time.
- Identify supervisor ownership so an exiting old supervisor cannot delete a replacement.
- Bind supervised process startup to supervisor cancellation.
- Stop polling logs once an execution is terminal, fetching final output on transition.
- Add CI build/lint/test and internal-package race checks.
- Add task configuration export and consistent SQLite snapshot download. Snapshots use private temporary storage and contain committed WAL data.
- Add full-current-file log download, leaving the existing tail endpoint intact.
- Verify snapshots independently open and retain data after source changes; refuse destination overwrite.
- Add opt-in instance-wide execution timeout, global concurrency cap, and per-task concurrency cap.
- Track reservations until process completion and release them on startup failures. Record deadline expiration separately from manual cancellation.
- Add explicit retention preview/apply API with age plus per-task minimum history, managed-file checks, rotation archive cleanup, and partial-error reporting.

Validation: local frontend production build and ESLint passed; `go test -race ./internal/...` passed with temporary databases. Duplicate-instance, lock-release, startup-failure exhaustion, manual-stop, and concurrent-start regressions are covered. CI has not run remotely; abnormal process termination and rendered UI behavior still need validation.

## Remaining Work

- Validate abnormal-exit lock release and rapid disable/enable transitions end to end before release.
- Task-specific timeout/concurrency editing in the UI (current limits are instance-wide defaults).
- Retention UI and optional scheduled cleanup/total disk budget. The explicit preview/apply backend is implemented; it is not enabled automatically.
- Backup/export UI, guarded restore workflow, and log-directory portability. Snapshot reopening is covered; an interactive restore workflow is not implemented.
- Record skip, cancellation, timeout, and retry-exhaustion reasons; add failure notifications.
- Log search, copy/download, follow-output control, and historical log access.
- Retain stable task selection during runtime updates and expand lifecycle transition regressions.

## Notes

Task state and row order are separate concerns: refreshing operational state should not move the control the user just clicked.
Single-file rotation does not bound the total size of per-execution log directories.
Launch admission must be reserved before creating records or spawning a child, and released on every failure path.

## Execution Limits

The following process environment settings are optional; omitted or zero means no limit:

| Setting | Example | Meaning |
| --- | --- | --- |
| `ROOSTER_MAX_CONCURRENT` | `8` | Total starting/running executions |
| `ROOSTER_MAX_PER_TASK` | `2` | Starting/running executions per task, even when parallel overlap is enabled |
| `ROOSTER_EXECUTION_TIMEOUT` | `30m` | Maximum execution lifetime, including process startup; applies to resident and scheduled tasks |

Negative or malformed values prevent application startup. Capacity rejection returns HTTP 409; manual calls are not queued. Scheduled triggers at capacity are skipped; resident supervision waits for capacity without increasing its failure count. Timeout is opt-in because resident services normally run indefinitely. It records a failed execution with `context deadline exceeded` and follows the task's restart policy.
