import axios from 'axios'
const instanceAxios = axios.create({ baseURL: '/api/' })
export function getJobList() { return instanceAxios.get('job-list') }
export function runInfo() { return instanceAxios.get('run-info') }
export function saveTask(data: any) { return instanceAxios.post('save-task', data) }
export function removeTask(jobId: any) { return instanceAxios.post('remove-task', { jobId }) }
export function runJob(jobId: any) { return instanceAxios.post('run-job-resident-task', { jobId }) }
export function stopJob(jobId: any) { return instanceAxios.post('stop-job-resident-task', { jobId }) }
export function runOpenCloseTask(jobId: any, open: boolean) { return instanceAxios.post('open-close-task', { uuid: jobId, run: open }) }
export function runTask(jobId: any) { return instanceAxios.post('run-task', { taskId: jobId }) }
export function restartJob(jobId: any) { return instanceAxios.post('restart-job-resident-task', { jobId }) }
export function getJobLogList() { return instanceAxios.get('job-log-list') }
export function getJobLog(jobId: any, lines = 200, bytes = 0) { return instanceAxios.get('job-log', { params: { jobId, lines, bytes } }) }
export function downloadJobLog(jobId: any) { return instanceAxios.get('job-log-download', { params: { jobId }, responseType: 'blob' }) }
