# app_template

GMSSH 微应用模板仓库。**每个微应用一个分支**，分支根目录直接放该应用的完整代码（前端 + 后端 + 打包脚本），不做嵌套目录。

## 分支目录

| 分支 | 应用 | 目录结构 | 说明 |
|---|---|---|---|
| [portMap](../../tree/portMap) | 端口地图 | `web/` + `backend/` | 可视化查看服务器当前监听的 TCP/UDP 端口，按地址分组画成节点图，可点选查看进程、用户、命令行等归属信息 |
| [systemScore](../../tree/systemScore) | 系统跑分 | `frontend/` + `backend/` | 一键跑分，输出 CPU 单核/多核、内存、磁盘、持续负载 5 个维度的综合性能分（满分 10000） |
| [sshFootprint](../../tree/sshFootprint) | SSH 安全足迹 | `web/` + `backend/` | 只读统计本机 SSH 认证活动：失败/成功次数、时间分布、可疑来源 IP 排行，附原始日志取证 |
| [attackRadar](../../tree/attackRadar) | 攻击雷达 | `web/` + `backend/` | 3D 地球 / 2D 平面双视图展示本机被攻击态势；另接 Cloudflare Radar 展示全球攻击热区与路径 |

## 通用约定

所有分支遵循同一套 GMSSH 微应用规范，换应用只换业务内容，框架部分完全一致。

### 后端：纯 CLI，无常驻进程

宿主每次调用都执行一次二进制，它打印一个 JSON 信封后立刻退出。没有 Unix Socket、没有 daemon、没有 RPC 框架。

```
/.__gmssh/plugin/liteapp/{user_id}/{app_name}/app/bin/main <命令> [参数] --json
```

```go
command.Define("<命令名>").
    Summary("cmd.xxx.summary").
    Args(
        command.Arg{Name: "action", Type: command.TypeString, Default: "scan"},
        // ...
    ).
    Examples("<命令名> --json").
    Handle(Run).
    Register()
```

### 前端：`$gm.request` + `exec_shell`

前端不直接连后端，而是让宿主代为执行命令：

```js
const bin = `/.__gmssh/plugin/liteapp/${user_id}/${app_name}/app/bin/main`
const cmd_string = `'${bin}' <命令名> --action scan --json`

await window.$gm.request('/api/command/exec_shell', {
  method: 'post',
  data: { cmd_string, timeout: '30', cwd: '', user: '' },
})
```

返回值是 JSON 信封，`data` 字段才是业务数据：

```json
{ "ok": true, "data": { ... } }
```

`user_id` / `app_name` 从应用自己的 `liteapp.json` 读取（平台创建应用时写入），不要在代码里改。

### 打包：两个独立产物

```bash
./pack.sh
```

| 产物 | 内容 | 部署位置 |
|---|---|---|
| `*_dist.tar.gz`（前端） | 静态文件，解压即用 | `app/www/` |
| `backend_dist.tar.gz` | 平坦结构：`app/bin/main`、`install.sh`、`config/`、`data/`、`logs/`、`tmp/` | 应用根目录 |

前端**不要**打成带顶层 `app/` 的嵌套结构；macOS 打包要带 `COPYFILE_DISABLE=1`，否则会混进 `._*` 元数据文件。

## 新增一个微应用

```bash
# 1. 从 main 拉一个新分支，分支名 = 应用标识
git clone git@github.com:GMSSH/app_template.git
cd app_template
git checkout -b myNewApp origin/main

# 2. 在这个分支根目录直接放应用代码
#    web/ 或 frontend/  前端
#    backend/           Go CLI
#    pack.sh            打包脚本
#    README.md          本应用说明（标题、用途、请求示例）

# 3. 提交推送
git add -A
git commit -m "feat: add myNewApp GMSSH micro-app"
git push -u origin myNewApp
```

### 每个应用分支的 README 建议内容

在下表的模板里把占位换成实际值，别人一眼就能知道这个应用干什么、怎么调：

```markdown
# 应用名 (appName) · 极目桌面微应用

一句话说明这个应用解决什么问题。

## 接口请求

| 项 | 值 |
|---|---|
| 应用标识 | `user_id: jg` / `app_name: myNewApp` |
| 二进制路径 | `/.__gmssh/plugin/liteapp/jg/myNewApp/app/bin/main` |
| 命令名 | `my_command` |

```js
await window.$gm.request('/api/command/exec_shell', {
  method: 'post',
  data: {
    cmd_string: "'/.__gmssh/plugin/liteapp/jg/myNewApp/app/bin/main' my_command --action scan --json",
    timeout: '30', cwd: '', user: '',
  },
})
```

返回：

```json
{ "ok": true, "data": { "field": "value" } }
```

## 本地联调

```bash
cd web && node dev-server.mjs   # http://dev.adde.cn:5185
```
```

## 相关文档

- GMSSH 纯 CLI 架构与命令定义：见各分支 `backend/internal/command/`
- 前端 SDK（`$gm.request`、`$gm.theme`、`$gm.getRectSize`）：`gm-app-sdk` 包
