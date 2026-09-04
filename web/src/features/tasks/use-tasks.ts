import { useCallback, useEffect, useRef, useState } from "react"
import { toast } from "sonner"

import { api } from "./api"
import type { Task, TaskDraft } from "./types"

export function useTasks() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(true)
  const mounted = useRef(true)

  const refresh = useCallback(async (quiet = false) => {
    try {
      const next = await api.tasks()
      if (mounted.current) setTasks(next)
    } catch (error) {
      if (!quiet) toast.error(messageOf(error))
    } finally {
      if (mounted.current) setLoading(false)
    }
  }, [])

  useEffect(() => {
    mounted.current = true
    const initialTimer = window.setTimeout(() => void refresh(), 0)
    const timer = window.setInterval(() => void refresh(true), 2_000)
    return () => {
      mounted.current = false
      window.clearTimeout(initialTimer)
      window.clearInterval(timer)
    }
  }, [refresh])

  const act = useCallback(
    async (operation: () => Promise<unknown>, success: string) => {
      try {
        await operation()
        toast.success(success)
        await refresh(true)
      } catch (error) {
        toast.error(messageOf(error))
        throw error
      }
    },
    [refresh]
  )

  return {
    tasks,
    loading,
    refresh,
    create: (draft: TaskDraft) =>
      act(() => api.createTask(draft), "任务已创建"),
    update: (task: Task) => act(() => api.updateTask(task), "任务已保存"),
    remove: (id: string) => act(() => api.deleteTask(id), "任务已删除"),
    toggle: (id: string, enabled: boolean) =>
      act(
        () => api.setEnabled(id, enabled),
        enabled ? "自动运行已启用" : "自动运行已停用"
      ),
    run: (id: string) => act(() => api.run(id), "任务已启动"),
    stop: (executionId: string) =>
      act(() => api.stop(executionId), "正在停止任务"),
  }
}

export function messageOf(error: unknown) {
  return error instanceof Error ? error.message : "操作失败"
}
