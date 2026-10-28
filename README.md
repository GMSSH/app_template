# 端口地图 (portMap) · 极目桌面微应用

一眼看清这台服务器现在开了哪些端口、谁在监听。把 `ss\ -lntup` 的扁平行按监听地址分组，画成可点选的节点图，点开任一端口能看到占用的进程、用户、完整命令行和工作目录。

**只读**：主流程只做一次固定查询，不执行任意 shell，不接受把搜索词拼进命令。

## 接口请求

前端不直连后端，统一让宿主执行命令再解析 JSON。

| 项 | 值 |
|---|---|
| 应用标识 | `user_id: jg` / `app_name: portMap` |
| 二进制路径 | `/.__gmssh/plugin/liteapp/jg/portMap/app/bin/main` |
| 命令名 | `ports` |

```js
const bin = '/.__gmssh/plugin/liteapp/jg/portMap/app/bin/main'

await window.$gm.request('/api/command/exec_shell', {
  method: 'post',
  data: {
    cmd_string: `'${bin}' ports --action scan --json`,
    timeout: '30', cwd: '', user: '',
  },
})
```

### 命令一览

| `--action` | 参数 | 用途 |
|---|---|---|
| `scan`（默认） | 无 | 列出当前监听的 TCP/UDP 端口 |
| `kill` | `--pid <n>` | 结束指定进程（可选能力） |
| `release` | `--port <n> --proto <tcp\|udp>` | 释放占用该端口的进程（可选能力） |

### 返回结构

统一信封，业务数据在 `data` 里，是端口记录数组：

```json
{
  "ok": true,
  "data": [
    {
      "protocol": "tcp",
      "port": 22,
      "address": "0.0.0.0",
      "state": "LISTEN",
      "pid": 812,
      "processName": "sshd",
      "user": "root",
      "command": "sshd: /usr/sbin/sshd -D",
      "workingDirectory": "/",
      "ownerType": "systemd",
      "ownerName": "ssh.service",
      "scannedAt": "2026-10-26 11:53:43"
    }
  ]
}
```

### 搜索行为

搜索是**纯前端**过滤，不重新执行命令、不把输入拼进 shell：

- 只接受 1–65535 的端口号，精确匹配
- 输入 `80` 不会匹配 `8080` 或 `18080`
- 空搜索显示全部端口

### 服务器上直接验证

```bash
BIN=/.__gmssh/plugin/liteapp/jg/portMap/app/bin/main

$BIN ports --action scan --json | jq '.data | length'
$BIN ports --action scan --json | jq '.data[] | select(.port == 22)'
```

## 架构说明

- **前端**：Vue 3 + Vite（`web/`）。用 Vue Flow 画节点图，搜索时收窄到匹配项并自动适配视口。
- **后端**：Go 纯 CLI（`backend/`），`ports` 命令执行一次后即刻退出，无常驻进程、无 Unix Socket。

## 目录结构

```
端口地图/
├── web/                       # 前端（Vue 3 + Vite）
│   ├── src/
│   │   ├── App.vue
│   │   ├── api/               # exec_shell 封装
│   │   ├── components/        # PortGraph / DetailPanel / StatCard
│   │   ├── composables/       # useApp（窗口尺寸）
│   │   ├── locales/           # 中英文案
│   │   └── types/
│   └── public/liteapp.json    # 宿主应用身份
├── backend/                   # Go 纯 CLI
│   ├── cmd/main/main.go
│   └── internal/biz/ports/    # scan / kill / release
└── README.md
```

## 依赖提醒

前端在 `web/`，需要 `npm install && npm run dev`；后端改完要 `make linux-amd64` 才会产出可部署二进制。
