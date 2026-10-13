---
name: rooster
description: Manage local Rooster resident processes and scheduled commands through HTTP, including task configuration, execution history, and logs. Use when asked to operate Rooster without its web dashboard.
---

# Rooster

Use the local HTTP API to perform the user's requested task. Read
[references/api.md](references/api.md) for endpoints, fields, defaults, and errors.
For a skill loaded directly over HTTP, fetch `/api/docs` from the same server
instead of resolving the relative reference.

## Connect

Use the Rooster address supplied by the user or running application's logs.
The default is `http://127.0.0.1:9090`; Rooster may select a later port if occupied.
Set `ROOSTER_BASE_URL` to the actual address and verify `GET /api/health`.
Do not assume a failed connection means there are no tasks.

```sh
export ROOSTER_BASE_URL=http://127.0.0.1:9090
curl --fail-with-body "$ROOSTER_BASE_URL/api/health"
curl --fail-with-body "$ROOSTER_BASE_URL/api/docs"
```

## Operate

- Find tasks with `GET /api/tasks`; use IDs from responses, never infer them
  from names. Names are not unique. Resolve ambiguous matches before changing them.
- Create with `POST /api/tasks`. Prefer `enabled: false` to configure first;
  enabling a resident task starts its supervisor immediately, while enabling a
  scheduled task activates its cron schedule. Recheck existing tasks before
  creating to avoid duplicates.
- For a single manual run, `POST /api/tasks/{id}/executions` works even when
  disabled. Retain the returned execution ID and poll that ID in
  `GET /api/tasks/{id}/executions?limit=200`. A `202` response means accepted,
  not completed. Terminal states are `succeeded`, `failed`, and `canceled`.
  If history no longer includes the ID, completion is unknown.
- Read output with `GET /api/executions/{id}/logs`; this is plain text containing
  at most the last 64 KiB, not JSON or a live stream.
- Before editing or deleting, disable the task, stop any remaining active
  executions, and wait for their terminal states. Disabling a scheduled task
  does not stop existing executions. Stopping an enabled resident execution
  also disables its automation so its supervisor does not restart it.
- Update with `PUT /api/tasks/{id}` using the full configuration from the task
  list, preserving unrelated fields and `kind`. PUT replaces configuration;
  it is not a partial update. Re-enable only when the requested outcome requires it.
- On `409`, inspect active executions. On `422`, read the `error` field and
  correct the request or lifecycle precondition. On a timeout after a mutation,
  inspect tasks/history before retrying: create and run are not idempotent.

Poll with a delay and a bounded deadline appropriate to the command. Report
pending work when the deadline expires rather than claiming completion.
Only make changes within the user's requested scope. Commands execute with
the privileges of the local Rooster process.
