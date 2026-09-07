import { lazy, Suspense, useDeferredValue, useMemo, useState } from "react"
import {
  ActivityIcon,
  CalendarClockIcon,
  PlusIcon,
  RefreshCwIcon,
  SearchIcon,
  ServerCogIcon,
  SparklesIcon,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { ExecutionRail } from "@/features/tasks/execution-rail"
import { TaskTable } from "@/features/tasks/task-table"
import type { Task, TaskKind } from "@/features/tasks/types"
import { useTasks } from "@/features/tasks/use-tasks"

type Filter = "all" | TaskKind
const TaskEditor = lazy(() =>
  import("@/features/tasks/task-editor").then((module) => ({
    default: module.TaskEditor,
  }))
)

export default function App() {
  const service = useTasks()
  const [filter, setFilter] = useState<Filter>("all")
  const [query, setQuery] = useState("")
  const deferredQuery = useDeferredValue(query.trim().toLowerCase())
  const [selectedId, setSelectedId] = useState<string>()
  const [editorOpen, setEditorOpen] = useState(false)
  const [editing, setEditing] = useState<Task | null>(null)
  const [initialKind, setInitialKind] = useState<TaskKind>("resident")

  const selected =
    service.tasks.find((task) => task.id === selectedId) ??
    service.tasks[0] ??
    null
  const visibleTasks = useMemo(
    () =>
      service.tasks.filter((task) => {
        if (filter !== "all" && task.kind !== filter) return false
        if (!deferredQuery) return true
        return `${task.name} ${task.description} ${task.command}`
          .toLowerCase()
          .includes(deferredQuery)
      }),
    [service.tasks, filter, deferredQuery]
  )
  const running = service.tasks.filter(
    (task) => task.runtime.state === "running"
  ).length
  const enabledSchedules = service.tasks.filter(
    (task) => task.kind === "scheduled" && task.enabled
  ).length

  function create(kind: TaskKind) {
    setEditing(null)
    setInitialKind(kind)
    setEditorOpen(true)
  }
  function edit(task: Task) {
    setEditing(task)
    setInitialKind(task.kind)
    setEditorOpen(true)
  }

  return (
    <main className="min-h-svh bg-background text-foreground">
      <header className="border-b bg-background">
        <div className="mx-auto flex h-16 max-w-[1680px] items-center justify-between gap-4 px-4 md:px-6">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-sm">
              <SparklesIcon className="size-4" />
            </div>
            <div className="min-w-0">
              <h1 className="truncate text-base font-semibold">Rooster</h1>
              <p className="truncate text-xs text-muted-foreground">
                任务守护与调度
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={() => void service.refresh()}
                  />
                }
              >
                <RefreshCwIcon />
                <span className="sr-only">刷新</span>
              </TooltipTrigger>
              <TooltipContent>刷新任务</TooltipContent>
            </Tooltip>
            <Button onClick={() => create("resident")}>
              <PlusIcon data-icon="inline-start" />
              创建任务
            </Button>
          </div>
        </div>
      </header>

      <section className="mx-auto max-w-[1680px] px-4 py-4 md:px-6">
        <div className="grid grid-cols-3 divide-x overflow-hidden rounded-lg border bg-card">
          <Metric
            icon={<ActivityIcon />}
            label="运行中"
            value={running}
            tone="status"
          />
          <Metric
            icon={<ServerCogIcon />}
            label="常驻任务"
            value={
              service.tasks.filter((task) => task.kind === "resident").length
            }
            tone="primary"
          />
          <Metric
            icon={<CalendarClockIcon />}
            label="调度启用"
            value={enabledSchedules}
            tone="schedule"
          />
        </div>
      </section>

      <section className="mx-auto grid max-w-[1680px] grid-cols-1 gap-4 px-4 pb-5 md:px-6 xl:grid-cols-[minmax(0,1fr)_420px]">
        <div className="min-w-0 overflow-hidden rounded-lg border bg-card">
          <div className="flex flex-col gap-3 border-b p-3 sm:flex-row sm:items-center sm:justify-between">
            <Tabs
              value={filter}
              onValueChange={(value) => setFilter(value as Filter)}
            >
              <TabsList>
                <TabsTrigger value="all">全部</TabsTrigger>
                <TabsTrigger value="resident">常驻</TabsTrigger>
                <TabsTrigger value="scheduled">定时</TabsTrigger>
              </TabsList>
            </Tabs>
            <div className="flex items-center gap-2">
              <div className="relative min-w-0 flex-1 sm:w-64 sm:flex-none">
                <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  className="pl-8"
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="搜索任务或命令"
                  aria-label="搜索任务"
                />
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={() => create("scheduled")}
              >
                <CalendarClockIcon data-icon="inline-start" />
                新建定时
              </Button>
            </div>
          </div>
          <TaskTable
            tasks={visibleTasks}
            loading={service.loading}
            selectedId={selected?.id}
            onSelect={(task) => setSelectedId(task.id)}
            onEdit={edit}
            onDelete={service.remove}
            onToggle={service.toggle}
            onRun={service.run}
            onStop={service.stop}
            onCreate={() =>
              create(filter === "scheduled" ? "scheduled" : "resident")
            }
          />
        </div>
        <aside className="min-h-[360px] overflow-hidden rounded-lg border bg-card xl:h-[calc(100svh-13.25rem)]">
          <ExecutionRail key={selected?.id ?? "empty"} task={selected} />
        </aside>
      </section>

      {editorOpen ? (
        <Suspense fallback={null}>
          <TaskEditor
            key={editing?.id ?? `new-${initialKind}`}
            open
            task={editing}
            initialKind={initialKind}
            onOpenChange={setEditorOpen}
            onCreate={service.create}
            onUpdate={service.update}
          />
        </Suspense>
      ) : null}
    </main>
  )
}

function Metric({
  icon,
  label,
  value,
  tone,
}: {
  icon: React.ReactNode
  label: string
  value: number
  tone: "primary" | "status" | "schedule"
}) {
  return (
    <div className="flex min-w-0 items-center gap-3 px-3 py-3 sm:px-5">
      <div
        data-tone={tone}
        className="metric-icon flex size-8 shrink-0 items-center justify-center rounded-md"
      >
        {icon}
      </div>
      <div className="min-w-0">
        <div className="text-lg leading-none font-semibold tabular-nums">
          {value}
        </div>
        <div className="mt-1 truncate text-xs text-muted-foreground">
          {label}
        </div>
      </div>
    </div>
  )
}
