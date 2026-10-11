# Rooster HTTP API

## Connection and formats

Base URL: the running Rooster server, normally `http://127.0.0.1:9090`.
`ROOSTER_LISTEN` chooses the preferred bind address; an occupied port causes
Rooster to try the next 100 ports. Use the actual address from its startup logs.
The server binds to loopback by default and has no authentication layer.

All paths below are relative to that base URL. JSON requests use
`Content-Type: application/json`. Field names are case-sensitive camelCase.
Unknown request fields are rejected with 422. Timestamps are RFC 3339 strings.
No pagination cursor, idempotency key, PATCH, or bulk operation is supported.

## Endpoints

| Method | Path | Success | Request and response |
| --- | --- | --- | --- |
| GET | `/api/skill` | 200 | Markdown SKILL.md with YAML frontmatter |
| GET | `/api/docs` | 200 | This API reference as Markdown |
| GET | `/api/health` | 200 | `{"status":"ok"}` |
| GET | `/api/tasks` | 200 | `{"tasks":[TaskView,...]}`; empty list is `[]` |
| POST | `/api/tasks` | 201 | Task configuration body; returns Task |
| PUT | `/api/tasks/{id}` | 200 | Full Task configuration; returns Task; disable and stop first |
| DELETE | `/api/tasks/{id}` | 204 | No body; disable and stop first; empty response |
| POST | `/api/tasks/{id}/enable` | 200 | No body; `{"enabled":true}` |
| POST | `/api/tasks/{id}/disable` | 200 | No body; `{"enabled":false}` |
| POST | `/api/tasks/{id}/executions` | 202 | No body; starts one manual execution; returns Execution |
| GET | `/api/tasks/{id}/executions?limit=50` | 200 | `{"executions":[Execution,...]}`, newest first |
| POST | `/api/executions/{id}/stop` | 202 | No body; `{"status":"stopping"}`; asynchronous |
| GET | `/api/executions/{id}/logs` | 200 | `text/plain; charset=utf-8`, last 64 KiB of current log file |

`{id}` is a server-generated task or execution UUID, according to the path.
Execution-list `limit` is an integer from 1 to 200; missing, invalid, zero,
negative, or greater than 200 uses 50. An unknown task's execution list can
return an empty array; use the task list to check existence.
Logs return 404 when the execution or its current log file is absent. A log
can be unavailable briefly after dispatch. Rotation means this endpoint does
not include archived log files. Stop returns 404 for an inactive execution.

## Task configuration

| Field | Type | Requirement / default / behavior |
| --- | --- | --- |
| `name` | string | Required, nonblank; names need not be unique |
| `description` | string | Optional, default empty |
| `link` | string | Optional; empty or absolute HTTP/HTTPS URL |
| `kind` | string | Required: `resident` or `scheduled`; cannot change on PUT |
| `enabled` | boolean | Default false; activates supervisor or schedule |
| `commandMode` | string | `shell` (default) or `exec` |
| `command` | string | Required, nonblank; shell command or executable |
| `arguments` | string[] | Default `[]`; executable arguments in exec mode |
| `workingDir` | string | Optional; empty inherits Rooster's working directory |
| `shell` | string | Optional executable for shell mode; empty uses OS default |
| `environment` | object of string values | Default `{}`; overrides inherited cached environment |
| `schedule` | string | Required for scheduled tasks; five-field cron expression |
| `overlapPolicy` | string | `skip` (default) or `parallel`; skip blocks another active execution |
| `restartPolicy` | string | `on_failure` (default), `always`, or `never`; for resident supervisor |
| `maxRetries` | integer | Nonnegative; omitted or 0 becomes 3 |
| `minRunSeconds` | integer | Nonnegative; omitted or 0 becomes 10 |

Environment keys must be nonempty and contain neither `=` nor NUL; values
must not contain NUL. Prefer absolute executable and working-directory paths
when the process must not depend on Rooster's startup environment.

