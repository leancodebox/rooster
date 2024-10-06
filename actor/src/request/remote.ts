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

export function saveTask(params: any) {
    return instanceAxios.post("run-save", {
        uuid: params.uuid,
        jobName: params.jobName,
        type: params.type,
        run: params.run,
        binPath: params.binPath,
        params: params.params,
        dir: params.dir,
        spec: params.spec,
        options: params.options,
    })
}


export function removeTask(uuid: any) {
    return instanceAxios.post("remove-task", {
        uuid: uuid,
    })
}

export function runOpenCloseTask(uuid: any, run: boolean) {
    return instanceAxios.post("run-open-close-task", {
        uuid: uuid,
        run: run,
    })
}
