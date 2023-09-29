import axios from "axios"
import {createDiscreteApi,} from "naive-ui";

const {message} = createDiscreteApi(
    ["message"],
);


const instanceAxios = axios.create({
    baseURL: import.meta.env.VITE_DEV_API_HOST,
    timeout: 10 * 1000,
    headers: {}
})

instanceAxios.interceptors.request.use(config => {
    return config;
});


const success = 0
const fail = 1

export function getJobList() {
    return instanceAxios.get("job-list")
}


export function runJob(jobId: any) {
    return instanceAxios.post("run-job", {
        jobId: jobId
    })
}


export function stopJob(jobId: any) {
    return instanceAxios.post("stop-job", {
        jobId: jobId
    })
}

export function runTask(taskId: any) {
    return instanceAxios.post("run-task", {
        taskId: taskId
    })
}
