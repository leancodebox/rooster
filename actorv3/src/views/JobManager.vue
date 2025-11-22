<script setup lang="ts">
import {onMounted, onUnmounted, ref} from 'vue'
import {
  downloadJobLog,
  getJobList,
  getJobLog,
  getJobLogList,
  removeTask,
  runInfo,
  runJob,
  runTask,
  saveTask,
  stopJob,
  getHomePath
} from '../request/remote'

const hasLogById = ref<Record<string, boolean>>({}) // 兼容旧用法
const logMapById = ref<Record<string, any>>({})
const data = ref<any[]>([])
const resident = ref<any[]>([])
const scheduled = ref<any[]>([])
const showModal = ref(false)
const showLogModal = ref(false)
const logContent = ref('')
const logInfo = ref<any>({hasLog: false, logPath: '', size: 0, modTime: '', uuid: ''})
const logOrigin = ref<'文件' | '内存' | ''>('')
const isStreaming = ref(false)
const autoScroll = ref(true)
let logTimer: any = 0
let evt: EventSource | null = null
const appRunTime = ref({start: '', runTime: ''})
const defaultLogDir = ref('')
const model = ref(getInitData(1))
let timer: any = 0

function getInitData(type: number) {
  return {
    uuid: '', jobName: '', type, spec: '* * * * *', binPath: '', dir: '', run: false,
    options: {maxFailures: 5, outputPath: defaultLogDir.value || '/tmp', outputType: 2}, edit: false, readonly: false
  }
}

function add(type: number) {
  model.value = getInitData(type);
  showModal.value = true
}

async function onPositiveClick() {
  await saveTask(model.value);
  await refresh();
  showModal.value = false
}

function edit(row: any) {
  model.value = {...row, edit: true, readonly: row.status === 1};
  showModal.value = true
}

async function viewLog(row: any) {
  const item = logMapById.value[row.uuid]
  logInfo.value = item || {hasLog: false}
  logOrigin.value = logInfo.value.hasLog ? (logInfo.value.logPath ? '文件' : '内存') : ''
  isStreaming.value = false
  clearInterval(logTimer)
  if (evt) { evt.close(); evt = null }
  if (!logInfo.value.hasLog) {
    logContent.value = '未开启日志或日志暂无内容'
  } else {
    const resp = await getJobLog(row.uuid, 200, 0)
    logContent.value = resp.data.content || ''
  }
  showLogModal.value = true
}

async function downloadLog() {
  const r = await downloadJobLog(logInfo.value.uuid)
  const url = window.URL.createObjectURL(r.data as any)
  const a = window.document.createElement('a')
  a.href = url;
  a.download = (logInfo.value.logPath || 'log.txt');
  a.click();
  window.URL.revokeObjectURL(url)
}

function scrollToBottom() {
  if (!autoScroll.value) return
  const pre = document.querySelector('#log-view-pre') as HTMLElement
  if (pre) pre.scrollTop = pre.scrollHeight
}

function startStreaming() {
  if (!logInfo.value.hasLog) return
  isStreaming.value = true
  clearInterval(logTimer)
  if (evt) { evt.close(); evt = null }
  if (logInfo.value.logPath) {
    evt = new EventSource(`/api/job-log-stream?jobId=${encodeURIComponent(logInfo.value.uuid)}`)
    evt.onmessage = (e) => {
      logContent.value += e.data + '\n'
      scrollToBottom()
    }
    evt.onerror = () => {}
  } else {
    logTimer = setInterval(async () => {
      const resp = await getJobLog(logInfo.value.uuid, 200, 0)
      const content = resp.data.content || ''
      logContent.value = content
      scrollToBottom()
    }, 1000)
  }
}

function stopStreaming() {
  isStreaming.value = false
  clearInterval(logTimer)
  if (evt) { evt.close(); evt = null }
}

async function refresh() {
  const resp = await getJobList();
  data.value = resp.data.message
  const logsResp = await getJobLogList();
  const logList = logsResp.data.message as any[]
  hasLogById.value = {};
  logMapById.value = {};
  for (const item of logList) {
    hasLogById.value[item.uuid] = item.hasLog
    logMapById.value[item.uuid] = item
  }
  resident.value = data.value.filter((x) => x.type === 1)
  scheduled.value = data.value.filter((x) => x.type === 2)
}

