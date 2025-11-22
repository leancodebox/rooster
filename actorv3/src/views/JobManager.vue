<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { getJobList, removeTask, runInfo, runJob, runTask, saveTask, stopJob, getJobLogList, getJobLog, downloadJobLog } from '../request/remote'

const hasLogById = ref<Record<string, boolean>>({})
const data = ref<any[]>([])
const resident = ref<any[]>([])
const scheduled = ref<any[]>([])
const showModal = ref(false)
const showLogModal = ref(false)
const logContent = ref('')
const logInfo = ref<any>({ hasLog: false, logPath: '', size: 0, modTime: '', uuid: '' })
const appRunTime = ref({ start: '', runTime: '' })
const model = ref(getInitData(1))
let timer: any = 0

function getInitData(type: number) {
  return {
    uuid: '', jobName: '', type, spec: '* * * * *', binPath: '', dir: '', run: false,
    params: [], options: { maxFailures: 5, outputPath: '/tmp', outputType: 1 }, edit: false
  }
}
function add(type: number) { model.value = getInitData(type); showModal.value = true }
async function onPositiveClick() { await saveTask(model.value); await refresh(); showModal.value = false }
function edit(row: any) { model.value = { ...row, edit: true }; showModal.value = true }

async function viewLog(row: any) {
  const list = await getJobLogList()
  const item = (list.data.message as any[]).find((x) => x.uuid === row.uuid)
  logInfo.value = item || { hasLog: false }
  if (!logInfo.value.hasLog) { logContent.value = '未开启文件日志或日志文件不存在' }
  else { const resp = await getJobLog(row.uuid, 200, 0); logContent.value = resp.data.content }
  showLogModal.value = true
}
async function downloadLog() {
  const r = await downloadJobLog(logInfo.value.uuid)
  const url = window.URL.createObjectURL(r.data as any)
  const a = window.document.createElement('a')
  a.href = url; a.download = (logInfo.value.logPath || 'log.txt'); a.click(); window.URL.revokeObjectURL(url)
}

async function refresh() {
  const resp = await getJobList(); data.value = resp.data.message
  const logsResp = await getJobLogList(); const logList = logsResp.data.message as any[]
  hasLogById.value = {}; for (const item of logList) hasLogById.value[item.uuid] = item.hasLog
  resident.value = data.value.filter((x) => x.type === 1)
  scheduled.value = data.value.filter((x) => x.type === 2)
}

onMounted(() => { refresh(); timer = setInterval(() => { runInfo().then((r: any) => { appRunTime.value = { runTime: r.data.runTime, start: r.data.start } }) }, 1000) })
onUnmounted(() => { clearInterval(timer) })
</script>