Cron normally uses the Rooster process's local timezone; `CRON_TZ=Asia/Shanghai`
may prefix the expression to select a timezone. Standard cron descriptors
such as `@hourly` are also accepted by the parser.

A resident supervisor considers a nonzero exit or a run shorter than
`minRunSeconds` a failure. `maxRetries` bounds consecutive failures; reaching
that count disables the task. Zero does not mean unlimited retries in this API.
The execution's status still reflects the process exit, so a short successful
execution can be restarted by its supervisor.

Task responses additionally contain `id`, `createdAt`, and `updatedAt`.
Create generates the ID and timestamps. PUT uses the path ID and retains the
stored creation timestamp, though its response echoes the supplied `createdAt`
(or the zero timestamp if omitted). Read the task list for stored timestamps;
omit server-owned fields in new requests.
TaskView adds `runtime` and, for scheduled tasks, `nextRuns` (up to five upcoming
timestamps). `runtime` has `state` and optional `executionId` and `pid`.
States include `idle`, `starting`, `running`, `stopping`, `backoff`, and `failed`.
For parallel runs it describes one active execution; use history for all runs.
PUT accepts TaskView's `runtime` and `nextRuns` but ignores them.

PUT replaces configuration: omitted fields revert to zero values/defaults.
Existing enabled tasks and tasks with active executions cannot be edited or
deleted (422). Preserve `kind` on PUT; changing it currently returns 404.
Disabling a scheduled task removes future triggers without stopping current
runs. Disabling a resident supervisor requests stop of its managed executions;
manually started executions may still need explicit stop.

## Execution response

| Field | Type | Meaning |
| --- | --- | --- |
| `id` | string | Execution UUID |
| `taskId` | string | Parent task UUID |
| `trigger` | string | `manual`, `schedule`, or `supervisor` |
| `status` | string | `queued`, `starting`, `running`, `stopping`, `succeeded`, `failed`, `canceled` |
| `pid` | integer | Process ID; omitted if zero |
| `startedAt` | string | Optional start timestamp |
| `finishedAt` | string | Optional finish timestamp |
| `exitCode` | integer | Optional exit code; zero is included when available |
| `error` | string | Optional failure explanation |
| `logPath` | string | Local log file path |
| `createdAt` | string | Creation timestamp |

Run is allowed even for disabled tasks. `202` is not a completion guarantee;
poll history using the returned ID. A process-launch failure may return 422
while recording a failed execution. Inspect history before retrying.

## Errors

Operation errors use `{"error":"explanation"}`:

- 404: missing task/execution/log, inactive execution on stop, or changed kind on PUT.
- 409: another execution is active and overlap policy is skip.
- 422: malformed JSON, unknown fields, invalid configuration/cron, unmet lifecycle
  precondition, process launch error, or another engine/storage error.

HTTP routing errors such as 405 may be plain text. Check status before parsing.
Mutations are not transactional across persistence and process reconciliation:
an error or connection loss can occur after a change is stored. Inspect current
state before retrying, especially POST create and POST run.

## Example: configure and manually run a scheduled command

```sh
export ROOSTER_BASE_URL=http://127.0.0.1:9090
curl --fail-with-body "$ROOSTER_BASE_URL/api/tasks" \
  -H 'Content-Type: application/json' \
  --data '{"name":"hourly-check","kind":"scheduled","enabled":false,"commandMode":"shell","command":"echo ready","schedule":"0 * * * *","overlapPolicy":"skip","restartPolicy":"never"}'
```

Use the returned `id` as `TASK_ID`:

```sh
curl --fail-with-body -X POST "$ROOSTER_BASE_URL/api/tasks/$TASK_ID/executions"
curl --fail-with-body "$ROOSTER_BASE_URL/api/tasks/$TASK_ID/executions?limit=200"
```

Use the returned execution `id` as `EXECUTION_ID`:

```sh
curl --fail-with-body "$ROOSTER_BASE_URL/api/executions/$EXECUTION_ID/logs"
```

To activate recurring execution, POST `/api/tasks/$TASK_ID/enable`.
