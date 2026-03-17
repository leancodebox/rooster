import { useEffect, useState } from 'react'

interface TaskEditModalProps {
  show: boolean
  modelValue: any
  onUpdateModelValue: (value: any) => void
  onClose: () => void
  onSave: () => void
}

export default function TaskEditModal({ show, modelValue, onUpdateModelValue, onClose, onSave }: TaskEditModalProps) {
  const [localModel, setLocalModel] = useState({ ...modelValue })

  useEffect(() => {
    setLocalModel({ ...modelValue })
  }, [modelValue])

  useEffect(() => {
    // 简单的深度比较，避免死循环，实际项目中可能需要更严谨的比较
    if (JSON.stringify(localModel) !== JSON.stringify(modelValue)) {
      onUpdateModelValue(localModel)
    }
  }, [localModel])

  const handleChange = (field: string, value: any) => {
    setLocalModel((prev: any) => {
      const newData = { ...prev }
      if (field.includes('.')) {
        const [parent, child] = field.split('.')
        newData[parent] = { ...newData[parent], [child]: value }
      } else {
        newData[field] = value
      }
      return newData
    })
  }

  if (!show) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white rounded-lg p-6 shadow-xl w-11/12 max-w-xl">
        <h3 className="font-bold text-lg mb-4">{localModel.edit ? '任务调整' : '任务新增'}</h3>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="flex flex-col gap-1">
            <label className="text-sm font-medium text-gray-700">JobName</label>
            <input
              className="w-full px-3 py-1.5 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:bg-gray-100 disabled:text-gray-500"
              value={localModel.jobName}
              onChange={(e) => handleChange('jobName', e.target.value)}
              disabled={localModel.readonly}
            />
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-sm font-medium text-gray-700">类型</label>
            <select
              className="w-full px-3 py-1.5 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:bg-gray-100 disabled:text-gray-500 appearance-none bg-white"
              value={localModel.type}
              onChange={(e) => handleChange('type', Number(e.target.value))}
              disabled={localModel.edit || localModel.readonly}
            >
              <option value={1}>常驻任务</option>
              <option value={2}>定时任务</option>
            </select>
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-sm font-medium text-gray-700">link</label>
            <input
              className="w-full px-3 py-1.5 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:bg-gray-100 disabled:text-gray-500"
              value={localModel.link}
              onChange={(e) => handleChange('link', e.target.value)}
              disabled={localModel.readonly}
            />
          </div>
          {localModel.type === 2 && (
            <div className="flex flex-col gap-1">
              <label className="text-sm font-medium text-gray-700">Cron</label>
              <input
                className="w-full px-3 py-1.5 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:bg-gray-100 disabled:text-gray-500"
                value={localModel.spec}
                onChange={(e) => handleChange('spec', e.target.value)}
                disabled={localModel.readonly}
              />
            </div>
          )}
          <div className="flex flex-col gap-1">
            <label className="text-sm font-medium text-gray-700">Command</label>
            <input
              className="w-full px-3 py-1.5 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:bg-gray-100 disabled:text-gray-500"
              value={localModel.binPath}
              onChange={(e) => handleChange('binPath', e.target.value)}
              disabled={localModel.readonly}
              placeholder="例如：/bin/bash -lc 'echo hi'"
            />
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-sm font-medium text-gray-700">RunPath</label>
            <input
              className="w-full px-3 py-1.5 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:bg-gray-100 disabled:text-gray-500"
              value={localModel.dir}
              onChange={(e) => handleChange('dir', e.target.value)}
              disabled={localModel.readonly}
            />
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-sm font-medium text-gray-700">LogDir</label>
            <input
              className="w-full px-3 py-1.5 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:bg-gray-100 disabled:text-gray-500"
              value={localModel.options.outputPath}
              onChange={(e) => handleChange('options.outputPath', e.target.value)}
              disabled={localModel.readonly}
              placeholder="留空使用默认路径"
            />
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-sm font-medium text-gray-700">MaxFailures</label>
            <input
              type="number"
              className="w-full px-3 py-1.5 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:bg-gray-100 disabled:text-gray-500"
              value={localModel.options.maxFailures}
              onChange={(e) => handleChange('options.maxFailures', Number(e.target.value))}
              disabled={localModel.readonly}
            />
          </div>
        </div>
        <div className="mt-6 flex justify-end gap-2">
          <button
            className="px-4 py-2 bg-gray-100 text-gray-700 font-medium rounded-md hover:bg-gray-200 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            onClick={onSave}
            disabled={localModel.readonly}
            title="保存"
            aria-label="保存"
          >
            <i className="fa-solid fa-floppy-disk text-xl"></i>
          </button>
          <button
            className="px-4 py-2 bg-gray-100 text-gray-700 font-medium rounded-md hover:bg-gray-200 transition-colors"
            onClick={onClose}
            title="关闭"
            aria-label="关闭"
          >
            <i className="fa-solid fa-xmark text-xl"></i>
          </button>
        </div>
      </div>
    </div>
  )
}