function confirmStatus(jobId: string, check: (row: any) => boolean, retries = 10, interval = 300) {
  let count = 0
  const h = setInterval(async () => {
    await refresh()
    const row = data.value.find((x: any) => x.uuid === jobId)
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

onMounted(async () => {
  refresh();
  try {
    const r = await getHomePath()
    const h = r.data.home || ''
    defaultLogDir.value = h ? `${h}/.roosterTaskConfig/log` : ''
  } catch {}
  timer = setInterval(() => {
    runInfo().then((r: any) => {
      appRunTime.value = {runTime: r.data.runTime, start: r.data.start}
    })
  }, 1000)
})
onUnmounted(() => {
  clearInterval(timer)
})
</script>

<template>
  <div class="max-w-7xl mx-auto px-4 md:px-6 lg:px-8 py-6 space-y-6">
    <h1 class="text-2xl font-bold">Task Manager</h1>
    <div class="flex items-center justify-between">
      <div class="flex flex-wrap gap-2">
        <button class="btn btn-primary btn-sm" @click="refresh" title="刷新列表" aria-label="刷新列表"><i class="fa-solid fa-arrows-rotate text-xl"></i></button>
        <button class="btn btn-success btn-sm" @click="add(1)" title="新增常驻任务" aria-label="新增常驻任务"><i class="fa-solid fa-plus text-xl"></i></button>
        <button class="btn btn-success btn-sm" @click="add(2)" title="新增定时任务" aria-label="新增定时任务"><i class="fa-solid fa-calendar-plus text-xl"></i></button>
      </div>
      <div class="text-sm" v-if="appRunTime.start"> 已运行 {{ appRunTime.runTime }}</div>
    </div>
    <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
      <div>
        <h2 class="text-lg font-semibold mb-2">常驻任务</h2>
        <div class="overflow-x-auto">
          <table class="table  table-sm border border-base-300 rounded-lg">
            <thead>
            <tr>
              <th>JobName</th>
              <th>跟随启动</th>
              <th>运行状态</th>
              <th>操作</th>
            </tr>
            </thead>
            <tbody>
            <tr v-for="row in resident" :key="row.uuid">
              <td class="truncate max-w-[14rem] sm:max-w-[18rem]">{{ row.jobName }}</td>
              <td class="whitespace-nowrap"><span class="text-[10px] sm:text-sm"
                                                  :class="row.run ? 'badge badge-success badge-sm whitespace-nowrap' : 'badge badge-sm whitespace-nowrap'">{{
                  row.run ? '开启' : '关闭'
                }}</span></td>
              <td class="whitespace-nowrap"><span class="text-[10px] sm:text-sm"
                                                  :class="row.status === 1 ? 'badge badge-success badge-sm whitespace-nowrap' : 'badge badge-warning badge-sm whitespace-nowrap'">{{
                  row.status === 1 ? '运行' : '暂停'
                }}</span></td>
              <td>
                <div class="join">
                  <button class="btn btn-sm join-item" @click="onStopResident(row.uuid)" title="停止" aria-label="停止"><i class="fa-solid fa-stop text-sm"></i></button>
                  <button class="btn btn-sm btn-primary join-item" :disabled="row.status===1" @click="onStartResident(row.uuid)" title="启动" aria-label="启动"><i class="fa-solid fa-play text-sm"></i></button>
                  <button class="btn btn-sm join-item" @click="edit(row)" title="编辑" aria-label="编辑"><i class="fa-solid fa-pen-to-square text-sm"></i></button>
                  <button class="btn btn-sm join-item" :disabled="row.status===1" @click="removeTask(row.uuid).then(refresh)" title="删除" aria-label="删除"><i class="fa-solid fa-trash text-sm"></i></button>
                  <button class="btn btn-sm join-item" :disabled="!logMapById[row.uuid]?.hasLog" @click="viewLog(row)" :title="logMapById[row.uuid]?.hasLog ? (logMapById[row.uuid]?.logPath ? '日志(文件)' : '日志(内存)') : '日志(未开启)'" aria-label="查看日志"><i class="fa-regular fa-file-lines text-sm"></i></button>
                </div>
              </td>
            </tr>
            </tbody>
          </table>
        </div>
      </div>
      <div>
        <h2 class="text-lg font-semibold mb-2">定时任务</h2>
        <div class="overflow-x-auto">
          <table class="table  table-sm border border-base-300 rounded-lg">
            <thead>
            <tr>
              <th>JobName</th>
              <th>已开启</th>
              <th>操作</th>
            </tr>
            </thead>
            <tbody>
            <tr v-for="row in scheduled" :key="row.uuid">
              <td class="truncate max-w-[14rem] sm:max-w-[18rem]">{{ row.jobName }}</td>
              <td class="whitespace-nowrap"><span class="text-[10px] sm:text-sm"
                                                  :class="row.run ? 'badge badge-success badge-sm whitespace-nowrap' : 'badge badge-sm whitespace-nowrap'">{{
                  row.run ? '开启' : '关闭'
                }}</span></td>
              <td>
                <div class="join">
                  <button class="btn btn-sm join-item" @click="onStopResident(row.uuid)" title="停止" aria-label="停止"><i class="fa-solid fa-stop text-sm"></i></button>
                  <button class="btn btn-sm btn-primary join-item" @click="runTask(row.uuid).then(refresh)" title="运行" aria-label="运行"><i class="fa-solid fa-play text-sm"></i></button>
                  <button class="btn btn-sm join-item" @click="edit(row)" title="编辑" aria-label="编辑"><i class="fa-solid fa-pen-to-square text-sm"></i></button>
                  <button class="btn btn-sm join-item" :disabled="row.status===1" @click="removeTask(row.uuid).then(refresh)" title="删除" aria-label="删除"><i class="fa-solid fa-trash text-sm"></i></button>
                  <button class="btn btn-sm join-item" :disabled="!logMapById[row.uuid]?.hasLog" @click="viewLog(row)" :title="logMapById[row.uuid]?.hasLog ? (logMapById[row.uuid]?.logPath ? '日志(文件)' : '日志(内存)') : '日志(未开启)'" aria-label="查看日志"><i class="fa-regular fa-file-lines text-sm"></i></button>
                </div>
              </td>
            </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <div v-if="showModal" class="modal modal-open">
      <div class="modal-box w-11/12 max-w-xl">
        <h3 class="font-bold text-lg">{{ model.edit ? '任务调整' : '任务新增' }}</h3>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3 py-4">
          <div class="form-control"><label class="label"><span class="label-text">JobName</span></label><input
              class="input input-bordered input-sm" v-model="model.jobName" :disabled="model.readonly"/></div>
          <div class="form-control"><label class="label"><span class="label-text">类型</span></label><select
              class="select select-bordered select-sm" v-model="model.type" :disabled="model.edit || model.readonly">
            <option :value="1">常驻任务</option>
            <option :value="2">定时任务</option>
          </select></div>
          <div class="form-control" v-if="model.type===2"><label class="label"><span
              class="label-text">Cron</span></label><input class="input input-bordered input-sm" v-model="model.spec"
                                                           :disabled="model.readonly"/></div>
          <div class="form-control"><label class="label"><span class="label-text">Command</span></label><input
              class="input input-bordered input-sm" v-model="model.binPath" :disabled="model.readonly"
              placeholder="例如：/bin/bash -lc 'echo hi'"/></div>
          <div class="form-control"><label class="label"><span class="label-text">RunPath</span></label><input
              class="input input-bordered input-sm" v-model="model.dir" :disabled="model.readonly"/></div>
          <div class="form-control"><label class="label"><span class="label-text">日志方式</span></label><select
              class="select select-bordered select-sm" v-model="model.options.outputType" :disabled="model.readonly">
            <option :value="1">标准</option>
            <option :value="2">文件</option>
          </select></div>
          <div class="form-control" v-if="model.options.outputType===2"><label class="label"><span class="label-text">LogDir</span></label><input
              class="input input-bordered input-sm" v-model="model.options.outputPath" :disabled="model.readonly"/>
          </div>
          <div class="form-control"><label class="label"><span class="label-text">MaxFailures</span></label><input
              type="number" class="input input-bordered input-sm" v-model="model.options.maxFailures"
              :disabled="model.readonly"/></div>
        </div>
        <div class="modal-action">
          <button class="btn" @click="onPositiveClick" :disabled="model.readonly" title="保存" aria-label="保存"><i class="fa-solid fa-floppy-disk text-xl"></i></button>
          <button class="btn" @click="showModal=false" title="关闭" aria-label="关闭"><i class="fa-solid fa-xmark text-xl"></i></button>
        </div>
      </div>
    </div>

    <div v-if="showLogModal" class="modal modal-open">
      <div class="modal-box w-11/12 max-w-2xl">
        <h3 class="font-bold text-lg">日志查看</h3>
        <div class="py-2">来源: {{ logOrigin || '-' }} | 路径: {{ logInfo.logPath || '-' }} | 大小: {{ logInfo.size }} | 更新时间: {{ logInfo.modTime || '-' }}</div>
        <div class="flex items-center gap-2 mb-2">
          <label class="label cursor-pointer">
            <span class="label-text">实时滚动</span>
            <input type="checkbox" class="toggle toggle-sm" v-model="isStreaming" @change="isStreaming ? startStreaming() : stopStreaming()"/>
          </label>
          <label class="label cursor-pointer">
            <span class="label-text">自动滚动</span>
            <input type="checkbox" class="toggle toggle-sm" v-model="autoScroll"/>
          </label>
        </div>
        <pre id="log-view-pre" class="max-h-[420px] overflow-auto bg-black text-gray-100 p-3 rounded font-mono text-sm">{{
            logContent
          }}</pre>
        <div class="modal-action"><button class="btn" :disabled="!logInfo.logPath" @click="downloadLog" title="下载" aria-label="下载"><i class="fa-solid fa-download text-xl"></i></button><button class="btn" @click="showLogModal=false; stopStreaming()" title="关闭" aria-label="关闭"><i class="fa-solid fa-xmark text-xl"></i></button></div>
      </div>
    </div>
  </div>
</template>
