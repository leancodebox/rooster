<div align="center">
  <img src="./cmd/roostertray/icon_mac_256.png" alt="Rooster Logo" width="120" />
  <h1>Rooster</h1>
  <p><strong>一个兼备程序唤起和任务调度的现代化管理工具</strong></p>
</div>

---

**Rooster** 是一款跨平台的任务调度与程序守护工具。它不仅支持基于 cron 表达式的定时任务，还支持常驻任务的进程守护。更重要的是，它提供了一个基于 React + TailwindCSS 的现代化 Web 控制台 (Dashboard)，让你随时随地掌控你的任务状态。

## ✨ 特性

- 🔄 **进程守护**：常驻任务异常退出自动重启。
- ⏱️ **定时任务**：支持标准的 Crontab 表达式执行计划任务。
- 🌐 **Web 面板**：内置现代化 Web Dashboard，可视化管理所有任务。
- 🖥️ **跨平台**：支持 Windows / macOS / Linux，并提供原生的系统托盘 (System Tray) 管理。
- 📝 **日志追踪**：支持 Web 终端实时流式输出任务执行日志。

## 🚀 快速开始

### 方式一：完整版 (Rooster 包含 Web Dashboard 和系统托盘)

自行编译完整版，环境需要 `Go 1.21+` 和 `Node.js`。

```shell
# 1. 克隆代码
git clone https://github.com/leancodebox/rooster.git 
cd rooster 

# 2. 编译前端面板 (React)
cd actorv3 
npm install
npm run build
cd ..

# 3. 编译 Go 核心与托盘程序
go build -o rooster ./cmd/roostertray
```

### 方式二：Rooster-CLI (纯命令行版)

如果你只需要纯净的后端调度功能，可以通过以下命令快速安装：

```shell
go install github.com/leancodebox/rooster-cli@latest 
```

执行 `rooster-cli` 后会判断当前目录是否存在 `jobConfig.json`。如果没有，会提示是否生成默认配置。生成配置后，修改完毕再次执行 `rooster-cli` 即可运行任务调度。

## ⚙️ 配置文件说明 (`jobConfig.json`)

| 键名 | 类型 | 说明 |
| :--- | :---: | :--- |
| `config` | `object` | 基础配置 |
| `config.dashboard` | `object` | Web 面板配置 |
| `config.dashboard.port` | `int` | 面板监听端口（小于 1 不开启，CLI 版无论是否配置都不会开启） |
| `residentTask` | `array` | **常驻任务列表**（守护进程） |
| `residentTask[].jobName` | `string` | 任务名称 |
| `residentTask[].binPath` | `string` | 可执行文件路径，或环境变量中的命令 |
| `residentTask[].params` | `array` | 执行参数列表 `["arg1", "arg2"]` |
| `residentTask[].dir` | `string` | 任务的工作目录 |
| `residentTask[].run` | `bool` | 是否启用该任务（可在 Web 面板中切换） |
| `residentTask[].options` | `object` | 高级选项 |
| `residentTask[].options.outputType` | `int` | 输出模式：`0` 标准输出，`1` 文件输出 |
| `residentTask[].options.outputPath` | `string` | 日志输出路径 |
| `scheduledTask` | `array` | **定时任务列表** (Cron) |
| `scheduledTask[].jobName` | `string` | 任务名称 |
| `scheduledTask[].binPath` | `string` | 可执行文件路径，或环境变量中的命令 |
| `scheduledTask[].params` | `array` | 执行参数列表 `["arg1", "arg2"]` |
| `scheduledTask[].dir` | `string` | 任务的工作目录 |
| `scheduledTask[].spec` | `string` | Crontab 格式的调度周期，例如 `* * * * *` |
| `scheduledTask[].run` | `bool` | 是否启用该任务 |
| `scheduledTask[].options` | `object` | 同常驻任务选项配置 |

---
<div align="center">
  <sub>Built with ❤️ by Leancodebox</sub>
</div>

