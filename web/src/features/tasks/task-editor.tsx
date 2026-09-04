import { useState, type FormEvent } from "react"
import { SaveIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Textarea } from "@/components/ui/textarea"
import type { Task, TaskDraft } from "./types"
import { emptyTask } from "./types"

interface TaskEditorProps {
  open: boolean
  task: Task | null
  initialKind: "resident" | "scheduled"
  onOpenChange: (open: boolean) => void
  onCreate: (task: TaskDraft) => Promise<unknown>
  onUpdate: (task: Task) => Promise<unknown>
}

const kindItems = [
  { label: "常驻任务", value: "resident" },
  { label: "定时任务", value: "scheduled" },
]
const modeItems = [
  { label: "Shell 命令", value: "shell" },
  { label: "直接执行", value: "exec" },
]
const restartItems = [
  { label: "失败时重启", value: "on_failure" },
  { label: "始终重启", value: "always" },
  { label: "不重启", value: "never" },
]
const overlapItems = [
  { label: "跳过本次", value: "skip" },
  { label: "允许并行", value: "parallel" },
]

export function TaskEditor({
  open,
  task,
  initialKind,
  onOpenChange,
  onCreate,
  onUpdate,
}: TaskEditorProps) {
  const initial = task ? stripRuntime(task) : emptyTask(initialKind)
  const [draft, setDraft] = useState<TaskDraft>(initial)
  const [environment, setEnvironment] = useState(() =>
    formatEnvironment(initial.environment)
  )
  const [saving, setSaving] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (!draft.name.trim() || !draft.command.trim()) return
    setSaving(true)
    try {
      const value = { ...draft, environment: parseEnvironment(environment) }
      if (task) await onUpdate({ ...task, ...value })
      else await onCreate(value)
      onOpenChange(false)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full sm:max-w-xl">
        <form className="flex min-h-0 flex-1 flex-col" onSubmit={submit}>
          <SheetHeader>
            <SheetTitle>{task ? "编辑任务" : "创建任务"}</SheetTitle>
            <SheetDescription>
              配置执行方式和自动运行策略。保存后可随时启停。
            </SheetDescription>
          </SheetHeader>
          <div className="min-h-0 flex-1 overflow-y-auto px-4 pb-6">
            <FieldGroup>
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <Field>
                  <FieldLabel htmlFor="task-name">名称</FieldLabel>
                  <Input
                    id="task-name"
                    required
                    value={draft.name}
                    onChange={(event) =>
                      setDraft({ ...draft, name: event.target.value })
                    }
                    placeholder="例如 API 服务"
                  />
                </Field>
                <Field>
                  <FieldLabel>类型</FieldLabel>
                  <Select
                    items={kindItems}
                    value={draft.kind}
                    onValueChange={(value) =>
                      value &&
                      setDraft({
                        ...draft,
                        kind: value as TaskDraft["kind"],
                        schedule:
                          value === "scheduled"
                            ? draft.schedule || "0 * * * *"
                            : "",
                      })
                    }
                    disabled={Boolean(task)}
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectGroup>
                        {kindItems.map((item) => (
                          <SelectItem key={item.value} value={item.value}>
                            {item.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </Field>
              </div>
              <Field>
                <FieldLabel htmlFor="task-description">说明</FieldLabel>
                <Input
                  id="task-description"
                  value={draft.description}
                  onChange={(event) =>
                    setDraft({ ...draft, description: event.target.value })
                  }
                  placeholder="这个任务负责什么"
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="task-link">配置链接</FieldLabel>
                <Input
                  id="task-link"
                  type="url"
                  value={draft.link}
                  onChange={(event) =>
                    setDraft({ ...draft, link: event.target.value })
                  }
                  placeholder="例如 http://127.0.0.1:3000"
                />
                <FieldDescription>
                  填写后可在任务列表点击任务名打开对应页面。
                </FieldDescription>
              </Field>
              <Field>
                <FieldLabel>执行方式</FieldLabel>
                <Select
                  items={modeItems}
                  value={draft.commandMode}
                  onValueChange={(value) =>
                    value &&
                    setDraft({
                      ...draft,
                      commandMode: value as TaskDraft["commandMode"],
                    })
                  }
                >
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {modeItems.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FieldDescription>
                  {draft.commandMode === "shell"
                    ? "加载用户环境，支持管道、重定向和完整 Shell 语法。"
                    : "直接启动可执行文件，不经过 Shell。"}
                </FieldDescription>
              </Field>
              <Field>
                <FieldLabel htmlFor="task-command">命令</FieldLabel>
                <Textarea
                  id="task-command"
                  required
                  rows={3}
                  value={draft.command}
                  onChange={(event) =>
                    setDraft({ ...draft, command: event.target.value })
                  }
                  placeholder={
                    draft.commandMode === "shell"
                      ? "pnpm start"
                      : "/usr/local/bin/node"
                  }
                />
              </Field>
              {draft.commandMode === "exec" ? (
                <Field>
                  <FieldLabel htmlFor="task-arguments">参数</FieldLabel>
                  <Textarea
                    id="task-arguments"
                    rows={3}
                    value={draft.arguments.join("\n")}
                    onChange={(event) =>
                      setDraft({
                        ...draft,
                        arguments: splitArguments(event.target.value),
                      })
                    }
                    placeholder={"server.js\n--port\n3000"}
                  />
                  <FieldDescription>
                    每行一个参数，不进行 Shell 转义或拆词。
                  </FieldDescription>
                </Field>
              ) : null}
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <Field>
                  <FieldLabel htmlFor="task-directory">工作目录</FieldLabel>
                  <Input
                    id="task-directory"
                    value={draft.workingDir}
                    onChange={(event) =>
                      setDraft({ ...draft, workingDir: event.target.value })
                    }
                    placeholder="留空使用 Rooster 目录"
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="task-shell">Shell</FieldLabel>
                  <Input
                    id="task-shell"
                    value={draft.shell}
                    onChange={(event) =>
                      setDraft({ ...draft, shell: event.target.value })
                    }
                    placeholder="自动检测"
                    disabled={draft.commandMode === "exec"}
                  />
                </Field>
              </div>
              <Field>
                <FieldLabel htmlFor="task-environment">任务环境变量</FieldLabel>
                <Textarea
                  id="task-environment"
                  rows={3}
                  value={environment}
                  onChange={(event) => setEnvironment(event.target.value)}
                  placeholder={"NODE_ENV=production\nPORT=3000"}
                />
                <FieldDescription>
                  每行一个 KEY=value，将覆盖登录 Shell 中的同名变量。
                </FieldDescription>
              </Field>
              {draft.kind === "scheduled" ? (
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <Field>
                    <FieldLabel htmlFor="task-schedule">Cron</FieldLabel>
                    <Input
                      id="task-schedule"
                      required
                      value={draft.schedule}
                      onChange={(event) =>
                        setDraft({ ...draft, schedule: event.target.value })
                      }
                    />
                  </Field>
                  <Field>
                    <FieldLabel>重叠执行</FieldLabel>
                    <Select
                      items={overlapItems}
                      value={draft.overlapPolicy}
                      onValueChange={(value) =>
                        value &&
                        setDraft({
                          ...draft,
                          overlapPolicy: value as TaskDraft["overlapPolicy"],
                        })
                      }
                    >
                      <SelectTrigger className="w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectGroup>
                          {overlapItems.map((item) => (
                            <SelectItem key={item.value} value={item.value}>
                              {item.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </Field>
                </div>
              ) : (
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                  <Field>
                    <FieldLabel>重启策略</FieldLabel>
                    <Select
                      items={restartItems}
                      value={draft.restartPolicy}
                      onValueChange={(value) =>
                        value &&
                        setDraft({
                          ...draft,
                          restartPolicy: value as TaskDraft["restartPolicy"],
                        })
                      }
                    >
                      <SelectTrigger className="w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectGroup>
                          {restartItems.map((item) => (
                            <SelectItem key={item.value} value={item.value}>
                              {item.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="task-retries">失败上限</FieldLabel>
                    <Input
                      id="task-retries"
                      type="number"
                      min={1}
                      value={draft.maxRetries}
                      onChange={(event) =>
                        setDraft({
                          ...draft,
                          maxRetries: Number(event.target.value),
                        })
                      }
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="task-healthy">健康时长</FieldLabel>
                    <Input
                      id="task-healthy"
                      type="number"
                      min={0}
                      value={draft.minRunSeconds}
                      onChange={(event) =>
                        setDraft({
                          ...draft,
                          minRunSeconds: Number(event.target.value),
                        })
                      }
                    />
                  </Field>
                </div>
              )}
            </FieldGroup>
          </div>
          <SheetFooter>
            <Button
              type="submit"
              disabled={saving || !draft.name.trim() || !draft.command.trim()}
            >
              <SaveIcon data-icon="inline-start" />
              {saving ? "保存中" : "保存任务"}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  )
}

function stripRuntime(task: Task): TaskDraft {
  return {
    name: task.name,
    description: task.description,
    link: task.link,
    kind: task.kind,
    enabled: task.enabled,
    commandMode: task.commandMode,
    command: task.command,
    arguments: task.arguments,
    workingDir: task.workingDir,
    shell: task.shell,
    environment: task.environment,
    schedule: task.schedule,
    overlapPolicy: task.overlapPolicy,
    restartPolicy: task.restartPolicy,
    maxRetries: task.maxRetries,
    minRunSeconds: task.minRunSeconds,
  }
}

function formatEnvironment(environment: Record<string, string>) {
  return Object.entries(environment)
    .map(([key, value]) => `${key}=${value}`)
    .join("\n")
}
function parseEnvironment(value: string) {
  return Object.fromEntries(
    value
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => {
        const index = line.indexOf("=")
        return index < 1
          ? [line, ""]
          : [line.slice(0, index), line.slice(index + 1)]
      })
  )
}
function splitArguments(value: string) {
  return value
    .split("\n")
    .map((item) => item.trim())
    .filter(Boolean)
}
