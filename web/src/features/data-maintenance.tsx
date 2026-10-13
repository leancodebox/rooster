import { useState } from "react"
import {
  DatabaseIcon,
  DownloadIcon,
  SearchIcon,
  Trash2Icon,
} from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from "@/components/ui/alert-dialog"
import {
  Field,
  FieldGroup,
  FieldLabel,
  FieldDescription,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip"
import { api, type RetentionResult } from "./tasks/api"
import { messageOf } from "./tasks/use-tasks"

export function DataMaintenance() {
  const [days, setDays] = useState(30)
  const [keep, setKeep] = useState(100)
  const [result, setResult] = useState<RetentionResult | null>(null)
  const [busy, setBusy] = useState(false)
  const [confirm, setConfirm] = useState(false)
  const valid =
    Number.isInteger(days) &&
    days >= 1 &&
    days <= 36500 &&
    Number.isInteger(keep) &&
    keep >= 1

  async function retention(dryRun: boolean) {
    if (busy || !valid || (!dryRun && !result?.dryRun)) return
    setBusy(true)
    try {
      const next = await api.retention(days, keep, dryRun)
      setResult(next)
      if (!dryRun) {
        setConfirm(false)
        if (next.errors.length) toast.error("清理部分完成，请检查错误记录")
        else toast.success(`已清理 ${next.deleted} 条执行记录`)
      }
    } catch (error) {
      toast.error(messageOf(error))
      setResult(null)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      onOpenChange={() => {
        if (!busy) {
          setResult(null)
          setConfirm(false)
        }
      }}
    >
      <Tooltip>
        <TooltipTrigger
          render={
            <DialogTrigger
              render={
                <Button variant="outline" size="icon" aria-label="数据维护" />
              }
            />
          }
        >
          <DatabaseIcon />
        </TooltipTrigger>
        <TooltipContent>数据维护</TooltipContent>
      </Tooltip>
      <DialogContent className="max-h-[calc(100svh-2rem)] overflow-y-auto rounded-lg sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>数据维护</DialogTitle>
        </DialogHeader>
        <section className="flex flex-col gap-3">
          <h3 className="text-sm font-medium">备份与导出</h3>
          <p className="text-sm text-muted-foreground">
            配置和数据库包含任务环境变量，请妥善保管。数据库备份不包含日志文件。
          </p>
          <div className="flex flex-wrap gap-2">
            <Button
              variant="outline"
              nativeButton={false}
              render={<a href="/api/export" download />}
            >
              <DownloadIcon data-icon="inline-start" />
              导出任务配置
            </Button>
            <Button
              variant="outline"
              nativeButton={false}
              render={<a href="/api/backup" download />}
            >
              <DownloadIcon data-icon="inline-start" />
              下载数据库备份
            </Button>
          </div>
        </section>
        <Separator />
        <section className="flex flex-col gap-3">
          <h3 className="text-sm font-medium">历史与日志清理</h3>
          <FieldGroup className="grid gap-3 sm:grid-cols-2">
            <Field>
              <FieldLabel htmlFor="retention-days">保留天数</FieldLabel>
              <Input
                id="retention-days"
                type="number"
                min={1}
                max={36500}
                value={days}
                disabled={busy || confirm}
                onChange={(event) => {
                  setDays(Number(event.target.value))
                  setResult(null)
                }}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="retention-keep">
                每个任务至少保留次数
              </FieldLabel>
              <Input
                id="retention-keep"
                type="number"
                min={1}
                value={keep}
                disabled={busy || confirm}
                onChange={(event) => {
                  setKeep(Number(event.target.value))
                  setResult(null)
                }}
              />
              <FieldDescription>
                只清理已结束的旧执行及对应日志。
              </FieldDescription>
            </Field>
          </FieldGroup>
          <div className="flex flex-wrap gap-2">
            <Button
              variant="outline"
              disabled={busy || !valid}
              onClick={() => void retention(true)}
            >
              <SearchIcon data-icon="inline-start" />
              {busy ? "处理中" : "预览清理"}
            </Button>
            <Button
              variant="destructive"
              disabled={
                busy ||
                !result?.dryRun ||
                result.candidates === 0 ||
                result.errors.length > 0
              }
              onClick={() => setConfirm(true)}
            >
              <Trash2Icon data-icon="inline-start" />
              执行清理
            </Button>
          </div>
          {result ? (
            <div role="status" className="flex flex-col gap-2 text-sm">
              <p>
                {result.dryRun ? "待清理" : "已清理"}：
                {result.dryRun ? result.candidates : result.deleted} 条记录
              </p>
              <p>
                文件：{result.files} 个 ·{" "}
                {result.dryRun ? "预计释放" : "计划清理"}{" "}
                {(result.bytes / 1024 / 1024).toFixed(2)} MiB
              </p>
              {result.errors.length ? (
                <ul className="max-h-40 overflow-auto break-all text-destructive">
                  {result.errors.map((error, index) => (
                    <li key={index}>{error}</li>
                  ))}
                </ul>
              ) : null}
            </div>
          ) : null}
        </section>
        <AlertDialog
          open={confirm}
          onOpenChange={(open) => {
            if (!busy) setConfirm(open)
          }}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>确认清理历史与日志？</AlertDialogTitle>
              <AlertDialogDescription>
                将按当前条件重新计算并删除过期执行及日志，无法撤销。请先备份；执行时的数量可能与预览略有差异。
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={busy}>取消</AlertDialogCancel>
              <AlertDialogAction
                variant="destructive"
                disabled={busy}
                onClick={() => void retention(false)}
              >
                {busy ? "清理中" : "确认删除"}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </DialogContent>
    </Dialog>
  )
}
