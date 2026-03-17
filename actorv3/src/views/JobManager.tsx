import { useEffect, useState, useRef } from 'react'
import { useScrollLock } from '../hooks/useScrollLock'
import {
  getHomePath,
  getJobList,
  removeTask,
  runInfo,
  runJob,
  runOpenCloseTask,
  runTask,
  saveTask,
  stopJob
} from '../request/remote'

import LogViewerModal from '../components/LogViewerModal'
import TaskEditModal from '../components/TaskEditModal'

export default function JobManager() {
  const [, setData] = useState<any[]>([])
  const [resident, setResident] = useState<any[]>([])
  const [scheduled, setScheduled] = useState<any[]>([])
  const [showModal, setShowModal] = useState(false)
  const [showLogModal, setShowLogModal] = useState(false)
  const [showDeleteModal, setShowDeleteModal] = useState(false)
  const [pendingDeleteUuid, setPendingDeleteUuid] = useState('')
  const [logInfo, setLogInfo] = useState<any>({ realLogPath: '', size: 0, modTime: '', uuid: '' })
  const [currentJob, setCurrentJob] = useState<any>(null)
  const [appRunTime, setAppRunTime] = useState({ start: '', runTime: '' })
  const [defaultLogDir, setDefaultLogDir] = useState('')
  const [model, setModel] = useState(getInitData(1))
  const [runtimeDigits, setRuntimeDigits] = useState({ day: 0, hour: 0, minute: 0, second: 0 })
  const [toast, setToast] = useState({ show: false, message: '' })

  const timerRef = useRef<any>(0)

  // 监听模态框状态，防止背景滚动穿透
  useScrollLock(showModal || showLogModal || showDeleteModal)

  function showToast(msg: string) {
    setToast({ show: true, message: msg })
    setTimeout(() => {
      setToast((prev) => ({ ...prev, show: false }))
    }, 600)
  }

  function calcDigits(ms: number) {
    const s = Math.max(0, Math.floor(ms / 1000))
    const day = Math.floor(s / 86400)
    const hour = Math.floor((s % 86400) / 3600)
    const minute = Math.floor((s % 3600) / 60)
    const second = s % 60
    return { day, hour, minute, second }
  }

  function parseStart(s: string) {
    const p = s.split(/[- :]/).map((x) => parseInt(x, 10))
    if (p.length >= 6 && p.every((x) => !isNaN(x))) {
      const [Y, M, D, h, m, s2] = p as [number, number, number, number, number, number]
      return new Date(Y, M - 1, D, h, m, s2)
    }
    const t = s.replace(' ', 'T')
    const d = new Date(t)
    return isNaN(d.getTime()) ? new Date() : d
  }

  function getInitData(type: number) {
    return {
      uuid: '', jobName: '', link: '', type, spec: '* * * * *', binPath: '', dir: '', run: false,
      options: { maxFailures: 5, outputPath: defaultLogDir || '/tmp' }, edit: false, readonly: false
    }
  }

  function add(type: number) {
    setModel(getInitData(type))
    setShowModal(true)
  }

  async function onPositiveClick() {
    await saveTask(model)
    await refresh()
    setShowModal(false)
  }

  function edit(row: any) {
    setModel({ ...row, edit: true, readonly: row.run === true })
    setShowModal(true)
  }

  async function viewLog(row: any) {
    setLogInfo(row || { realLogPath: '' })
    setCurrentJob(row)
    setShowLogModal(true)
  }

  async function refresh() {
    const resp = await getJobList()
    const msg = resp.data?.message || []
    setData(msg)
    setResident(msg.filter((x: any) => x.type === 1))
    setScheduled(msg.filter((x: any) => x.type === 2))
  }

  async function onRefreshClick() {
    await refresh()
    showToast('刷新完毕')
  }

  function onRemove(uuid: string) {
    setPendingDeleteUuid(uuid)
    setShowDeleteModal(true)
  }

  async function confirmDelete() {
    if (pendingDeleteUuid) {
      await removeTask(pendingDeleteUuid)
      await refresh()
      setShowDeleteModal(false)
      setPendingDeleteUuid('')
    }
  }

  function confirmStatus(jobId: string, check: (row: any) => boolean, retries = 10, interval = 300) {
    let count = 0
    const h = setInterval(async () => {
      await refresh()
      // 注意：这里 data 是闭包中的旧值，或者需要用函数式更新/直接获取最新
      // 在 React 中直接调用 refresh() 会更新 state，但这里 setInterval 里的 data 可能是旧的
      // 更好的方式是重新请求或依赖 effect。
      // 为简化，这里再次请求列表
      const resp = await getJobList()
      const currentList = resp.data.message
      const row = currentList.find((x: any) => x.uuid === jobId)
      if (row && check(row)) {
        clearInterval(h)
      } else if (++count >= retries) {
        clearInterval(h)
      }
    }, interval)
  }

  async function onStopResident(jobId: string) {
    await stopJob(jobId)
    await refresh()
    confirmStatus(jobId, (row) => row.status === 0)
  }

  async function onStartResident(jobId: string) {
    await runJob(jobId)
    await refresh()
    confirmStatus(jobId, (row) => row.status === 1)
  }

  async function openScheduled(jobId: string) {
    await runOpenCloseTask(jobId, true)
    await refresh()
  }

  async function closeScheduled(jobId: string) {
    await runOpenCloseTask(jobId, false)
    await refresh()
  }

  useEffect(() => {
    refresh()
    getHomePath().then(r => {
      const h = r.data.home || ''
      setDefaultLogDir(h ? `${h}/.roosterTaskConfig/log` : '')
    }).catch(() => {})

    runInfo().then(info => {
      const startStr = info.data.start
      const startAt = parseStart(startStr)
      setAppRunTime({ runTime: '', start: startStr })
      setRuntimeDigits(calcDigits(Date.now() - startAt.getTime()))
      timerRef.current = setInterval(() => {
        const diff = Date.now() - startAt.getTime()
        setRuntimeDigits(calcDigits(diff))
      }, 1000)
    }).catch(() => {})

    return () => {
      clearInterval(timerRef.current)
    }
  }, [])

  return (
    <div className="max-w-7xl mx-auto px-4 md:px-6 lg:px-8 py-6 space-y-6 font-mono bg-white min-h-screen">
      <div className="flex items-center justify-between border-b border-gray-100 pb-4">
        <h1 className="text-2xl font-bold text-gray-900 tracking-tight">Task Manager</h1>
        <div className="flex items-center gap-4">
          <div className="flex gap-2">
            <button 
              className="flex items-center justify-center w-8 h-8 rounded-full bg-indigo-50 text-indigo-600 hover:bg-indigo-100 transition-colors" 
              onClick={() => add(1)} 
              title="新增常驻任务"
            >
              <i className="fa-solid fa-plus text-sm"></i>
            </button>
            <button 
              className="flex items-center justify-center w-8 h-8 rounded-full bg-emerald-50 text-emerald-600 hover:bg-emerald-100 transition-colors" 
              onClick={() => add(2)} 
              title="新增定时任务"
            >
              <i className="fa-solid fa-calendar-plus text-sm"></i>
            </button>
            <button 
              className="flex items-center justify-center w-8 h-8 rounded-full bg-gray-50 text-gray-600 hover:bg-gray-100 transition-colors" 
              onClick={onRefreshClick} 
              title="刷新列表"
            >
              <i className="fa-solid fa-arrows-rotate text-sm"></i>
            </button>
          </div>
          {appRunTime.start && (
            <div className="hidden sm:block text-xs font-medium text-gray-500 bg-gray-50 px-3 py-1.5 rounded-full border border-gray-100">
              <div className="grid grid-flow-col gap-3">
                <div className="flex items-baseline gap-1">
                  <span className="font-mono text-gray-900">{runtimeDigits.day}</span>
                  <span>天</span>
                </div>
                <div className="flex items-baseline gap-1">
                  <span className="font-mono text-gray-900">{String(runtimeDigits.hour).padStart(2, '0')}</span>
                  <span>时</span>
                </div>
                <div className="flex items-baseline gap-1">
                  <span className="font-mono text-gray-900">{String(runtimeDigits.minute).padStart(2, '0')}</span>
                  <span>分</span>
                </div>
                <div className="flex items-baseline gap-1">
                  <span className="font-mono text-gray-900 w-[18px] text-right">{String(runtimeDigits.second).padStart(2, '0')}</span>
                  <span>秒</span>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div>
          <h2 className="text-lg font-semibold mb-2">常驻任务</h2>
          <div className="overflow-x-auto border border-gray-200 rounded-lg">
            <table className="w-full text-left border-collapse text-sm">
              <thead className="bg-gray-50 border-b border-gray-200">
                <tr>
                  <th className="px-4 py-3 font-semibold text-gray-700">JobName</th>
                  <th className="px-4 py-3 font-semibold text-gray-700 text-center">跟随启动</th>
                  <th className="px-4 py-3 font-semibold text-gray-700 text-center">运行状态</th>
                  <th className="px-4 py-3 font-semibold text-gray-700">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {resident.map((row) => (
                  <tr key={row.uuid} className="hover:bg-gray-50/50">
                    <td className="px-4 py-3 truncate max-w-[14rem] sm:max-w-[18rem]">
                      {row.link ? (
                        <a href={row.link} className="text-blue-600 hover:text-blue-800 font-mono" target="_blank">{row.jobName}</a>
                      ) : (
                        <span>{row.jobName}</span>
                      )}
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap">
                      <div className="flex items-center justify-center gap-2">
                        <div className={row.run ? 'w-3 h-3 rounded-full bg-green-500' : 'w-3 h-3 rounded-full bg-gray-300'} title={row.run ? '开启' : '关闭'}></div>
                      </div>
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap">
                      <div className="flex items-center justify-center gap-2">
                        <div className={row.status === 1 ? 'w-3 h-3 rounded-full bg-green-500' : 'w-3 h-3 rounded-full bg-yellow-500'} title={row.status === 1 ? '运行' : '暂停'}></div>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex -space-x-px">
                        <button className="px-2.5 py-1.5 text-gray-700 bg-white border border-gray-300 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed first:rounded-l-md last:rounded-r-md transition-colors" onClick={() => onStopResident(row.uuid)} title="停止" aria-label="停止">
                          <i className="fa-solid fa-stop text-sm"></i></button>
                        <button className="px-2.5 py-1.5 text-white bg-blue-600 border border-blue-600 hover:bg-blue-700 disabled:bg-blue-400 disabled:border-blue-400 disabled:cursor-not-allowed first:rounded-l-md last:rounded-r-md transition-colors" disabled={row.status === 1}
                          onClick={() => onStartResident(row.uuid)} title="启动" aria-label="启动"><i
                            className="fa-solid fa-play text-sm"></i></button>
                        <button className="px-2.5 py-1.5 text-gray-700 bg-white border border-gray-300 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed first:rounded-l-md last:rounded-r-md transition-colors" onClick={() => edit(row)} title="编辑" aria-label="编辑"><i
                          className="fa-solid fa-pen-to-square text-sm"></i></button>
                        <button className="px-2.5 py-1.5 text-gray-700 bg-white border border-gray-300 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed first:rounded-l-md last:rounded-r-md transition-colors" disabled={row.run === true}
                          onClick={() => onRemove(row.uuid)} title="删除" aria-label="删除"><i
                            className="fa-solid fa-trash text-sm"></i></button>
                        <button className="px-2.5 py-1.5 text-gray-700 bg-white border border-gray-300 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed first:rounded-l-md last:rounded-r-md transition-colors" disabled={!row.realLogPath} onClick={() => viewLog(row)}
                          title={row.realLogPath ? '查看日志' : '日志(未开启)'}
                          aria-label="查看日志"><i className="fa-regular fa-file-lines text-sm"></i></button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
        <div>
          <h2 className="text-lg font-semibold mb-2">定时任务</h2>
          <div className="overflow-x-auto border border-gray-200 rounded-lg">
            <table className="w-full text-left border-collapse text-sm">
              <thead className="bg-gray-50 border-b border-gray-200">
                <tr>
                  <th className="px-4 py-3 font-semibold text-gray-700">JobName</th>
                  <th className="px-4 py-3 font-semibold text-gray-700 text-center">已开启</th>
                  <th className="px-4 py-3 font-semibold text-gray-700">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {scheduled.map((row) => (
                  <tr key={row.uuid} className="hover:bg-gray-50/50">
                    <td className="px-4 py-3 truncate max-w-[14rem] sm:max-w-[18rem]">{row.jobName}</td>
                    <td className="px-4 py-3 whitespace-nowrap">
                      <div className="flex items-center justify-center gap-2">
                        <div className={row.run ? 'w-3 h-3 rounded-full bg-green-500' : 'w-3 h-3 rounded-full bg-gray-300'} title={row.run ? '开启' : '关闭'}></div>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <input type="checkbox" className="w-9 h-5 bg-gray-200 rounded-full appearance-none relative checked:bg-blue-600 cursor-pointer after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all checked:after:translate-x-4" checked={row.run}
                          onChange={(e) => e.target.checked ? openScheduled(row.uuid) : closeScheduled(row.uuid)}
                          title={row.run ? '关闭定时' : '开启定时'} aria-label={row.run ? '关闭定时' : '开启定时'} />
                        <div className="flex -space-x-px">
                          <button className="px-2.5 py-1.5 text-gray-700 bg-white border border-gray-300 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed first:rounded-l-md last:rounded-r-md transition-colors" onClick={() => runTask(row.uuid).then(refresh)} title="运行一次"
                            aria-label="运行一次"><i className="fa-solid fa-play text-sm"></i></button>
                          <button className="px-2.5 py-1.5 text-gray-700 bg-white border border-gray-300 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed first:rounded-l-md last:rounded-r-md transition-colors" onClick={() => edit(row)} title="编辑" aria-label="编辑"><i
                            className="fa-solid fa-pen-to-square text-sm"></i></button>
                          <button className="px-2.5 py-1.5 text-gray-700 bg-white border border-gray-300 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed first:rounded-l-md last:rounded-r-md transition-colors" disabled={row.run === true}
                            onClick={() => onRemove(row.uuid)} title="删除" aria-label="删除"><i
                              className="fa-solid fa-trash text-sm"></i></button>
                          <button className="px-2.5 py-1.5 text-gray-700 bg-white border border-gray-300 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed first:rounded-l-md last:rounded-r-md transition-colors" disabled={!row.realLogPath} onClick={() => viewLog(row)}
                            title={row.realLogPath ? '查看日志' : '日志(未开启)'}
                            aria-label="查看日志"><i className="fa-regular fa-file-lines text-sm"></i></button>
                        </div>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <TaskEditModal
        show={showModal}
        modelValue={model}
        onUpdateModelValue={setModel}
        onClose={() => setShowModal(false)}
        onSave={onPositiveClick}
      />

      <LogViewerModal
        show={showLogModal}
        job={currentJob}
        logInfo={logInfo}
        onClose={() => setShowLogModal(false)}
      />

      {showDeleteModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="bg-white rounded-lg p-6 shadow-xl w-11/12 max-w-md">
            <h3 className="font-bold text-lg">确认删除</h3>
            <p className="py-4 text-gray-600">确定要删除这个任务吗？此操作无法撤销。</p>
            <div className="mt-6 flex justify-end gap-2">
              <button className="px-4 py-2 bg-red-600 text-white font-medium rounded-md hover:bg-red-700 transition-colors" onClick={confirmDelete}>删除</button>
              <button className="px-4 py-2 bg-gray-100 text-gray-700 font-medium rounded-md hover:bg-gray-200 transition-colors" onClick={() => setShowDeleteModal(false)}>取消</button>
            </div>
          </div>
        </div>
      )}

      {toast.show && (
        <div className="fixed top-4 left-1/2 -translate-x-1/2 z-50 transition-all duration-300">
          <div className="px-4 py-3 rounded-lg bg-green-50 border border-green-200 text-green-800 flex items-center gap-2 shadow-md font-medium text-sm">
            <span>{toast.message}</span>
          </div>
        </div>
      )}
    </div>
  )
}
