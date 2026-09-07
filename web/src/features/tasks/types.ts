export type TaskKind = "resident" | "scheduled"
export type CommandMode = "shell" | "exec"
export type RuntimeState =
  "idle" | "running" | "starting" | "stopping" | "backoff" | "failed"

export interface Task {
  id: string
  name: string
  description: string
  link: string
  kind: TaskKind
  enabled: boolean
  commandMode: CommandMode
  command: string
  arguments: string[]
  workingDir: string
  shell: string
  environment: Record<string, string>
  schedule: string
  overlapPolicy: "skip" | "parallel"
  restartPolicy: "on_failure" | "always" | "never"
  maxRetries: number
  minRunSeconds: number
  createdAt: string
  updatedAt: string
  nextRuns?: string[]
  runtime: {
    state: RuntimeState
    executionId?: string
    pid?: number
  }
}

export interface Execution {
  id: string
  taskId: string
  trigger: "manual" | "schedule" | "supervisor"
  status:
    | "queued"
    | "starting"
    | "running"
    | "stopping"
    | "succeeded"
    | "failed"
    | "canceled"
  pid?: number
  startedAt?: string
  finishedAt?: string
  exitCode?: number
  error?: string
  logPath: string
  createdAt: string
}

export type TaskDraft = Omit<
  Task,
  "id" | "createdAt" | "updatedAt" | "runtime" | "nextRuns"
>

export const emptyTask = (kind: TaskKind = "resident"): TaskDraft => ({
  name: "",
  description: "",
  link: "",
  kind,
  enabled: false,
  commandMode: "shell",
  command: "",
  arguments: [],
  workingDir: "",
  shell: "",
  environment: {},
  schedule: kind === "scheduled" ? "0 * * * *" : "",
  overlapPolicy: "skip",
  restartPolicy: "on_failure",
  maxRetries: 3,
  minRunSeconds: 10,
})
