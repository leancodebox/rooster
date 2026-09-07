import { useMemo, useState } from "react"
import {
  BracesIcon,
  CalendarClockIcon,
  EllipsisIcon,
  ExternalLinkIcon,
  HistoryIcon,
  PencilIcon,
  PlayIcon,
  ServerCogIcon,
  SquareIcon,
  Trash2Icon,
} from "lucide-react"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogMedia,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import { StatusBadge } from "./status-badge"
import type { Task } from "./types"

interface TaskTableProps {
  tasks: Task[]
  loading: boolean
  selectedId?: string
  onSelect: (task: Task) => void
  onEdit: (task: Task) => void
  onDelete: (id: string) => Promise<unknown>
  onToggle: (id: string, enabled: boolean) => Promise<unknown>
  onRun: (id: string) => Promise<unknown>
  onStop: (executionId: string) => Promise<unknown>
  onCreate: () => void
}

export function TaskTable(props: TaskTableProps) {
  const [pendingDelete, setPendingDelete] = useState<Task | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const rows = useMemo(() => props.tasks, [props.tasks])

  async function action(key: string, operation: () => Promise<unknown>) {
    setBusy(key)
    try {
      await operation()
    } finally {
      setBusy(null)
    }
  }

  if (props.loading)
    return (
      <div className="flex flex-col gap-2 p-4">
        {Array.from({ length: 6 }).map((_, index) => (
          <Skeleton className="h-12 w-full" key={index} />
        ))}
      </div>
    )
  if (rows.length === 0)
    return (
      <Empty className="min-h-80 border-0">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <BracesIcon />
          </EmptyMedia>
          <EmptyTitle>还没有任务</EmptyTitle>
          <EmptyDescription>
            创建第一个任务，让 Rooster 替你守着它。
          </EmptyDescription>
        </EmptyHeader>
        <EmptyContent>
          <Button onClick={props.onCreate}>创建任务</Button>
        </EmptyContent>
      </Empty>
    )

  return (
    <>
      <div className="divide-y sm:hidden">
        {rows.map((task) => (
          <div
            key={task.id}
            className={cn(
              "flex items-center gap-2 px-3 py-3",
              props.selectedId === task.id && "bg-accent/60"
            )}
            onClick={() => props.onSelect(task)}
          >
            <div
              className={cn(
                "flex size-9 shrink-0 items-center justify-center rounded-lg",
                task.kind === "resident"
                  ? "bg-primary/10 text-primary"
                  : "bg-schedule/10 text-schedule"
              )}
            >
              {task.kind === "resident" ? (
                <ServerCogIcon className="size-4" />
              ) : (
                <CalendarClockIcon className="size-4" />
              )}
            </div>
            <div className="min-w-0 flex-1">
              <TaskName task={task} />
              <div className="truncate font-mono text-xs text-muted-foreground">
                {task.command}
              </div>
            </div>
            <StatusBadge state={task.runtime.state} />
            <div onClick={(event) => event.stopPropagation()}>
              <Switch
                size="sm"
                aria-label={`${task.enabled ? "停用" : "启用"}${task.name}`}
                checked={task.enabled}
                disabled={busy === `toggle-${task.id}`}
                onCheckedChange={(checked) =>
                  void action(`toggle-${task.id}`, () =>
                    props.onToggle(task.id, checked)
                  )
                }
              />
            </div>
            <div
              className="flex shrink-0"
              onClick={(event) => event.stopPropagation()}
            >
              {task.runtime.state === "running" && task.runtime.executionId ? (
                <IconButton
                  label="停止"
                  onClick={() =>
                    void action(`run-${task.id}`, () =>
                      props.onStop(task.runtime.executionId!)
                    )
                  }
                >
                  <SquareIcon />
                </IconButton>
              ) : (
                <IconButton
                  label="运行一次"
                  onClick={() =>
                    void action(`run-${task.id}`, () => props.onRun(task.id))
                  }
                >
                  <PlayIcon />
                </IconButton>
              )}
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={<Button variant="ghost" size="icon-sm" />}
                >
                  <EllipsisIcon />
                  <span className="sr-only">更多操作</span>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuGroup>
                    <DropdownMenuItem onClick={() => props.onSelect(task)}>
                      <HistoryIcon />
                      执行记录
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      disabled={task.enabled}
                      onClick={() => props.onEdit(task)}
                    >
                      <PencilIcon />
                      编辑
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      variant="destructive"
                      disabled={task.enabled}
                      onClick={() => setPendingDelete(task)}
                    >
                      <Trash2Icon />
                      删除
                    </DropdownMenuItem>
                  </DropdownMenuGroup>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
        ))}
      </div>
      <div className="hidden sm:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>任务</TableHead>
              <TableHead className="hidden lg:table-cell">策略</TableHead>
              <TableHead>状态</TableHead>
              <TableHead className="w-24 text-center">自动</TableHead>
              <TableHead className="w-28 text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((task) => (
              <TableRow
                key={task.id}
                data-state={
                  props.selectedId === task.id ? "selected" : undefined
                }
                className="cursor-pointer"
                onClick={() => props.onSelect(task)}
              >
                <TableCell>
                  <div className="flex min-w-0 items-center gap-3">
                    <div
                      className={cn(
                        "flex size-9 shrink-0 items-center justify-center rounded-lg",
                        task.kind === "resident"
                          ? "bg-primary/10 text-primary"
                          : "bg-schedule/10 text-schedule"
                      )}
                    >
                      {task.kind === "resident" ? (
                        <ServerCogIcon className="size-4" />
                      ) : (
                        <CalendarClockIcon className="size-4" />
                      )}
                    </div>
                    <div className="min-w-0">
                      <TaskName task={task} />
                      <div className="max-w-72 truncate font-mono text-xs text-muted-foreground">
                        {task.command}
                      </div>
                    </div>
                  </div>
                </TableCell>
                <TableCell className="hidden lg:table-cell">
                  {task.kind === "scheduled" ? (
                    <SchedulePreview task={task} />
                  ) : (
                    <span className="text-xs text-muted-foreground">
                      {restartLabel(task.restartPolicy)} · {task.maxRetries} 次
                    </span>
                  )}
                </TableCell>
                <TableCell>
                  <StatusBadge state={task.runtime.state} />
                </TableCell>
                <TableCell
                  className="text-center"
                  onClick={(event) => event.stopPropagation()}
                >
                  <Switch
                    aria-label={`${task.enabled ? "停用" : "启用"}${task.name}`}
                    checked={task.enabled}
                    disabled={busy === `toggle-${task.id}`}
                    onCheckedChange={(checked) =>
                      void action(`toggle-${task.id}`, () =>
                        props.onToggle(task.id, checked)
                      )
                    }
                  />
                </TableCell>
                <TableCell onClick={(event) => event.stopPropagation()}>
                  <div className="flex justify-end gap-1">
                    {task.runtime.state === "running" &&
                    task.runtime.executionId ? (
                      <IconButton
                        label="停止"
                        onClick={() =>
                          void action(`run-${task.id}`, () =>
                            props.onStop(task.runtime.executionId!)
                          )
                        }
                        disabled={busy === `run-${task.id}`}
                      >
                        <SquareIcon />
                      </IconButton>
                    ) : (
                      <IconButton
                        label="运行一次"
                        onClick={() =>
                          void action(`run-${task.id}`, () =>
                            props.onRun(task.id)
                          )
                        }
                        disabled={busy === `run-${task.id}`}
                      >
                        <PlayIcon />
                      </IconButton>
                    )}
                    <DropdownMenu>
                      <DropdownMenuTrigger
                        render={<Button variant="ghost" size="icon-sm" />}
                      >
                        <EllipsisIcon />
                        <span className="sr-only">更多操作</span>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuGroup>
                          <DropdownMenuItem
                            onClick={() => props.onSelect(task)}
                          >
                            <HistoryIcon />
                            执行记录
                          </DropdownMenuItem>
                          <DropdownMenuItem
                            disabled={task.enabled}
                            onClick={() => props.onEdit(task)}
                          >
                            <PencilIcon />
                            编辑
                          </DropdownMenuItem>
                          <DropdownMenuItem
                            variant="destructive"
                            disabled={task.enabled}
                            onClick={() => setPendingDelete(task)}
                          >
                            <Trash2Icon />
                            删除
                          </DropdownMenuItem>
                        </DropdownMenuGroup>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <AlertDialog
        open={Boolean(pendingDelete)}
        onOpenChange={(open) => !open && setPendingDelete(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogMedia>
              <Trash2Icon />
            </AlertDialogMedia>
            <AlertDialogTitle>删除“{pendingDelete?.name}”？</AlertDialogTitle>
            <AlertDialogDescription>
              任务和执行记录会一并删除，日志文件暂时保留。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={() =>
                pendingDelete &&
                void action(`delete-${pendingDelete.id}`, async () => {
                  await props.onDelete(pendingDelete.id)
                  setPendingDelete(null)
                })
              }
            >
              删除任务
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

function IconButton({
  label,
  children,
  ...props
}: React.ComponentProps<typeof Button> & { label: string }) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={<Button variant="ghost" size="icon-sm" {...props} />}
      >
        {children}
        <span className="sr-only">{label}</span>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  )
}

function TaskName({ task }: { task: Task }) {
  if (!task.link)
    return <div className="truncate text-sm font-medium">{task.name}</div>

  return (
    <a
      className="flex items-center gap-1 truncate text-sm font-medium text-primary hover:underline"
      href={task.link}
      target="_blank"
      rel="noreferrer"
      title={`打开 ${task.name} 的配置链接`}
      onClick={(event) => event.stopPropagation()}
    >
      <span className="truncate">{task.name}</span>
      <ExternalLinkIcon className="size-3 shrink-0" />
    </a>
  )
}

const runTimeFormatter = new Intl.DateTimeFormat("zh-CN", {
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  weekday: "short",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
})

function SchedulePreview({ task }: { task: Task }) {
  const nextRuns = task.nextRuns ?? []
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            type="button"
            className="inline-flex items-center gap-2 text-left"
            onClick={(event) => event.stopPropagation()}
          />
        }
      >
        <Badge variant="secondary" className="font-mono">
          {task.schedule}
        </Badge>
      </TooltipTrigger>
      <TooltipContent
        side="bottom"
        align="start"
        className="w-72 max-w-72 flex-col items-stretch gap-2 px-3 py-2.5"
      >
        <div className="flex items-baseline justify-between gap-3">
          <span className="font-medium">未来 5 次触发时间</span>
          <span className="truncate text-[11px] opacity-70">{timezone}</span>
        </div>
        {!task.enabled ? (
          <p className="opacity-70">任务当前已停用，启用后才会执行。</p>
        ) : null}
        {nextRuns.length > 0 ? (
          <ol className="flex flex-col gap-1 font-mono tabular-nums">
            {nextRuns.map((run, index) => (
              <li key={run} className="flex items-center gap-2">
                <span className="w-4 shrink-0 text-right opacity-50">
                  {index + 1}
                </span>
                <time dateTime={run}>
                  {runTimeFormatter.format(new Date(run))}
                </time>
              </li>
            ))}
          </ol>
        ) : (
          <p>暂时无法计算后续触发时间。</p>
        )}
      </TooltipContent>
    </Tooltip>
  )
}

function restartLabel(policy: Task["restartPolicy"]) {
  return policy === "always"
    ? "始终重启"
    : policy === "never"
      ? "不重启"
      : "失败重启"
}
