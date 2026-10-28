# SSH 安全足迹 (sshFootprint) · 极目桌面微应用

读取 Linux 服务器本地 SSH 认证日志，把「谁在尝试登录这台机器」变成一份可核对的可视化报告：失败/成功次数、时间分布、可疑来源 IP 排行，以及每条判定的原始日志取证。

**只读**：不访问外部网络、不装软件、不改 SSH 配置、不动防火墙。

## 接口请求

前端不直连后端，统一让宿主执行命令再解析 JSON。

| 项 | 值 |
|---|---|
| 应用标识 | `user_id: jg` / `app_name: sshFootprint` |
| 二进制路径 | `/.__gmssh/plugin/liteapp/jg/sshFootprint/app/bin/main` |
| 命令名 | `ssh_footprint` |

```js
const bin = '/.__gmssh/plugin/liteapp/jg/sshFootprint/app/bin/main'

await window.$gm.request('/api/command/exec_shell', {
  method: 'post',
  data: {
    cmd_string: `'${bin}' ssh_footprint --action scan --range 24h --json`,
    timeout: '30', cwd: '', user: '',
  },
})
```

### 命令一览

| `--action` | 参数 | 用途 |
|---|---|---|
| `probe` | 无 | 探测可用日志源、权限、时区 |
| `scan` | `--range 24h\|7d\|30d` | 解析日志并返回统计结果 |

### 返回结构

统一信封，业务数据在 `data` 里：

```json
{
  "ok": true,
  "data": {
    "range": "24h",
    "scan_time": "2026-10-26 11:53:43",
    "timezone": "CST",
    "window_start": "2026-10-25 11:53:43",
    "window_end": "2026-10-26 11:53:43",
    "count_method": "按认证事件计数：一次认证尝试只算 1 次……",
    "active_source": "systemd-journal (ssh.service)",
    "summary": { "total_failed": 1284, "total_success": 42, "unique_ips": 30 },
    "buckets": [
      { "label": "11:00", "start_time": 1761, "end_time": 1762, "failed_count": 63, "success_count": 1 }
    ],
    "ip_list": [
      {
        "ip": "212.18.52.25",
        "failed_count": 128, "success_count": 0,
        "users": ["admin", "root"],
        "first_seen": "2026-10-26 23:01:55", "last_seen": "2026-10-26 23:41:02",
        "risk_level": "high",
        "risk_tags": ["高频登录尝试", "多账号尝试"],
        "risk_reasons": ["5分钟内连续认证失败超过20次", "尝试了 3 个不同用户名 (admin, root, test)"],
        "logs": ["Oct 26 23:41:02 srv sshd[1283]: Failed password for invalid user admin from 212.18.52.25 port 48102 ssh2"]
      }
    ]
  }
}
```

### 统计口径（与日志行数核对时必读）

`failed_count` **不是**日志行数。一次 SSH 认证尝试在 sshd 里会写 2~3 行（`Invalid user`、`Failed password`、`Connection closed [preauth]`）。

| 计数项 | 定义 |
|---|---|
| 失败 | `Failed password` 或 `Invalid user` 行，同一连接去重后算 1 次 |
| 成功 | `Accepted password` 或 `Accepted publickey` 行 |
| 不计入 | `Connection closed`、`Received disconnect`、banner exchange 等连接层日志 |

所以 `grep -c` 出来的行数一定大于接口值，这是设计如此。

### 服务器上直接验证

```bash
BIN=/.__gmssh/plugin/liteapp/jg/sshFootprint/app/bin/main

$BIN ssh_footprint --action probe --json | jq '.data'
$BIN ssh_footprint --action scan --range 24h --json | jq '.data.summary'
$BIN ssh_footprint --action scan --range 24h --json | jq '.data.ip_list[:5]'

# 口径自校验：逐 IP 求和应等于汇总
$BIN ssh_footprint --action scan --range 24h --json | jq '{
  total_failed: .data.summary.total_failed,
  sum_from_ip_list: ([.data.ip_list[].failed_count] | add),
  unique_ips: .data.summary.unique_ips,
  ip_list_len: (.data.ip_list | length)
}'
```

