import type { Execution, Task, TaskDraft } from "./types"

class ApiError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: init?.body
      ? { "Content-Type": "application/json", ...init.headers }
      : init?.headers,
  })
  if (!response.ok) {
    const body = await response
      .json()
      .catch(() => ({ error: response.statusText }))
    throw new ApiError(body.error || response.statusText, response.status)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export const api = {
  async tasks() {
    const result = await request<{ tasks: Task[] | null }>("/api/tasks")
    return Array.isArray(result.tasks) ? result.tasks : []
  },
  createTask(task: TaskDraft) {
    return request<Task>("/api/tasks", {
      method: "POST",
      body: JSON.stringify(task),
    })
  },
  updateTask(task: Task) {
    return request<Task>(`/api/tasks/${task.id}`, {
      method: "PUT",
      body: JSON.stringify(task),
    })
  },
  deleteTask(id: string) {
    return request<void>(`/api/tasks/${id}`, { method: "DELETE" })
  },
  setEnabled(id: string, enabled: boolean) {
    return request<{ enabled: boolean }>(
      `/api/tasks/${id}/${enabled ? "enable" : "disable"}`,
      { method: "POST" }
    )
  },
  run(id: string) {
    return request<Execution>(`/api/tasks/${id}/executions`, { method: "POST" })
  },
  stop(executionId: string) {
    return request<{ status: string }>(`/api/executions/${executionId}/stop`, {
      method: "POST",
    })
  },
  async executions(taskId: string) {
    const result = await request<{ executions: Execution[] | null }>(
      `/api/tasks/${taskId}/executions?limit=50`
    )
    return Array.isArray(result.executions) ? result.executions : []
  },
  async log(executionId: string) {
    const response = await fetch(`/api/executions/${executionId}/logs`)
    if (!response.ok) throw new ApiError("日志尚未生成", response.status)
    return response.text()
  },
}
