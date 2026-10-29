# 站长百宝箱 (webmasterToolbox) · 极目桌面微应用

站长、运维每天真正会用到的检测工具合集：查一次 DNS、Ping 一下、看端口通不通、盯一眼证书还剩几天、查域名什么时候注册的；再顺手带上 JSON 格式化、Base64、哈希、时间戳这些文本小工具。

界面是一个首页宫格 + 10 个工具页，全部**真实数据**：网络类走服务器本机的 `dig` / `ping` / `curl` / `openssl` / WHOIS(TCP 43)，文本类在浏览器本地算，没有任何 mock。

## 应用标识

| 项 | 值 |
|---|---|
| 应用名称 | 站长百宝箱 |
| appid | `user_id: jg` / `app_name: webmasterToolbox` |
| 部署路径 | `/.__gmssh/plugin/liteapp/jg/webmasterToolbox/` |
| 二进制路径 | `/.__gmssh/plugin/liteapp/jg/webmasterToolbox/app/bin/main` |
| 命令名 | `toolbox`（一个命令，按 `--action` 分发） |
| 前端技术 | 原生 HTML5 / CSS3 / ES Modules，**零构建** |

## 功能

### 网络检测（后端真实执行）

| 工具 | action | 数据来源 | 说明 |
|---|---|---|---|
| DNS 查询 | `dns` | `dig` → `nslookup` → `getent` 逐级降级 | A / AAAA / CNAME / MX / TXT / NS，`ALL` 一次性查全并按类型分组；支持指定 DNS 服务器 |
| Ping 检测 | `ping` | 系统 `ping` | 5 次探测，丢包位置来自 `icmp_seq` 真实空缺，TTL / 字节数取回包原文 |
| 端口检测 | `port` | 纯 TCP 连接 | 21/22/80/443/3306/6379/8080 并发探测，只输出可确证的 开放 / 关闭(RST) / 已过滤(超时) |
| HTTP 状态 | `http` | `curl` | 状态码、协议、远程 IP、重定向次数、DNS/TCP/TLS/TTFB/总耗时拆解、最终响应头 |
| SSL 证书 | `ssl` | `openssl s_client` + `x509` | 颁发者、有效期与剩余天数、签名算法、公钥、SAN、证书链、SHA-256 指纹 |
| WHOIS 查询 | `whois` | 裸 TCP 43（IANA → 注册局） | 注册商、注册/到期/更新时间、域名状态、NS、DNSSEC，中英注册局格式都归一成中文标签 |

### 文本工具（浏览器本地，不发请求）

| 工具 | 说明 |
|---|---|
| JSON 工具 | 格式化 / 压缩 / 校验，报错带位置提示 |
| Base64 | UTF-8 安全的编码与解码 |
| 哈希计算 | SHA-1 / SHA-256 / SHA-384 / SHA-512，输入即算，点击复制 |
| 时间戳转换 | 当前时间实时刷新、时间戳↔日期双向互转 |

> 哈希用纯 JS 实现（`web/js/sha.js`）：微应用可能被宿主以 http 源加载，此时浏览器不提供 `crypto.subtle`，不能因此让工具不可用。算法常量按 FIPS 180-4 从素数算出，与 `sha1sum`/`sha256sum` 输出逐字节一致。

## 接口请求

前端不直连后端，统一让宿主执行命令再解析 JSON。唯一入口是 `web/js/cli.js`。

```js
const bin = '/.__gmssh/plugin/liteapp/jg/webmasterToolbox/app/bin/main'

await window.$gm.request('/api/command/exec_shell', {
  method: 'post',
  data: {
    cmd_string: `'${bin}' toolbox --action 'dns' --target 'example.com' --type 'ALL' --json`,
    timeout: '30', cwd: '', user: '',
  },
})
```

命令参数一律 **单引号转义 + 字符白名单**，后端再用「可执行文件 + 参数数组」执行外部命令，用户输入永远无法变成命令拼接。