---

## 核心特性

- **纯离线安全设计**：完全不访问外部网络，不调用第三方 IP 库或安全接口；不修改系统 SSH、防火墙或配置。
- **全日志源兼容**：自动优先适配 `journalctl` (systemd Journal)，并自动回退检测 `/var/log/auth.log` (Debian/Ubuntu)、`/var/log/secure` (RHEL/CentOS/Rocky) 及历史轮转日志与 `last` / `lastb`。
- **智能事件归并与去重**：精准合并同一认证流程中的 `Invalid user` 和 `Failed password`，过滤 SSH 握手断开误报，保证认证次数严谨真实。
- **轻量异常线索识别**：
  - **高频登录尝试**：短时间（如 5 分钟内 ≥ 20 次）密集失败。
  - **多账号暴力猜解**：同一 IP 反复尝试 root、admin、test 等不同用户名。
  - **失败后成功核对**：某 IP 连续失败后出现成功认证，提示用户重点核对。
- **精致 Apple 风格设计**：
  - 红绿双色细柱状图（失败红色 `#E66B67`，成功绿色 `#28A879`），圆角细柱、同一基准纵轴、疏密有致，悬浮 Tooltip 联动。
  - 重点关注来源 IP 列表与右侧滑出抽屉，可查看原始审计日志证据。
  - 自适应微应用窗口缩放（无固定最大宽度），界面统一亮色。
- **GMSSH 正确纯 CLI 架构**：每次通过宿主 `/api/command/exec_shell` 执行后端 `app/bin/main`，输出标准 JSON 信封后立即退出，无常驻进程、无 socket。

---

## 前端（原生 HTML + CSS + JS，零构建）

前端在 `web/`，**不需要 npm、不需要构建**，改完直接部署。

```
web/
├── index.html        结构
├── css/base.css      主题变量 + Apple 滚动条
├── css/app.css       全部样式
├── js/gm-sdk.js      宿主 SDK（从 npm 包直接拷出，勿改）
├── js/api.js         后端调用（$gm.request → exec_shell → mock 回退）
├── js/mock.js        模拟数据
├── js/chart.js       红绿柱状图（原生 SVG DOM）
├── js/app.js         主逻辑（状态 + 渲染 + 抽屉）
├── dev-server.mjs    本地联调服务器（静态 + exec_shell 桥接）
└── liteapp.json      应用标识
```

本地联调：

```bash
cd web
node dev-server.mjs      # → http://dev.adde.cn:5185
```

`dev-server.mjs` 会把前端的 `exec_shell` 请求转成 `go run ./cmd/main ssh_footprint ...`，
所以在浏览器（无宿主）里也能拿到**真实日志数据**，不是 mock。

直接部署：把 `web_dist.tar.gz` 解压到 `app/www/` 即可，无需任何构建步骤。

---

## 项目结构

```
SSH 入侵足迹/
├── web/                       # 前端（原生 HTML/CSS/JS，零构建）
│   ├── index.html             # 页面结构
│   ├── css/base.css           # 主题变量 + Apple 滚动条
│   ├── css/app.css            # 全部样式
│   ├── js/gm-sdk.js           # 宿主 SDK
│   ├── js/api.js              # exec_shell 请求封装
│   ├── js/mock.js             # 模拟数据
│   ├── js/chart.js            # 红绿柱状图
│   ├── js/app.js              # 主逻辑
│   ├── dev-server.mjs         # 本地联调服务器（不打包）
│   └── liteapp.json           # 应用标识
├── backend/                   # 纯 CLI Go 后端（gmssh-cli-plugin 归档模板）
│   ├── cmd/main/main.go       # 入口：blank-import internal/biz/footprint
│   └── internal/biz/footprint # 业务核心：探测、解析、去重、聚合、异常规则
├── pack.sh                    # 一键打包
└── README.md
```

---

## 本地开发与联调

### 1. 前端开发服务

```bash
cd web
node dev-server.mjs
```