<template>
  <div class="app-container py-6 space-y-6">
    <h1 class="text-2xl font-bold">Task Manager</h1>
    <div class="flex items-center justify-between">
      <div class="space-x-2">
        <button class="btn btn-primary" @click="refresh">刷新列表</button>
        <button class="btn btn-success" @click="add(1)">新增常驻任务</button>
        <button class="btn btn-success" @click="add(2)">新增定时任务</button>
      </div>
      <div class="text-sm">启动于 {{ appRunTime.start }} 已运行 {{ appRunTime.runTime }}</div>
    </div>
    <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
      <div class="card bg-base-100 shadow">
        <div class="card-body">
          <h2 class="card-title">常驻任务</h2>
          <div class="overflow-x-auto">
            <table class="table table-zebra">
              <thead>
                <tr><th>JobName</th><th>跟随启动</th><th>运行状态</th><th>操作</th></tr>
              </thead>
              <tbody>
                <tr v-for="row in resident" :key="row.uuid">
                  <td>{{ row.jobName }}</td>
                  <td><span :class="row.run ? 'badge badge-success' : 'badge'">{{ row.run ? '开启' : '关闭' }}</span></td>
                  <td><span :class="row.status === 1 ? 'badge badge-success' : 'badge badge-warning'">{{ row.status === 1 ? '运行' : '暂停' }}</span></td>
                  <td class="space-x-2">
                    <button class="btn btn-sm" @click="stopJob(row.uuid).then(refresh)">停止</button>
                    <button class="btn btn-sm btn-primary" :disabled="row.status===1" @click="runJob(row.uuid).then(refresh)">启动</button>
                    <button class="btn btn-sm" :disabled="row.status===1" @click="edit(row)">编辑</button>
                    <button class="btn btn-sm" :disabled="row.status===1" @click="removeTask(row.uuid).then(refresh)">删除</button>
                    <button class="btn btn-sm" :disabled="!hasLogById[row.uuid]" @click="viewLog(row)">{{ hasLogById[row.uuid] ? '日志' : '日志(未开启)' }}</button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
      <div class="card bg-base-100 shadow">
        <div class="card-body">
          <h2 class="card-title">定时任务</h2>
          <div class="overflow-x-auto">
            <table class="table table-zebra">
              <thead>
                <tr><th>JobName</th><th>已开启</th><th>操作</th></tr>
              </thead>
              <tbody>
                <tr v-for="row in scheduled" :key="row.uuid">
                  <td>{{ row.jobName }}</td>
                  <td><span :class="row.run ? 'badge badge-success' : 'badge'">{{ row.run ? '开启' : '关闭' }}</span></td>
                  <td class="space-x-2">
                    <button class="btn btn-sm" @click="stopJob(row.uuid).then(refresh)">停止</button>
                    <button class="btn btn-sm btn-primary" @click="runTask(row.uuid).then(refresh)">运行</button>
                    <button class="btn btn-sm" :disabled="row.status===1" @click="edit(row)">编辑</button>
                    <button class="btn btn-sm" :disabled="row.status===1" @click="removeTask(row.uuid).then(refresh)">删除</button>
                    <button class="btn btn-sm" :disabled="!hasLogById[row.uuid]" @click="viewLog(row)">{{ hasLogById[row.uuid] ? '日志' : '日志(未开启)' }}</button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>

    <div v-if="showModal" class="modal modal-open">
      <div class="modal-box w-11/12 max-w-xl">
        <h3 class="font-bold text-lg">{{ model.edit? '任务调整':'任务新增' }}</h3>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4 py-4">
          <div class="form-control"><label class="label"><span class="label-text">JobName</span></label><input class="input input-bordered" v-model="model.jobName" /></div>
          <div class="form-control"><label class="label"><span class="label-text">类型</span></label><select class="select select-bordered" v-model="model.type" :disabled="model.edit"><option :value="1">常驻任务</option><option :value="2">定时任务</option></select></div>
          <div class="form-control" v-if="model.type===2"><label class="label"><span class="label-text">Cron</span></label><input class="input input-bordered" v-model="model.spec" /></div>
          <div class="form-control"><label class="label"><span class="label-text">BinPath</span></label><input class="input input-bordered" v-model="model.binPath" /></div>
          <div class="form-control"><label class="label"><span class="label-text">RunPath</span></label><input class="input input-bordered" v-model="model.dir" /></div>
          <div class="form-control"><label class="label"><span class="label-text">日志方式</span></label><select class="select select-bordered" v-model="model.options.outputType"><option :value="1">标准</option><option :value="2">文件</option></select></div>
          <div class="form-control" v-if="model.options.outputType===2"><label class="label"><span class="label-text">LogDir</span></label><input class="input input-bordered" v-model="model.options.outputPath" /></div>
          <div class="form-control"><label class="label"><span class="label-text">MaxFailures</span></label><input type="number" class="input input-bordered" v-model="model.options.maxFailures" /></div>
        </div>
        <div class="modal-action"><button class="btn" @click="onPositiveClick">保存</button><button class="btn" @click="showModal=false">放弃</button></div>
      </div>
    </div>

    <div v-if="showLogModal" class="modal modal-open">
      <div class="modal-box w-11/12 max-w-2xl">
        <h3 class="font-bold text-lg">日志查看</h3>
        <div class="py-2">路径: {{ logInfo.logPath || '-' }} | 大小: {{ logInfo.size }} | 更新时间: {{ logInfo.modTime || '-' }}</div>
        <pre class="max-h-[400px] overflow-auto bg-black text-gray-100 p-3 rounded">{{ logContent }}</pre>
        <div class="modal-action"><button class="btn" @click="downloadLog">下载</button><button class="btn" @click="showLogModal=false">关闭</button></div>
      </div>
    </div>
  </div>
</template>
