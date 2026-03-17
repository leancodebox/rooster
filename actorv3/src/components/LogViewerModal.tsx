import { useEffect, useRef, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'

interface LogViewerModalProps {
  show: boolean
  job: any
  logInfo: any
  onClose: () => void
}

export default function LogViewerModal({ show, job, logInfo, onClose }: LogViewerModalProps) {
  const [isStreaming, setIsStreaming] = useState(false)
  const [connectionState, setConnectionState] = useState('Disconnected')
  const [autoScroll, setAutoScroll] = useState(true)
  const terminalContainerRef = useRef<HTMLDivElement>(null)
  
  const termRef = useRef<Terminal | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const evtRef = useRef<EventSource | null>(null)
  const watchdogTimerRef = useRef<any>(0)

  // 初始化终端
  useEffect(() => {
    if (!show || !terminalContainerRef.current) return

    // 如果终端已存在，先清理
    if (termRef.current) {
      termRef.current.dispose()
    }

    const term = new Terminal({
      cursorBlink: true,
      disableStdin: true,
      theme: {
        background: '#1e1e1e',
      },
      convertEol: true,
    })
    
    const fitAddon = new FitAddon()
    term.loadAddon(fitAddon)
    term.loadAddon(new WebLinksAddon())
    
    term.open(terminalContainerRef.current)
    fitAddon.fit()

    termRef.current = term
    fitAddonRef.current = fitAddon

    const handleResize = () => fitAddon.fit()
    window.addEventListener('resize', handleResize)

    // 开始流式传输
    setIsStreaming(true)

    return () => {
      window.removeEventListener('resize', handleResize)
      if (termRef.current) {
        termRef.current.dispose()
        termRef.current = null
      }
    }
  }, [show])

  // 处理流式传输逻辑
  useEffect(() => {
    if (!show || !isStreaming) {
      stopStreaming()
      return
    }

    startStreaming()

    return () => {
      stopStreaming()
    }
  }, [show, isStreaming, job])

  const scrollToBottom = () => {
    if (autoScroll && termRef.current) {
      termRef.current.scrollToBottom()
    }
  }

  const resetWatchdog = () => {
    clearTimeout(watchdogTimerRef.current)
    watchdogTimerRef.current = setTimeout(() => {
      console.log('Watchdog timeout, reconnecting...')
      setConnectionState('Reconnecting...')
      startStreaming()
    }, 15000)
  }

  const startStreaming = () => {
    setConnectionState('Connecting...')
    clearTimeout(watchdogTimerRef.current)
    if (evtRef.current) {
      evtRef.current.close()
      evtRef.current = null
    }

    const useFile = !!(job && job.options && job.options.outputPath)
    if (useFile) {
      if (termRef.current && connectionState !== 'Reconnecting...') {
        termRef.current.clear()
      }
      
      const evt = new EventSource(`/api/job-log-stream?jobId=${encodeURIComponent(job.uuid)}`)
      evtRef.current = evt

      evt.onopen = () => {
        setConnectionState('Connected')
        resetWatchdog()
      }
      
      evt.addEventListener('ping', () => {
         resetWatchdog()
      })
      
      evt.onmessage = (e) => {
        resetWatchdog()
        if (termRef.current) {
          termRef.current.write(e.data)
          scrollToBottom()
        }
      }
      
      evt.onerror = () => {
        setConnectionState('Reconnecting...')
        console.log('EventSource error, retrying...')
      }
    }
  }

  const stopStreaming = () => {
    setConnectionState('Disconnected')
    clearTimeout(watchdogTimerRef.current)
    if (evtRef.current) {
      evtRef.current.close()
      evtRef.current = null
    }
  }

  if (!show) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white rounded-lg p-6 shadow-xl w-11/12 max-w-6xl h-[90vh] flex flex-col relative">
        <button 
          className="absolute right-4 top-4 text-gray-500 hover:text-gray-700 hover:bg-gray-100 p-2 rounded-full transition-colors" 
          onClick={onClose} 
          title="关闭" 
          aria-label="关闭"
        >
          <i className="fa-solid fa-xmark text-lg"></i>
        </button>
        <h3 className="font-bold text-lg mb-2">日志查看</h3>
        <div className="py-2 text-sm text-gray-600">
          路径: {logInfo.realLogPath || '-'} | 大小: {logInfo.size} | 更新时间: {logInfo.modTime || '-'}
        </div>
        <div className="flex items-center gap-4 mb-4">
          <div className={`px-2.5 py-0.5 rounded-full text-xs font-medium border ${
            connectionState === 'Connected' ? 'bg-green-50 text-green-700 border-green-200' :
            connectionState === 'Reconnecting...' ? 'bg-yellow-50 text-yellow-700 border-yellow-200' :
            'bg-gray-50 text-gray-700 border-gray-200'
          }`}>
            {connectionState}
          </div>
          <label className="flex items-center gap-2 cursor-pointer text-sm font-medium text-gray-700">
            <span>实时滚动</span>
            <input 
              type="checkbox" 
              className="w-9 h-5 bg-gray-200 rounded-full appearance-none relative checked:bg-blue-600 cursor-pointer after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all checked:after:translate-x-4" 
              checked={isStreaming}
              onChange={(e) => setIsStreaming(e.target.checked)}
            />
          </label>
          <label className="flex items-center gap-2 cursor-pointer text-sm font-medium text-gray-700">
            <span>自动滚动</span>
            <input 
              type="checkbox" 
              className="w-9 h-5 bg-gray-200 rounded-full appearance-none relative checked:bg-blue-600 cursor-pointer after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all checked:after:translate-x-4" 
              checked={autoScroll}
              onChange={(e) => setAutoScroll(e.target.checked)}
            />
          </label>
        </div>
        <div ref={terminalContainerRef} className="flex-1 bg-[#1e1e1e] rounded overflow-hidden"></div>
      </div>
    </div>
  )
}
