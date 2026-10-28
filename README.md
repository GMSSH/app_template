# 系统跑分 (systemScore) · 极目桌面微应用

给服务器做一次体检式跑分：CPU 单核 / 多核、内存带宽、磁盘 IO、持续负载 5 个维度各打分，加权成满分 10000 的综合性能分，并留历史记录便于对比升级前后的变化。

可指定次数做真机校准（`calibrate`），让参考基准贴合这台机器。

## 接口请求

前端不直连后端，统一让宿主执行命令再解析 JSON。

| 项 | 值 |
|---|---|
| 应用标识 | `user_id: jg` / `app_name: systemScore` |
| 二进制路径 | `/.__gmssh/plugin/liteapp/jg/systemScore/app/bin/main` |
| 命令名 | `score`（另有 probe / calibrate / check / history / overview） |

```js
const bin = '/.__gmssh/plugin/liteapp/jg/systemScore/app/bin/main'

await window.$gm.request('/api/command/exec_shell', {
  method: 'post',
  data: {
    cmd_string: `'${bin}' score --json --lang 'zh-CN'`,
    timeout: '30', cwd: '', user: '',
  },
})
```

### 命令一览

| 命令 | 参数 | 用途 |
|---|---|---|
| `score` | 无 | 完整跑分，返回综合分与各维度明细 |
| `probe` | `--key cpu\|mem\|disk`，可选 `--dir`、`--force` | 单独探测某一维度 |
| `calibrate` | `--n <次数>`，可选 `--dir` | 多次采样做真机校准 |
| `check` | 无 | 环境预检（是否适合跑分） |
| `history` | 无 | 历史跑分记录 |
| `overview` | 无 | 机器概况（CPU 型号 / 内存 / OS） |

`probe --force` 会跳过负载预检（`loadavg` / PSI）强制跑分——正常情况请先在 `check` 确认环境空闲，否则分数没有可比性。

### 返回结构

统一信封，业务数据在 `data` 里：

```json
{
  "ok": true,
  "data": {
    "cpu": 3980, "mem": 2760, "disk": 3110, "total": 9850,
    "parts": [
      { "key": "cpu", "score": 3980, "weight": 4000, "value": 12843.2, "unit": "ops", "percent": 99 }
    ]
  }
}
```

### 服务器上直接验证

```bash
BIN=/.__gmssh/plugin/liteapp/jg/systemScore/app/bin/main

$BIN check --json | jq '.data'
$BIN score --json | jq '.data | {total, cpu, mem, disk}'
$BIN history --json | jq '.data'
```

## 架构说明

- **前端**：Vue 3 + Vite（`frontend/`），三环 SVG 仪表盘展示综合分。
- **后端**：Go 纯 CLI（`backend/`），命令执行一次输出 JSON 后即刻退出。

### 评分维度与权重

| 维度 | 权重 | 说明 |
|---|---|---|
| CPU | 4000 | 单核 1800 + 多核 2500（按核数折算） |
| 内存 | 3000 | 带宽 + 延迟 |
| 磁盘 | 3000 | 顺序读写 + 随机 IO |
| 持续 | 700 | 长时间负载下的稳定性 |

满分 10000。**未做 `calibrate` 时分数只有相对意义**，不能跨机型直接比较。

## 目录结构

```
系统跑分/
├── frontend/                  # 前端（Vue 3 + Vite）
│   ├── src/
│   │   └── components/RunScorePanel.vue
│   └── public/liteapp.json
├── backend/                   # Go 纯 CLI
│   ├── cmd/main/main.go
│   ├── internal/biz/score/    # score / probe / calibrate / check / history / overview
│   └── internal/{probe,guard,bench,runner,score}/
├── pack.sh
└── README.md
```

## 依赖提醒

前端需要 `npm install && npm run dev`；后端改完要 `make linux-amd64` 才会产出可部署二进制。
