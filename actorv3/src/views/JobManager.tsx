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
    <div className="max-w-7xl mx-auto px-4 md:px-6 lg:px-8 py-6 space-y-4 font-sans bg-white min-h-screen">
      {/* Header */}
      <div className="flex items-center justify-between pb-2">
        <h1 className="text-2xl font-bold text-gray-900 tracking-tight">Task Manager</h1>
        <div className="flex items-center gap-4">
          <div className="flex gap-2">
            <button 
              className="flex items-center justify-center w-9 h-9 rounded-full bg-indigo-50 text-indigo-600 hover:bg-indigo-100 transition-colors focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-1" 
              onClick={() => add(1)} 
              title="新增常驻任务"
            >
              <i className="fa-solid fa-plus text-sm"></i>
            </button>
            <button 
              className="flex items-center justify-center w-9 h-9 rounded-full bg-emerald-50 text-emerald-600 hover:bg-emerald-100 transition-colors focus:outline-none focus:ring-2 focus:ring-emerald-500 focus:ring-offset-1" 
              onClick={() => add(2)} 
              title="新增定时任务"
            >
              <i className="fa-solid fa-calendar-plus text-sm"></i>
            </button>
            <button 
              className="flex items-center justify-center w-9 h-9 rounded-full bg-gray-50 text-gray-600 hover:bg-gray-100 transition-colors focus:outline-none focus:ring-2 focus:ring-gray-500 focus:ring-offset-1" 
              onClick={onRefreshClick} 
              title="刷新列表"
            >
              <i className="fa-solid fa-arrows-rotate text-sm"></i>
            </button>
          </div>
          {appRunTime.start && (
            <div className="hidden sm:block text-xs font-medium text-gray-500 bg-gray-50 px-4 py-2 rounded-full border border-gray-100 shadow-sm">
              <div className="grid grid-flow-col gap-3">
                <div className="flex items-baseline gap-1">
                  <span className="font-mono text-gray-900 font-bold">{runtimeDigits.day}</span>
                  <span className="text-gray-400">天</span>
                </div>
                <div className="flex items-baseline gap-1">
                  <span className="font-mono text-gray-900 font-bold">{String(runtimeDigits.hour).padStart(2, '0')}</span>
                  <span className="text-gray-400">时</span>
                </div>
                <div className="flex items-baseline gap-1">
                  <span className="font-mono text-gray-900 font-bold">{String(runtimeDigits.minute).padStart(2, '0')}</span>
                  <span className="text-gray-400">分</span>
                </div>
                <div className="flex items-baseline gap-1">
                  <span className="font-mono text-gray-900 font-bold w-[18px] text-right">{String(runtimeDigits.second).padStart(2, '0')}</span>
                  <span className="text-gray-400">秒</span>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {/* Resident Tasks Card */}
        <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden flex flex-col h-full">
          <div className="px-6 py-4 border-b border-gray-100 bg-gray-50/50 flex items-center justify-between">
            <h2 className="text-base font-semibold text-gray-900 flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-indigo-500"></span>
              常驻任务
            </h2>
            <span className="text-xs font-medium text-gray-500 bg-white px-2 py-1 rounded border border-gray-200">{resident.length}</span>
          </div>
          
          <div className="overflow-x-auto flex-1">
            {resident.length > 0 ? (
              <table className="w-full text-left border-collapse">
                <thead>
                  <tr className="bg-gray-50/50 border-b border-gray-100">
                    <th className="px-4 py-3 text-xs font-bold text-gray-600 uppercase tracking-wider">Job Name</th>
                    <th className="px-2 py-3 text-xs font-bold text-gray-600 uppercase tracking-wider text-center w-16">Auto</th>
                    <th className="px-2 py-3 text-xs font-bold text-gray-600 uppercase tracking-wider text-center w-24">Status</th>
                    <th className="px-4 py-3 text-xs font-bold text-gray-600 uppercase tracking-wider text-right w-32">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {resident.map((row) => (
                    <tr key={row.uuid} className="group hover:bg-gray-50 transition-colors">
                      <td className="px-4 py-3 whitespace-nowrap">
                        <div className="flex flex-col">
                          {row.link ? (
                            <a href={row.link} className="text-sm font-bold text-indigo-600 hover:text-indigo-800 font-mono tracking-tight truncate max-w-[8rem] sm:max-w-[12rem]" target="_blank" title={row.jobName}>{row.jobName}</a>
                          ) : (
                            <span className="text-sm font-bold text-gray-800 font-mono tracking-tight truncate max-w-[8rem] sm:max-w-[12rem]" title={row.jobName}>{row.jobName}</span>
                          )}
                        </div>
                      </td>
                      <td className="px-2 py-3 whitespace-nowrap text-center">
                         <div className={`inline-flex items-center justify-center w-6 h-6 rounded-full ${row.run ? 'bg-green-100 text-green-600' : 'bg-gray-100 text-gray-400'}`} title={row.run ? 'Auto Start: On' : 'Auto Start: Off'}>
                           <i className={`fa-solid ${row.run ? 'fa-bolt' : 'fa-power-off'} text-xs`}></i>
                         </div>
                      </td>
                      <td className="px-2 py-3 whitespace-nowrap text-center">
                        <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
                          row.status === 1 
                            ? 'bg-green-50 text-green-700 ring-1 ring-green-600/20' 
                            : 'bg-yellow-50 text-yellow-700 ring-1 ring-yellow-600/20'
                        }`}>
                          <span className={`w-1.5 h-1.5 rounded-full mr-1.5 ${row.status === 1 ? 'bg-green-500' : 'bg-yellow-500'}`}></span>
                          {row.status === 1 ? 'Run' : 'Stop'}
                        </span>
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap text-right">
                        <div className="flex items-center justify-end gap-1 opacity-80 group-hover:opacity-100 transition-opacity">
                          <button 
                            className="w-7 h-7 flex items-center justify-center text-gray-500 hover:text-red-600 hover:bg-red-50 rounded transition-colors"
                            onClick={() => onStopResident(row.uuid)} 
                            title="Stop"
                          >
                            <i className="fa-solid fa-stop text-xs"></i>
                          </button>
                          <button 
                            className={`w-7 h-7 flex items-center justify-center rounded transition-colors ${row.status === 1 ? 'text-gray-300 cursor-not-allowed' : 'text-emerald-600 hover:text-emerald-700 hover:bg-emerald-50'}`}
                            disabled={row.status === 1}
                            onClick={() => onStartResident(row.uuid)} 
                            title="Start"
                          >
                            <i className="fa-solid fa-play text-xs"></i>
                          </button>
                          <div className="w-px h-3 bg-gray-200 mx-1"></div>
                          <button 
                            className="w-7 h-7 flex items-center justify-center text-gray-500 hover:text-indigo-600 hover:bg-indigo-50 rounded transition-colors"
                            onClick={() => edit(row)} 
                            title="Edit"
                          >
                            <i className="fa-solid fa-pen text-xs"></i>
                          </button>
                          <button 
                            className={`w-7 h-7 flex items-center justify-center rounded transition-colors ${row.run === true ? 'text-gray-300 cursor-not-allowed' : 'text-gray-500 hover:text-red-600 hover:bg-red-50'}`}
                            disabled={row.run === true}
                            onClick={() => onRemove(row.uuid)} 
                            title="Delete"
                          >
                            <i className="fa-solid fa-trash text-xs"></i>
                          </button>
                          <button 
                            className={`w-7 h-7 flex items-center justify-center rounded transition-colors ${!row.realLogPath ? 'text-gray-300 cursor-not-allowed' : 'text-gray-500 hover:text-gray-900 hover:bg-gray-100'}`}
                            disabled={!row.realLogPath} 
                            onClick={() => viewLog(row)}
                            title="View Log"
                          >
                            <i className="fa-regular fa-file-lines text-xs"></i>
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <div className="flex flex-col items-center justify-center py-12 text-gray-400">
                <i className="fa-solid fa-box-open text-4xl mb-3 opacity-20"></i>
                <p className="text-sm">No resident tasks found</p>
                <button onClick={() => add(1)} className="mt-4 text-xs font-medium text-indigo-600 hover:text-indigo-800 hover:underline">Create one?</button>
              </div>
            )}
          </div>
        </div>

        {/* Scheduled Tasks Card */}
        <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden flex flex-col h-full">
          <div className="px-6 py-4 border-b border-gray-100 bg-gray-50/50 flex items-center justify-between">
             <h2 className="text-base font-semibold text-gray-900 flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-emerald-500"></span>
              定时任务
            </h2>
            <span className="text-xs font-medium text-gray-500 bg-white px-2 py-1 rounded border border-gray-200">{scheduled.length}</span>
          </div>
          
          <div className="overflow-x-auto flex-1">
             {scheduled.length > 0 ? (
              <table className="w-full text-left border-collapse">
                <thead>
                  <tr className="bg-gray-50/50 border-b border-gray-100">
                    <th className="px-4 py-3 text-xs font-bold text-gray-600 uppercase tracking-wider">Job Name</th>
                    <th className="px-2 py-3 text-xs font-bold text-gray-600 uppercase tracking-wider text-center w-16">Enabled</th>
                    <th className="px-4 py-3 text-xs font-bold text-gray-600 uppercase tracking-wider text-right w-32">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {scheduled.map((row) => (
                    <tr key={row.uuid} className="group hover:bg-gray-50 transition-colors">
                      <td className="px-4 py-3 whitespace-nowrap">
                         <span className="text-sm font-bold text-gray-800 font-mono tracking-tight truncate max-w-[8rem] sm:max-w-[12rem] block" title={row.jobName}>{row.jobName}</span>
                      </td>
                      <td className="px-2 py-3 whitespace-nowrap text-center">
                        <div className="flex justify-center">
                          <label className="relative inline-flex items-center cursor-pointer">
                            <input 
                              type="checkbox" 
                              className="sr-only peer" 
                              checked={row.run}
                              onChange={(e) => e.target.checked ? openScheduled(row.uuid) : closeScheduled(row.uuid)}
                            />
                            <div className="w-9 h-5 bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-emerald-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-emerald-500"></div>
                          </label>
                        </div>
                      </td>
                      <td className="px-4 py-3 whitespace-nowrap text-right">
                         <div className="flex items-center justify-end gap-1 opacity-80 group-hover:opacity-100 transition-opacity">
                          <button 
                            className="w-7 h-7 flex items-center justify-center text-gray-500 hover:text-emerald-600 hover:bg-emerald-50 rounded transition-colors"
                            onClick={() => runTask(row.uuid).then(refresh)} 
                            title="Run Once"
                          >
                            <i className="fa-solid fa-play text-xs"></i>
                          </button>
                          <div className="w-px h-3 bg-gray-200 mx-1"></div>
                          <button 
                            className="w-7 h-7 flex items-center justify-center text-gray-500 hover:text-indigo-600 hover:bg-indigo-50 rounded transition-colors"
                            onClick={() => edit(row)} 
                            title="Edit"
                          >
                            <i className="fa-solid fa-pen text-xs"></i>
                          </button>
                          <button 
                            className={`w-7 h-7 flex items-center justify-center rounded transition-colors ${row.run === true ? 'text-gray-300 cursor-not-allowed' : 'text-gray-500 hover:text-red-600 hover:bg-red-50'}`}
                            disabled={row.run === true}
                            onClick={() => onRemove(row.uuid)} 
                            title="Delete"
                          >
                            <i className="fa-solid fa-trash text-xs"></i>
                          </button>
                          <button 
                            className={`w-7 h-7 flex items-center justify-center rounded transition-colors ${!row.realLogPath ? 'text-gray-300 cursor-not-allowed' : 'text-gray-500 hover:text-gray-900 hover:bg-gray-100'}`}
                            disabled={!row.realLogPath} 
                            onClick={() => viewLog(row)}
                            title="View Log"
                          >
                            <i className="fa-regular fa-file-lines text-xs"></i>
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
             ) : (
              <div className="flex flex-col items-center justify-center py-12 text-gray-400">
                <i className="fa-regular fa-calendar-xmark text-4xl mb-3 opacity-20"></i>
                <p className="text-sm">No scheduled tasks</p>
                <button onClick={() => add(2)} className="mt-4 text-xs font-medium text-emerald-600 hover:text-emerald-800 hover:underline">Schedule one?</button>
              </div>
             )}
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
