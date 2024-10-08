<script setup lang="ts">
import {
  NButton,
  NCard,
  NDataTable,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NInputNumber,
  NModal,
  NRadio,
  NRadioGroup,
  NSelect,
  NSpace,
  NSwitch,
  NTabPane,
  NTabs,
  NTag,
  useMessage
} from "naive-ui"
import {h, onMounted, onUnmounted, ref} from "vue";
import {getJobList, removeTask, runInfo, runJob, runOpenCloseTask, runTask, saveTask, stopJob} from "@/request/remote"
import {ArrowDownCircleOutline, ArrowUpCircleOutline} from '@vicons/ionicons5'

const message = useMessage()
const columns = [
  {title: 'TaskId', key: 'jobName'},
  {
    title: '类型', key: 'type', render(row: any) {
      return row.type === 1 ? "常驻" : "定时"
    },
  },
  {
    title: '启动', key: 'run', render(row: any) {
      return h(NTag, {
        bordered: false,
        type: row.run ? "info" : "default",
      }, {default: () => row.run ? "开启" : "关闭"})
    },
  },
  {
    title: '运行状态', key: 'status', ellipsis: true, render(row: any) {
      return h(NTag, {
            bordered: false,
            type: row.status === 1 ? "success" : "warning"
          },
          {default: () => row.status === 1 ? "运行" : "暂停"}
      )
    },
  },
  {
    title: 'Opt', key: 'opt', ellipsis: true, render(row: any) {
      let rm = h(
          NButton,
          {
            // strong: true,
            // tertiary: true,
            type: "error",
            size: "small",
            ghost: true,
            onClick: () => removeTask(row.uuid).then(r => getData(0))
          },
          {default: () => "删除"}
      )
      let jobButton = [h(
          NButton,
          {
            // strong: true,
            // tertiary: true,
            type: "error",
            size: "small",
            ghost: true,
            onClick: () => stopJob(row.uuid).then(r => getData(0))
          },
          {default: () => "停止"}
      ), h(
          NButton,
          {
            // strong: true,
            // tertiary: true,
            type: "primary",
            size: "small",
            ghost: true,
            onClick: () => runJob(row.uuid).then(r => {
              message.success(r.data.message)
              getData(0)
            })
          },
          {default: () => "启动"}
      )]
      let taskButton = [
        h(NButton,
            {
              // strong: true,
              // tertiary: true,
              type: row.run ? "error" : "warning",
              size: "small",
              ghost: true,
              onClick: () => runOpenCloseTask(row.uuid, !row.run).then(r => {
                message.success(r.data.message)
                return getData(0)
              })
            },
            {default: () => row.run ? "停止" : "启动"}
        ),
        h(NButton,
            {
              // strong: true,
              // tertiary: true,
              type: "primary",
              size: "small",
              ghost: true,
              onClick: () => runTask(row.uuid).then(r => {
                message.success(r.data.message)
                return getData(0)
              })
            },
            {default: () => "运行"}
        )
      ]
      let editButton = h(NButton,
          {
            // strong: true,
            // tertiary: true,
            type: "default",
            size: "small",
            ghost: true,
            onClick: () => edit(row)
          },
          {default: () => "编辑"}
      )
      jobButton.push(editButton)
      jobButton.push(rm)
      taskButton.push(editButton)
      taskButton.push(rm)
      if (row.type === 1) {
        return h(NSpace,
            {},
            {default: () => jobButton})
      } else {
        return h(NSpace,
            {},
            {default: () => taskButton})
      }
    }
  }
]
const data = ref([])
const data4residentTask = ref<any>([])
const data4scheduledTask = ref<any>([])

onMounted(() => {
  getData(0)
})

const showModal = ref(false);
const rules = {}

function getInitData(type: any) {
  return {
    uuid: "",
    jobName: "",
    type: type,
    spec: "* * * * *",
    binPath: "",
    dir: "",
    run: false,
    params: [],
    maxFailures: 3,
    options: {
      maxFailures: 5,
      outputPath: "/tmp",
      outputType: 1
    },
    edit: false,
  }
}

const model = ref(getInitData(1))

function add(type: any) {
  console.log(type)
  model.value = getInitData(type)
  showModal.value = true

}


function onNegativeClick() {
  message.success('Cancel')
  showModal.value = false
}

async function onPositiveClick() {
  let res = await saveTask(model.value)
  message.info(res.data.message)
  await getData(0)
}

function edit(row: any) {
  model.value = {
    uuid: row.uuid,
    jobName: row.jobName,
    type: row.type,
    spec: row.spec,
    binPath: row.binPath,
    dir: row.dir,
    run: true,
    params: row.params,
    maxFailures: 3,
    options: row.options,
    edit: true
  }
  showModal.value = true
}