### 命令一览

| action | 主要参数 | 用途 |
|---|---|---|
| `probe` | 无 | 检测本机有哪些依赖可用（ping/dig/nslookup/curl/openssl/whois） |
| `dns` | `--target` `--type` `--server` | 域名解析 |
| `ping` | `--target` `--count` | 网络连通性 |
| `port` | `--target` | 常用端口探测 |
| `http` | `--target` `--port` | 网站响应与计时 |
| `ssl` | `--target` `--port`（默认 443） | 证书详情 |
| `whois` | `--target` | 域名注册信息 |

公共参数：`--timeout`（单阶段超时秒数，1–60）。

### 返回结构

统一信封，业务数据在 `data` 里；失败时信封写到 stderr，退出码 1（用法错误 2）：

```json
{
  "ok": true,
  "data": {
    "host": "example.com",
    "checks": [
      { "port": 443, "service": "HTTPS", "state": "open", "elapsed_ms": 363 }
    ],
    "open": 3, "closed": 0, "filtered": 4, "errors": 0, "elapsed_ms": 5001
  }
}
```

能力缺失时不伪造结果：`supported: false` + `error_kind: "missing_tool"`，界面直接显示原因。

### 服务器上直接验证

```bash
BIN=/.__gmssh/plugin/liteapp/jg/webmasterToolbox/app/bin/main

$BIN toolbox --action probe --json
$BIN toolbox --action dns   --target example.com --type ALL --json
$BIN toolbox --action port  --target example.com --json
$BIN toolbox --action ssl   --target example.com --json
```

## 架构说明

- **前端** `web/`：纯原生 ES Module，无框架、无打包、无路由库，页面切换靠 `location.hash`，各页输入与结果状态保留。
- **后端** `backend/`：`gmssh-cli-plugin` 纯 CLI，宿主每次操作执行一次、打印 JSON 后退出；框架层不含业务代码。
- 所有网络检测只读，不安装软件、不改系统配置、不动防火墙。

## 目录结构

```
站长百宝箱/
├── web/                        # 前端（零构建，直接部署）
│   ├── index.html
│   ├── css/style.css
│   ├── img/bg.jpg
│   ├── js/app.js               # 首页 + 10 个工具
│   ├── js/cli.js               # 唯一请求入口：exec_shell → 后端 CLI
│   ├── js/sha.js               # SHA-1/256/384/512（纯 JS）
│   ├── vendor/gm-sdk.js        # 宿主 SDK
│   ├── dev-server.mjs          # 本地联调（PORT 环境变量，默认 5185）
│   └── liteapp.json            # 应用身份
├── backend/                    # Go 纯 CLI
│   ├── cmd/main/main.go
│   └── internal/biz/toolbox/   # dns / dnsall / ping / portscan / http / ssl / whois
├── pack.sh                     # 产出 web_dist.tar.gz + backend_dist.tar.gz
└── README.md
```

## 本地开发与打包

```bash
# 前端联调（把 exec_shell 桥接成 go run）
cd web && node dev-server.mjs          # http://dev.adde.cn:5185
PORT=8085 node dev-server.mjs          # 换端口

# 后端自检与交叉编译
cd backend && make check && make linux-amd64

# 打包
./pack.sh
```

- `web_dist.tar.gz` → 解到 `web/`
- `backend_dist.tar.gz` → 解到 `app/`（根目录含 `install.sh` / `config` / `data` / `logs` / `tmp` / `bin/main`）

## 依赖提醒

- 网络类工具依赖服务器上的 `ping`、`dig`（或 `nslookup` / `getent`）、`curl`、`openssl`；缺失时界面明确提示，不会假装成功。
- WHOIS 走 TCP 43，部分网络会封锁该端口，失败时显示具体原因。
- 宿主的 `exec_shell` 超时是 30 秒，端口探测单端口等待已限制在 5 秒内。
