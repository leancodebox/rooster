import {
  CircleCheckIcon,
  CircleDashedIcon,
  CircleStopIcon,
  LoaderCircleIcon,
} from "lucide-react"
import { Badge } from "@/components/ui/badge"
import type { RuntimeState } from "./types"

const labels: Record<RuntimeState, string> = {
  idle: "空闲",
  running: "运行中",
  starting: "启动中",
  stopping: "停止中",
  backoff: "等待重启",
  failed: "失败",
}

export function StatusBadge({ state }: { state: RuntimeState }) {
  if (state === "running")
    return (
      <Badge className="bg-status text-status-foreground">
        <CircleCheckIcon data-icon="inline-start" />
        {labels[state]}
      </Badge>
    )
  if (state === "starting" || state === "stopping" || state === "backoff")
    return (
      <Badge variant="secondary">
        <LoaderCircleIcon className="animate-spin" data-icon="inline-start" />
        {labels[state]}
      </Badge>
    )
  if (state === "failed")
    return (
      <Badge variant="destructive">
        <CircleStopIcon data-icon="inline-start" />
        {labels[state]}
      </Badge>
    )
  return (
    <Badge variant="outline">
      <CircleDashedIcon data-icon="inline-start" />
      {labels[state]}
    </Badge>
  )
}
