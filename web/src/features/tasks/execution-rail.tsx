import { useEffect, useState } from "react"
import {
  Clock3Icon,
  FileTextIcon,
  LoaderCircleIcon,
  RotateCwIcon,
  TerminalSquareIcon,
} from "lucide-react"
import { toast } from "sonner"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Separator } from "@/components/ui/separator"
import { api } from "./api"
import { messageOf } from "./use-tasks"
import type { Execution, Task } from "./types"

export function ExecutionRail({ task }: { task: Task | null }) {
  const taskId = task?.id
  const [executions, setExecutions] = useState<Execution[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [log, setLog] = useState("")
  const [loading, setLoading] = useState(false)
  const selected =
    executions.find((execution) => execution.id === selectedId) ?? null

  useEffect(() => {
    if (!taskId) return
    let active = true
    async function load(quiet = false) {
      if (!quiet) setLoading(true)
      try {
        const items = await api.executions(taskId!)
        if (active) setExecutions(items)
      } catch (error) {
        if (!quiet) toast.error(messageOf(error))
      } finally {
        if (active) setLoading(false)
      }
    }
    void load()
    const timer = window.setInterval(() => void load(true), 2_000)
    return () => {
      active = false
      window.clearInterval(timer)
    }
  }, [taskId])

  useEffect(() => {
    if (!selectedId) return
    let active = true
    async function loadLog() {
      try {
        const value = await api.log(selectedId!)
        if (active) setLog(value)
      } catch (error) {
        if (active) setLog(messageOf(error))
      }
    }
    const initialTimer = window.setTimeout(() => void loadLog(), 0)
    const timer = window.setInterval(() => void loadLog(), 1_000)
    return () => {
      active = false
      window.clearTimeout(initialTimer)
      window.clearInterval(timer)
    }
  }, [selectedId])

  function inspect(execution: Execution) {
    setLog("正在读取日志...")
    setSelectedId(execution.id)
  }

  if (!task)
    return (
      <Empty className="h-full border-0">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <TerminalSquareIcon />
          </EmptyMedia>
          <EmptyTitle>选择一个任务</EmptyTitle>
          <EmptyDescription>这里会显示它最近的执行和日志。</EmptyDescription>
        </EmptyHeader>
      </Empty>
    )

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-start justify-between gap-3 px-4 py-4">
        <div className="min-w-0">
          <h2 className="truncate text-sm font-semibold">{task.name}</h2>
          <p className="mt-1 text-xs text-muted-foreground">执行记录</p>
        </div>
        <Badge variant={task.kind === "resident" ? "default" : "secondary"}>
          {task.kind === "resident" ? "常驻" : "定时"}
        </Badge>
      </div>
      <Separator />
      {selected ? (
        <div className="flex min-h-0 flex-1 flex-col">
          <div className="flex items-center justify-between gap-2 px-4 py-3">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                setSelectedId(null)
                setLog("")
              }}
            >
              返回记录
            </Button>
            <ExecutionBadge status={selected.status} />
          </div>
          <ScrollArea className="min-h-0 flex-1">
            <pre className="min-h-full bg-terminal p-4 font-mono text-xs leading-5 break-all whitespace-pre-wrap text-terminal-foreground">
              {log || "暂无输出"}
            </pre>
          </ScrollArea>
        </div>
      ) : executions.length === 0 && !loading ? (
        <Empty className="border-0">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <FileTextIcon />
            </EmptyMedia>
            <EmptyTitle>还没有执行记录</EmptyTitle>
            <EmptyDescription>
              运行任务后，结果和日志会出现在这里。
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <ScrollArea className="min-h-0 flex-1">
          <div className="flex flex-col p-2">
            {loading ? (
              <div className="flex items-center justify-center gap-2 py-12 text-xs text-muted-foreground">
                <LoaderCircleIcon className="size-4 animate-spin" />
                加载中
              </div>
            ) : (
              executions.map((execution) => (
                <button
                  key={execution.id}
                  className="flex w-full items-center gap-3 rounded-lg px-3 py-3 text-left transition-colors hover:bg-accent"
                  onClick={() => void inspect(execution)}
                >
                  <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-muted">
                    <Clock3Icon className="size-4" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <ExecutionBadge status={execution.status} />
                      <span className="text-xs text-muted-foreground">
                        {triggerLabel(execution.trigger)}
                      </span>
                    </div>
                    <p className="mt-1 truncate text-xs text-muted-foreground">
                      {formatDate(execution.startedAt || execution.createdAt)}
                      {execution.exitCode !== undefined
                        ? ` · exit ${execution.exitCode}`
                        : ""}
                    </p>
                  </div>
                </button>
              ))
            )}
          </div>
        </ScrollArea>
      )}
    </div>
  )
}

function ExecutionBadge({ status }: { status: Execution["status"] }) {
  const running =
    status === "running" || status === "starting" || status === "stopping"
  if (running)
    return (
      <Badge variant="secondary">
        <RotateCwIcon className="animate-spin" data-icon="inline-start" />
        {status === "running"
          ? "运行中"
          : status === "stopping"
            ? "停止中"
            : "启动中"}
      </Badge>
    )
  if (status === "succeeded")
    return <Badge className="bg-status text-status-foreground">成功</Badge>
  if (status === "failed") return <Badge variant="destructive">失败</Badge>
  return <Badge variant="outline">已取消</Badge>
}
function triggerLabel(trigger: Execution["trigger"]) {
  return trigger === "manual"
    ? "手动"
    : trigger === "schedule"
      ? "调度"
      : "守护"
}
function formatDate(value: string) {
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(new Date(value))
}
