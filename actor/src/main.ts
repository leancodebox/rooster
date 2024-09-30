// import './assets/main.css'

import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import router from './router'

const app = createApp(App)

let title = "";  // 用于临时存放原来的title内容
window.onblur = function () {
    // onblur时先存下原来的title,再更改title内容
    title = document.title;
    document.title = "布谷~布谷~";
};
window.onfocus = function () {
    // onfocus时原来的title不为空才替换回去
    // 防止页面还没加载完成且onblur时title=undefined的情况
    if (title) {
        document.title = title;
    }
}

app.use(createPinia())
app.use(router)

app.mount('#app')
