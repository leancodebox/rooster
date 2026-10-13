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
- Review fixes: consume persisted results when a short command completes before supervisor lookup; keep the instance lock/database until asynchronous shutdown finishes.
- Add log-tail line filtering, copy displayed output, full-current-file download, and execution failure reason display in the log dialog. Build/lint passed; rendered interaction validation remains pending.
- Add data-maintenance dialog with configuration/database downloads, retention inputs, preview invalidation on changes, and explicit deletion confirmation. Mocked-API Playwright checks passed at desktop/mobile sizes; no production backend was contacted.
- Add log follow-output switch and in-page deduplicated retry-exhaustion notification. Show supervisor failure cause, consecutive failures, and next retry time in the execution rail. Mocked-API browser checks verify follow/off scrolling and notification deduplication.
- Add rotation archive listing/selection, bounded gzip tail preview, and original archive downloads. Backend regression covers archive content/download and traversal rejection; mocked-API browser checks cover selecting an archive and updating the download URL.
- Add offline `rooster-restore` command: require a new target directory, validate integrity/schema/references, disable restored tasks, mark interrupted executions, and rebase all history log paths into the target directory. Tests cover source preservation, isolation, existing-target rejection and invalid-backup cleanup.
- Serialize enable/disable persistence and reconciliation; wait for canceled supervisors to exit before replacement, including recovery after interrupted disable requests.
- Add subprocess forced-exit lock-release regression and five-cycle rapid resident disable/enable regression; both pass under the race detector using temporary directories.

Validation: local frontend production build and ESLint passed; `go test -race ./internal/...` passed with temporary databases. Duplicate-instance, forced-exit lock-release, startup-failure exhaustion, manual-stop, concurrent-start and rapid-toggle regressions are covered. Mocked-API browser checks cover maintenance confirmation and log controls. CI has not run remotely; release-wide integrated acceptance is still pending.

## Remaining Work

- Release-wide integrated acceptance with an isolated instance before production deployment.
- Task-specific timeout/concurrency editing in the UI (current limits are instance-wide defaults).
- Optional scheduled cleanup/total disk budget. Preview/apply UI and backend are implemented; cleanup is not enabled automatically.
- Optional interactive restore UI and log-bundle import. Guarded offline snapshot restore is implemented; existing directories are never overwritten and archived log files are not part of a database snapshot.
- Persist skip/cancellation reasons and add native tray notifications; in-page retry-exhaustion feedback and timeout failure reasons are implemented.
- Cross-file log search. Current and archived file selection, per-file tail filtering, follow mode, copy and downloads are implemented.
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
