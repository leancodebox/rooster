const api = {
  jobList: () => fetch('/api/job-list').then(r=>r.json()),
  runInfo: () => fetch('/api/run-info').then(r=>r.json()),
  runJob: (uuid) => fetch('/api/run-job-resident-task',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({jobId:uuid})}).then(r=>r.json()),
  stopJob: (uuid) => fetch('/api/stop-job-resident-task',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({jobId:uuid})}).then(r=>r.json()),
  openCloseTask: (uuid,run) => fetch('/api/open-close-task',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({uuid,run})}).then(r=>r.json()),
  runTask: (uuid) => fetch('/api/run-task',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({taskId:uuid})}).then(r=>r.json()),
  saveTask: (data) => fetch('/api/save-task',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(data)}).then(r=>r.json()),
  removeTask: (uuid) => fetch('/api/remove-task',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({uuid})}).then(r=>r.json())
}

const el = {
  residentBody: document.getElementById('residentBody'),
  scheduledBody: document.getElementById('scheduledBody'),
  refresh: document.getElementById('refresh'),
  addResident: document.getElementById('addResident'),
  addScheduled: document.getElementById('addScheduled'),
  runtime: document.getElementById('runtime'),
  toast: document.getElementById('toast'),
  modal: document.getElementById('modal'),
  closeModal: document.getElementById('closeModal'),
  modalTitle: document.getElementById('modalTitle'),
  taskForm: document.getElementById('taskForm'),
  cronField: document.getElementById('cronField'),
  logDirField: document.getElementById('logDirField'),
  cancelBtn: document.getElementById('cancelBtn')
}

let data = []
let editMode = false

function showToast(msg){
  el.toast.textContent = msg
  el.toast.className = 'fixed bottom-4 right-4 px-3 py-2 bg-black text-white rounded shadow'
  el.toast.classList.remove('hidden')
  setTimeout(()=>el.toast.classList.add('hidden'),2000)
}

function render(){
  const resident = data.filter(x=>x.type===1)
  const scheduled = data.filter(x=>x.type===2)
  el.residentBody.innerHTML = resident.map(row=>{
    const follow = row.run?'<span class="badge badge-green">开启</span>':'<span class="badge badge-yellow">关闭</span>'
    const status = row.status===1?'<span class="badge badge-green">运行</span>':'<span class="badge badge-yellow">暂停</span>'
    return `<tr class="border-t">
      <td class="py-2 px-3">${row.jobName}</td>
      <td class="py-2 px-3">${follow}</td>
      <td class="py-2 px-3">${status}</td>
      <td class="py-2 px-3 flex gap-2">
        <button class="px-2 py-1 border rounded" data-act="stopJob" data-id="${row.uuid}">停止</button>
        <button class="px-2 py-1 bg-blue-600 text-white rounded" data-act="runJob" data-id="${row.uuid}">启动</button>
        <button class="px-2 py-1 border rounded" data-act="edit" data-id="${row.uuid}">编辑</button>
        <button class="px-2 py-1 border rounded" data-act="remove" data-id="${row.uuid}">删除</button>
      </td>
    </tr>`
  }).join('')
  el.scheduledBody.innerHTML = scheduled.map(row=>{
    const open = row.run?'<span class="badge badge-green">开启</span>':'<span class="badge badge-yellow">关闭</span>'
    return `<tr class="border-t">
      <td class="py-2 px-3">${row.jobName}</td>
      <td class="py-2 px-3">${open}</td>
      <td class="py-2 px-3 flex gap-2">
        <button class="px-2 py-1 ${row.run?'border':'bg-yellow-500 text-white'} rounded" data-act="toggle" data-id="${row.uuid}" data-run="${!row.run}">${row.run?'停止':'启动'}</button>
        <button class="px-2 py-1 bg-blue-600 text-white rounded" data-act="runTask" data-id="${row.uuid}">运行</button>
        <button class="px-2 py-1 border rounded" data-act="edit" data-id="${row.uuid}">编辑</button>
        <button class="px-2 py-1 border rounded" data-act="remove" data-id="${row.uuid}">删除</button>
      </td>
    </tr>`
  }).join('')
}

