import { lazy, Suspense, useDeferredValue, useMemo, useState } from "react"
import {
  ArrowUpDownIcon,
  CalendarClockIcon,
  PlusIcon,
  RefreshCwIcon,
  SearchIcon,
  SparklesIcon,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { ExecutionRail } from "@/features/tasks/execution-rail"
import { AgentAccess } from "@/features/agent-access"
import { TaskTable } from "@/features/tasks/task-table"
import type { Task, TaskKind } from "@/features/tasks/types"
import { useTasks } from "@/features/tasks/use-tasks"

type Filter = "all" | "running" | TaskKind
type Sort = "status" | "enabled" | "updated" | "name"

const sortItems: Array<{ label: string; value: Sort }> = [
  { label: "运行状态优先", value: "status" },
  { label: "自动启用优先", value: "enabled" },
  { label: "最近更新", value: "updated" },
  { label: "名称", value: "name" },
]

const runtimeOrder: Record<Task["runtime"]["state"], number> = {
  running: 0,
  starting: 1,
  stopping: 2,
  backoff: 3,
  failed: 4,
  idle: 5,
}

const TaskEditor = lazy(() =>
  import("@/features/tasks/task-editor").then((module) => ({
    default: module.TaskEditor,
  }))
)

export default function App() {
  const service = useTasks()
  const [filter, setFilter] = useState<Filter>("all")
  const [sort, setSort] = useState<Sort>("status")
  const [query, setQuery] = useState("")
  const deferredQuery = useDeferredValue(query.trim().toLowerCase())
  const [selectedId, setSelectedId] = useState<string>()
  const [editorOpen, setEditorOpen] = useState(false)
  const [editing, setEditing] = useState<Task | null>(null)
  const [initialKind, setInitialKind] = useState<TaskKind>("resident")
  const [ordering, setOrdering] = useState<{ sort: Sort; ids: string[] }>({
    sort: "status",
    ids: [],
  })

  // Live updates change row contents, but only an explicit sort changes positions.
  let orderedIds = ordering.ids
  if (
    ordering.sort !== sort ||
    (ordering.ids.length === 0 && service.tasks.length > 0)
  ) {
    orderedIds = service.tasks
      .toSorted((left, right) => compareTasks(left, right, sort))
      .map((task) => task.id)
    setOrdering({ sort, ids: orderedIds })
  } else {
    const known = new Set(ordering.ids)
    const added = service.tasks.filter((task) => !known.has(task.id))
    if (added.length > 0) {
      orderedIds = [...ordering.ids, ...added.map((task) => task.id)]
      setOrdering({ sort, ids: orderedIds })
    }
  }

  const visibleTasks = useMemo(() => {
    const positions = new Map(orderedIds.map((id, index) => [id, index]))
    return service.tasks
      .filter((task) => {
        if (filter === "running" && task.runtime.state !== "running")
          return false
        if (filter !== "all" && filter !== "running" && task.kind !== filter)
          return false
        if (!deferredQuery) return true
        return `${task.name} ${task.description} ${task.command}`
          .toLowerCase()
          .includes(deferredQuery)
      })
      .toSorted(
        (left, right) => positions.get(left.id)! - positions.get(right.id)!
      )
  }, [service.tasks, filter, deferredQuery, orderedIds])
  const counts = useMemo(
    () =>
      service.tasks.reduce(
        (totals, task) => {
          totals.all += 1
          if (task.runtime.state === "running") totals.running += 1
          if (task.kind === "resident") totals.resident += 1
          else totals.scheduled += 1
          return totals
        },
        { all: 0, running: 0, resident: 0, scheduled: 0 }
      ),
    [service.tasks]
  )
  const selected =
    visibleTasks.find((task) => task.id === selectedId) ??
    visibleTasks[0] ??
    null

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
            <AgentAccess />
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

      <section className="mx-auto grid max-w-[1680px] grid-cols-1 gap-4 px-4 py-4 md:px-6 xl:grid-cols-[minmax(0,1fr)_420px]">
        <div className="min-w-0 overflow-hidden rounded-lg border bg-card">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b p-3">
            <Tabs
              value={filter}
              onValueChange={(value) => setFilter(value as Filter)}
            >
              <TabsList>
                <TaskTab value="all" label="全部" count={counts.all} />
                <TaskTab
                  value="running"
                  label="运行中"
                  count={counts.running}
                />
                <TaskTab
                  value="resident"
                  label="常驻"
                  count={counts.resident}
                />
                <TaskTab
                  value="scheduled"
                  label="定时"
                  count={counts.scheduled}
                />
              </TabsList>
            </Tabs>
            <div className="flex w-full min-w-0 flex-wrap items-center justify-end gap-2 lg:w-auto lg:flex-1">
              <div className="relative min-w-0 basis-full sm:flex-1 sm:basis-auto lg:w-56 lg:flex-none">
                <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  className="pl-8"
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="搜索任务或命令"
                  aria-label="搜索任务"
                />
              </div>
              <Select
                items={sortItems}
                value={sort}
                onValueChange={(value) => value && setSort(value as Sort)}
              >
                <SelectTrigger className="w-40" aria-label="任务排序">
                  <ArrowUpDownIcon />
                  <SelectValue />
                </SelectTrigger>
                <SelectContent align="end">
                  <SelectGroup>
                    {sortItems.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
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
        <aside className="min-h-[360px] overflow-hidden rounded-lg border bg-card xl:h-[calc(100svh-6rem)]">
          <ExecutionRail key={selected?.id ?? "empty"} task={selected} />
        </aside>
      </section>

      {editorOpen ? (
        <Suspense fallback={null}>
          <TaskEditor
            key={editing?.id ?? `new-${initialKind}`}
            open
            task={
              service.tasks.find((task) => task.id === editing?.id) ?? editing
            }
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

function TaskTab({
  value,
  label,
  count,
}: {
  value: Filter
  label: string
  count: number
}) {
  return (
    <TabsTrigger value={value}>
      {label}
      <Badge variant="secondary">{count}</Badge>
    </TabsTrigger>
  )
}

function compareTasks(left: Task, right: Task, sort: Sort) {
  if (sort === "status") {
    const status =
      runtimeOrder[left.runtime.state] - runtimeOrder[right.runtime.state]
    if (status !== 0) return status
  }
  if (sort === "enabled") {
    const enabled = Number(right.enabled) - Number(left.enabled)
    if (enabled !== 0) return enabled
  }
  if (sort === "updated") {
    return Date.parse(right.updatedAt) - Date.parse(left.updatedAt)
  }
  return left.name.localeCompare(right.name, "zh-CN")
}
