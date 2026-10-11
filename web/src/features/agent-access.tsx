import { useState } from "react"
import { BotIcon, CopyIcon, ExternalLinkIcon, LinkIcon } from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Textarea } from "@/components/ui/textarea"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"

const documents = [
  { name: "Skill", path: "/api/skill" },
  { name: "API 详情", path: "/api/docs" },
]

export function AgentAccess() {
  const [busy, setBusy] = useState(false)
  const [manualCopy, setManualCopy] = useState("")
  const baseURL = window.location.origin

  async function copy(text: string) {
    try {
      await navigator.clipboard.writeText(text)
      setManualCopy("")
      toast.success("已复制")
    } catch {
      setManualCopy(text)
      toast.error("剪贴板不可用，请从下方文本框复制")
    }
  }

  async function copyDocument(path: string) {
    setBusy(true)
    try {
      const response = await fetch(path)
      if (
        !response.ok ||
        !response.headers.get("Content-Type")?.includes("text/markdown")
      ) {
        throw new Error("文档读取失败，请确认 Rooster 已更新并重新启动")
      }
      await copy(await response.text())
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "文档读取失败")
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog onOpenChange={() => setManualCopy("")}>
      <Tooltip>
        <TooltipTrigger
          render={
            <DialogTrigger
              render={
                <Button variant="outline" size="icon" aria-label="Agent 接入" />
              }
            />
          }
        >
          <BotIcon />
        </TooltipTrigger>
        <TooltipContent>Agent 接入</TooltipContent>
      </Tooltip>
      <DialogContent className="max-h-[calc(100svh-2rem)] overflow-y-auto rounded-lg sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Agent 接入</DialogTitle>
        </DialogHeader>
        <div className="divide-y">
          {documents.map((document) => (
            <div
              key={document.path}
              className="flex items-center justify-between gap-3 py-3"
            >
              <div className="min-w-0">
                <p className="font-medium">{document.name}</p>
                <p className="font-mono text-xs break-all text-muted-foreground">
                  {baseURL}
                  {document.path}
                </p>
              </div>
              <div className="flex shrink-0 gap-1">
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        variant="ghost"
                        size="icon"
                        render={
                          <a
                            href={document.path}
                            target="_blank"
                            rel="noopener noreferrer"
                          />
                        }
                        aria-label={`打开 ${document.name}`}
                      />
                    }
                  >
                    <ExternalLinkIcon />
                  </TooltipTrigger>
                  <TooltipContent>打开 {document.name}</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => void copy(`${baseURL}${document.path}`)}
                        aria-label={`复制 ${document.name} 链接`}
                      />
                    }
                  >
                    <LinkIcon />
                  </TooltipTrigger>
                  <TooltipContent>复制链接</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        variant="ghost"
                        size="icon"
                        disabled={busy}
                        onClick={() => void copyDocument(document.path)}
                        aria-label={`复制 ${document.name} 全文`}
                      />
                    }
                  >
                    <CopyIcon />
                  </TooltipTrigger>
                  <TooltipContent>复制全文</TooltipContent>
                </Tooltip>
              </div>
            </div>
          ))}
        </div>
        <Button
          variant="outline"
          onClick={() =>
            void copy(
              `通过 HTTP API 操作本地 Rooster。服务地址：${baseURL}\n请先读取 Skill：${baseURL}/api/skill\n再读取 API 详情：${baseURL}/api/docs\n健康检查：GET ${baseURL}/api/health\n根据我的要求调用接口，并检查执行结果。`
            )
          }
        >
          <CopyIcon data-icon="inline-start" />
          复制 Agent 接入信息
        </Button>
        {manualCopy ? (
          <Textarea
            aria-label="待复制内容"
            className="h-40 font-mono text-xs"
            readOnly
            value={manualCopy}
            onFocus={(event) => event.currentTarget.select()}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
