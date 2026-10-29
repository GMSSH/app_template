# 域名工具箱 (domainToolbox) · 极目桌面微应用

输入一个域名，一口气跑完 9 项真实网络检测：注册信息、DNS、多源 DNS、网站访问、SSL 证书、端口、跳转链、邮件配置、DNSSEC。

结果按**中心辐射拓扑**铺开，点任一节点看详情。全程只读，不改服务器任何配置。

## 应用标识（appkey）

| 项 | 值 |
|---|---|
| **user_id** | `jg` |
| **app_name（appkey）** | `domainToolbox` |
| 应用名称 | 域名工具箱 |
| 应用标题 | 域名工具箱 |
| 版本 | `1.0.0` |
| 后端二进制 | `/.__gmssh/plugin/liteapp/jg/domainToolbox/app/bin/main` |
| CLI 命令名 | `domain` |
| 前端入口 | `web/index.html` |
| 打包产物 | `web_dist.tar.gz` + `backend_dist.tar.gz` |

身份配置见 `web/liteapp.json`。

## 接口请求

前端不直连后端，统一让宿主执行命令再解析 JSON。

```js
const bin = '/.__gmssh/plugin/liteapp/jg/domainToolbox/app/bin/main'

await window.$gm.command.exec_shell({
  cmd_string: `${bin} domain --action overview --domain example.com --json`,
})
```

### 命令一览

| `--action` | 参数 | 用途 |
|---|---|---|
| `overview` | `--domain <域名>` | 九大模块并行检测，一次拿全部结果 |
| `probe` | 无 | 健康探针：版本、引擎、时间 |
| `rdap` | `--domain` | 注册信息（RDAP，无 RDAP 的 TLD 自动走 WHOIS） |
| `dns` | `--domain` | A / AAAA / CNAME / MX / TXT / NS / CAA / SOA |
| `multidns` | `--domain` | 本地 / Cloudflare / Google / AliDNS 四源对比 |
| `web` | `--domain` | HTTP 访问与耗时分解（DNS / TCP / TLS / TTFB） |
| `ssl` | `--domain` | TLS 握手取证书链、SAN、有效期、PEM 原文 |
| `port` | `--domain` | TCP 80 / 443 连通性 |
| `redirect` | `--domain` | 跳转链跟随（最多 10 跳，含死循环检测） |
| `mail` | `--domain` | MX / SPF / DMARC / DKIM |
| `dnssec` | `--domain` | DS / DNSKEY / RRSIG |

### 返回结构

统一信封，业务数据在 `data`：

```json
{
  "ok": true,
  "data": {
    "domain": "example.com",
    "timestamp": "2026-10-29T20:12:00+08:00",
    "httpStatus": 200,
    "primaryIP": "93.184.216.34",
    "modules": {
      "rdap": { "module": "rdap", "status": "success", "durationMs": 144, "data": {}, "error": "" },
      "dns": { "...": "..." }
    }
  }
}
```

- 每个模块有独立 `status`（`success` / `warning` / `error` / `unavailable` / `not_configured`）、独立超时与 `error`，单模块失败不影响其他模块。
- 失败时整个命令返回 `{"ok":false,"error":{code,type,message,key}}`，退出码非 0。

## 九大检测模块

| 模块 | 内容 |
|---|---|
| 域名信息 | RDAP 标准协议 + IANA Bootstrap 自适应匹配注册局；`.cn` 等无 RDAP 的 TLD 自动改走 WHOIS(43)，含注册人、注册商、注册/到期时间、域名状态、权威 NS |
| DNS 记录 | A、AAAA、CNAME、MX、TXT、NS、CAA、SOA，带 TTL 与解析器来源；查询超时的类型单独标记，不会当成"没有记录" |
| 多源 DNS | 本地系统 DNS / Cloudflare 1.1.1.1 / Google 8.8.8.8 / AliDNS 223.5.5.5 并行对比，按 IP 集合判一致（忽略顺序），失败解析器单独计数 |
| 网站访问 | 状态码、协议、Server、Content-Type，耗时分解 DNS / TCP / TLS / TTFB |
| SSL 证书 | 证书链、签名算法、SAN 列表、有效期与剩余天数、PEM 原文 |
| 端口连通 | TCP 80 / 443 拨测与延迟 |
| 跳转链 | 逐跳跟随 301/302，记录每跳耗时与 Location，最多 10 跳 |
| 邮件配置 | MX 优先级、SPF 策略、DMARC 策略、DKIM 选择器 |
| DNSSEC | DS / DNSKEY / RRSIG 与启用状态 |

## 安全边界

- **每次拨号前校验解析 IP**：阻断回环 `127.0.0.0/8`、RFC1918（`10/8`、`172.16/12`、`192.168/16`）、链路本地 `169.254/16`、IPv6 本地/ULA 以及云厂商元数据地址，防 SSRF 与 DNS Rebinding。见 `backend/internal/biz/domain/security/ssrf.go`。
- **只读**：不写配置、不改 DNS、不装软件、不调 AI/大模型，所有判定来自协议原文。
- **真实数据**：不使用模拟数据；拿不到就如实报错并附带原因。

## 目录结构

```
.
├── pack.sh                  # 一键打包前端 + 后端
├── web/                     # 零构建原生 HTML/CSS/JS（无运行时 CDN）
│   ├── index.html
│   ├── css/style.css
│   ├── js/{app,api,icons}.js
│   ├── vendor/gm-sdk.js     # 宿主 SDK
│   ├── liteapp.json         # 应用标识
│   └── dev-server.mjs       # 本地联调桥接（不进部署包）
└── backend/                 # Go 纯 CLI（gmssh-cli-plugin）
    ├── cmd/main/main.go
    ├── internal/biz/domain/ # register / inspector / security / types
    ├── internal/command/    # CLI 框架
    └── i18n/                # 中英文案
```

## 本地开发

```bash
# 前端 + 后端一起跑（把 exec_shell 桥接到 go run）
cd web && node dev-server.mjs        # http://dev.adde.cn:8085

# 直接跑后端命令
cd backend && go run ./cmd/main domain --action overview --domain example.com --json
```

## 打包与部署

```bash
./pack.sh    # 产出 web_dist.tar.gz + backend_dist.tar.gz
```

后端包解开是扁平结构，`app/bin/main` 即入口；前端包解开内容即为站点根目录。