function bindTableActions(){
  function handler(e){
    const act = e.target.getAttribute('data-act')
    if(!act) return
    const id = e.target.getAttribute('data-id')
    if(act==='runJob') api.runJob(id).then(r=>{showToast(r.message); load()})
    else if(act==='stopJob') api.stopJob(id).then(r=>{showToast(r.message); load()})
    else if(act==='toggle') {
      const run = e.target.getAttribute('data-run')==='true'
      api.openCloseTask(id, run).then(r=>{showToast(r.message); load()})
    }
    else if(act==='runTask') api.runTask(id).then(r=>{showToast(r.message)})
    else if(act==='edit') openEdit(id)
    else if(act==='remove') api.removeTask(id).then(r=>{showToast(r.message); load()})
  }
  el.residentBody.onclick = handler
  el.scheduledBody.onclick = handler
}

function openAdd(type){
  editMode = false
  el.modalTitle.textContent = type===1?'任务新增':'任务新增'
  el.taskForm.uuid.value = ''
  el.taskForm.jobName.value = ''
  el.taskForm.type.value = String(type)
  el.taskForm.spec.value = '* * * * *'
  el.taskForm.binPath.value = ''
  el.taskForm.dir.value = ''
  el.taskForm.paramsStr.value = ''
  el.taskForm.outputType.value = '1'
  el.taskForm.outputPath.value = '/tmp'
  toggleFields()
  showModal()
}

function openEdit(id){
  editMode = true
  const row = data.find(x=>x.uuid===id)
  el.modalTitle.textContent = '任务调整'
  el.taskForm.uuid.value = row.uuid
  el.taskForm.jobName.value = row.jobName
  el.taskForm.type.value = String(row.type)
  el.taskForm.spec.value = row.spec||''
  el.taskForm.binPath.value = row.binPath||''
  el.taskForm.dir.value = row.dir||''
  el.taskForm.paramsStr.value = (row.params||[]).join(' ')
  el.taskForm.outputType.value = String((row.options||{}).outputType||1)
  el.taskForm.outputPath.value = (row.options||{}).outputPath||'/tmp'
  toggleFields()
  showModal()
}

function showModal(){
  el.modal.classList.remove('hidden')
  el.modal.classList.add('flex')
}
function hideModal(){
  el.modal.classList.add('hidden')
  el.modal.classList.remove('flex')
}
function toggleFields(){
  const type = Number(el.taskForm.type.value)
  el.cronField.classList.toggle('hidden', type!==2)
  const ot = Number(el.taskForm.outputType.value)
  el.logDirField.classList.toggle('hidden', ot!==2)
}

function serializeForm(){
  const params = (el.taskForm.paramsStr.value||'').trim()
  const arr = params?params.split(/\s+/):[]
  const data = {
    uuid: el.taskForm.uuid.value,
    jobName: el.taskForm.jobName.value,
    type: Number(el.taskForm.type.value),
    spec: el.taskForm.spec.value,
    binPath: el.taskForm.binPath.value,
    dir: el.taskForm.dir.value,
    run: false,
    params: arr,
    options: {
      maxFailures: 5,
      outputType: Number(el.taskForm.outputType.value),
      outputPath: el.taskForm.outputPath.value
    }
  }
  return data
}

function load(){
  api.jobList().then(r=>{
    data = r.message||[]
    render()
  })
}

function tick(){
  api.runInfo().then(r=>{
    el.runtime.textContent = `启动于 ${r.start} 已运行 ${r.runTime}`
  })
}

el.refresh.onclick = ()=>load()
el.addResident.onclick = ()=>openAdd(1)
el.addScheduled.onclick = ()=>openAdd(2)
el.closeModal.onclick = ()=>hideModal()
el.cancelBtn.onclick = ()=>hideModal()
el.taskForm.type.onchange = toggleFields
el.taskForm.outputType.onchange = toggleFields
el.taskForm.onsubmit = (e)=>{
  e.preventDefault()
  const payload = serializeForm()
  api.saveTask(payload).then(r=>{
    showToast(r.message)
    hideModal()
    load()
  })
}

bindTableActions()
load()
tick()
setInterval(tick,1000)