async function getData(show = 1) {
  let resp = await getJobList()
  data.value = resp.data.message
  let wait4data4residentTask: any[] = [];
  let wait4data4scheduledTask: any[] = [];
  for (let item in resp.data.message) {
    if (resp.data.message[item].type === 1) {
      wait4data4residentTask.push(resp.data.message[item])
    } else {
      wait4data4scheduledTask.push(resp.data.message[item])
    }
  }
  data4residentTask.value = wait4data4residentTask
  data4scheduledTask.value = wait4data4scheduledTask
  if (show === 1) {
    message.success("刷新成功")
  }
}

let appRunTime = ref({
  start: "",
  runTime: "",
})
let id = setInterval(() => {
})

onMounted(() => {
  id = setInterval(() => {
    runInfo().then(r => {
      appRunTime.value = {
        runTime: r.data.runTime,
        start: r.data.start,
      }
    }).catch(r => {
    })
  }, 1000)
})
onUnmounted(() => {
  clearInterval(id)
})
</script>
<template>
  <n-tabs
      type="line"
      size="large"
      animated
  >
    <n-tab-pane name="守护">
      <n-space vertical>
        <n-space>
          <n-button @click="getData(1)">刷新列表</n-button>
          <n-button @click="add(1)">新增常驻任务</n-button>
        </n-space>
        <n-data-table
            :columns="columns"
            :data="data4residentTask"
            :bordered="true"
        />
      </n-space>
    </n-tab-pane>
    <n-tab-pane name="调度">
      <n-space vertical>
        <n-space>
          <n-button @click="getData(1)">刷新列表</n-button>
          <n-button @click="add(2)">新增定时任务</n-button>
        </n-space>
        <n-data-table
            :columns="columns"
            :data="data4scheduledTask"
            :bordered="true"
        />
      </n-space>
    </n-tab-pane>
    <n-tab-pane name="关于">
      <n-card>
        TaskManager启动于{{ appRunTime.start }},目前已经运行<span style="color: #3f7fe0;">{{
          appRunTime.runTime
        }}</span>
      </n-card>
    </n-tab-pane>
  </n-tabs>

  <n-modal
      v-model:show="showModal"
      style="width: 600px"
      :mask-closable="false"
      preset="dialog"
      :title="model.edit?'任务调整':'任务新增'"
      content="你确认?"
      positive-text="保存"
      negative-text="放弃"
      @positive-click="onPositiveClick"
      @negative-click="onNegativeClick"
      :style="{paddingTop:'40px'}"
  >
    <n-form
        ref="formRef"
        :model="model"
        :rules="rules"
        label-placement="left"
        label-width="auto"
        require-mark-placement="right-hanging"
        :style="{maxWidth: '640px',paddingTop:'20px'}"
    >
      <n-form-item label="JobName" path="JobName">
        <n-input v-model:value="model.jobName">ip</n-input>
      </n-form-item>
      <n-form-item label="Textarea" path="textareaValue">
        <n-radio-group v-model:value="model.type" name="radiogroup1" :disabled="model.edit">
          <n-space>
            <n-radio :value="1">
              常驻任务
            </n-radio>
            <n-radio :value="2">
              定时任务
            </n-radio>
          </n-space>
        </n-radio-group>
      </n-form-item>
      <n-form-item label="Cron" path="Cron" v-show="model.type===2">
        <n-input v-model:value="model.spec">port</n-input>
      </n-form-item>
      <n-form-item label="BinPath" path="BinPath">
        <n-input v-model:value="model.binPath"></n-input>
      </n-form-item>
      <n-form-item label="RunPath" path="RunPath">
        <n-input v-model:value="model.dir"></n-input>
      </n-form-item>
      <n-form-item label="Open" path="Run">
        <n-switch v-model:value="model.run" :disabled="true"/>
      </n-form-item>
      <n-form-item label="Params" path="Params">
        <n-select v-model:value="model.params"
                  filterable
                  multiple
                  tag
                  placeholder="输入，按回车确认"
                  :show-arrow="false"
                  :show="false"></n-select>
      </n-form-item>
      <n-form-item label="MaxFailures" path="MaxFailures">
        <n-input-number v-model:value="model.options.maxFailures">
          <template #minus-icon>
            <n-icon :component="ArrowDownCircleOutline"/>
          </template>
          <template #add-icon>
            <n-icon :component="ArrowUpCircleOutline"/>
          </template>
        </n-input-number>
      </n-form-item>
      <n-form-item label="Log" path="Log">
        <n-radio-group v-model:value="model.options.outputType" name="radioGroup1">
          <n-space>
            <n-radio :value="1">
              标准(默认)
            </n-radio>
            <n-radio :value="2">
              文件
            </n-radio>
          </n-space>
        </n-radio-group>
      </n-form-item>
      <n-form-item label="LogDir" path="MaxFailures" v-show="model.options.outputType===2">
        <n-input v-model:value="model.options.outputPath">
        </n-input>
      </n-form-item>
    </n-form>
  </n-modal>
</template>