- 监听 `http://0.0.0.0:5185`（端口固定 5185）。
- 在浏览器或极目桌面中访问：**`http://dev.adde.cn:5185`**。
- `dev-server.mjs` 把前端的 `exec_shell` 请求桥接成 `go run ./cmd/main ssh_footprint ...`，
  所以浏览器里拿到的也是真实日志数据；仅在桥接不可用时才回退到内置模拟数据，并在顶部显示「模拟数据」角标。

### 2. 后端 CLI 测试

```bash
cd backend

# 运行单元测试（覆盖 parser、去重、聚合和异常规则识别）
go test -v ./internal/biz/footprint/...

# 检查日志源及权限
go run ./cmd/main ssh_footprint --action probe --json

# 指定时间范围扫描（支持 24h、7d、30d）
go run ./cmd/main ssh_footprint --action scan --range 24h --json
go run ./cmd/main ssh_footprint --action scan --range 7d --json
go run ./cmd/main ssh_footprint --action scan --range 30d --json
```

---

## 统计口径（与日志行数核对时必读）

扫描结果按 **认证事件** 计数，不是按日志行计数，两者天然不等：

| 计入失败 | 计入成功 | 不计入 |
|---|---|---|
| `Failed password for ...` | `Accepted password for ...` | `Connection closed` / `Received disconnect` |
| `Invalid user xxx from ...` | `Accepted publickey for ...` | `banner exchange` / `Disconnected from` |
| | | 未能解析出时间戳的行 |

- 「值得核对的成功登录」只在**连续失败的性质像猜口令**时才给出：失败里出现过不存在账号（`Invalid user`），或换了 3 个以上账号。同一账号反复输错再登入属于本人重试，标为「重试后登录成功」（low），不报警。
- 一次爆破尝试通常产生 `Invalid user` + `Failed password` + `Connection closed` 三行，接口只算 **1 次失败**。
- 失败口径包含 `Invalid user`：爆破不存在的账号时某些发行版只记这一行。
- 时间窗口的上下界会用 `window_start` / `window_end` 字段明确返回，且 `journalctl --since` 直接使用 `window_start` 的绝对时间，与本地过滤边界完全一致，不再有滚动窗口漂移。
- `count_method` 字段携带完整口径说明，UI 悬停在日志源标签上可看到。

**核对方式**（不要在终端用 `grep -c` 数行数）：

```bash
BIN=/.__gmssh/plugin/liteapp/jg/sshFootprint/app/bin/main

# 1. 看窗口与口径
$BIN ssh_footprint --action scan --range 24h --json | jq '{window_start, window_end, count_method}'

# 2. 逐 IP 求和必须等于汇总
$BIN ssh_footprint --action scan --range 24h --json | jq '{
  total_failed:  .data.summary.total_failed,
  sum_failed:    ([.data.ip_list[].failed_count]   | add),
  total_success: .data.summary.total_success,
  sum_success:   ([.data.ip_list[].success_count]  | add),
  unique_ips:    .data.summary.unique_ips,
  ip_list_len:   (.data.ip_list | length)
}'

# 3. 单个 IP 的失败数与日志逐条核对（窗口内、排除连接层）
$BIN ssh_footprint --action scan --range 24h --json | jq -r '.data.ip_list[0].ip' | \
  xargs -I{} journalctl -u ssh -u sshd --since "<window_start>" --until "<window_end>" \
  | grep -F {} | grep -cE 'Failed password|Invalid user'
```

---

## 打包与发布

在微应用根目录执行打包脚本：

```bash
chmod +x pack.sh
./pack.sh
```

打包成功后将在根目录生成两个归档产物：
1. **`frontend_dist.tar.gz`**：前端静态文件及 `liteapp.json`，部署在宿主 `app/www/`。
2. **`backend_dist.tar.gz`**：后端 Linux amd64 纯 CLI 二进制（已剔除符号表、轻量编译）及配置，部署在宿主 `app/bin/`。

在极目 Linux 桌面中，导入上述两个包即可完成安装与更新。
