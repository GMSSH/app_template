# 攻击雷达 (Attack Radar) · 极目桌面微应用

基于 GMSSH 微应用标准的服务器 3D 态势感知与全球安全态势大屏。

## 架构说明

遵循 GMSSH 官方纯 CLI 规范：

- **前端**：原生 HTML + CSS + JavaScript（**无框架、无构建、无 npm**）。
  3D 用 Three.js，2D 用 Canvas 2D，地理数据用 d3-geo + topojson-client，
  全部以本地 vendor 文件引入。通过 ```window.$gm.request('/api/command/exec_shell', { data: { cmd_string } })``` 调度后端命令。
- **后端**：Go 纯 CLI（```app/bin/main radar --action <server|global|heat|paths> --json```），单次执行输出紧凑 JSON 信封后即刻退出，无常驻进程，无 Unix Socket。
- **降级保护**：当宿主的命令能力不可用（普通浏览器 + 未启动 dev-server）时，自动回退内置模拟态势数据；本地联调时 ```dev-server.mjs``` 会把 exec_shell 桥接成 ```go run ./cmd/main radar```，因此浏览器里看到的也是真实数据。

## 目录结构

```
攻击雷达/
├── web/                       # 前端（原生，零构建）
│   ├── index.html             # 全部结构
│   ├── css/radar.css          # 全部样式
│   ├── js/
│   │   ├── app.js             # 主逻辑（状态 / 渲染 / 交互）
│   │   ├── api.js             # exec_shell 封装 + 数据聚合 + Mock 回退
│   │   ├── Globe.js           # 3D 地球渲染引擎（由 Globe.ts 编译）
│   │   ├── Flat.js            # 2D 投影渲染引擎（由 Flat.ts 编译）
│   │   ├── config.js          # 服务器坐标、Radar 窗口等配置
│   │   └── countries.js       # 国家码 → 中文名 / 中心坐标
│   ├── vendor/                # 第三方库（原样拷贝，勿改）
│   │   ├── three.module.js    # Three.js r180
│   │   ├── three.core.js
│   │   ├── OrbitControls.js
│   │   ├── d3-array.min.js
│   │   ├── d3-geo.min.js
│   │   ├── topojson-client.min.js
│   │   ├── countries-110m.json
│   │   └── gm-sdk.js          # 宿主 SDK（gm-app-sdk）
│   ├── liteapp.json           # 宿主应用身份（平台创建应用时填入）
│   └── dev-server.mjs         # 本地联调服务器（端口 5185）
├── backend/                   # Go 纯 CLI 引擎
│   ├── cmd/main/main.go       # 纯 CLI 入口
│   └── internal/biz/radar/    # 雷达核心业务命令 (server/global/heat/paths)
├── pack.sh                    # 一键打包
└── README.md
```

## 本地开发与联调

### 1. 前端（固定 5185 端口）

```bash
cd web
node dev-server.mjs
```

联调入口：```http://dev.adde.cn:5185``` 或 ```http://localhost:5185```。

```dev-server.mjs``` 做两件事：
1. 把前端的 ```/api/command/exec_shell``` 请求桥接成 ```go run ./cmd/main radar ...```（真实数据）
2. 把前端对 ```/cf-radar/*``` 的请求转发到 Cloudflare Radar（浏览器没有 CORS 也读不到服务器上的 token）

### 2. Radar Token（可选，仅全球热区需要）

不要把 token 写进 JS。两种方式：

**线上（推荐）** — token 只放服务器，前端通过 exec_shell 读取：
```bash
sudo mkdir -p /etc/gm-radar
echo '你的token' | sudo tee /etc/gm-radar/radar.token
sudo chmod 600 /etc/gm-radar/radar.token
```

**本地联调** — 写 ```web/.radar.token```，或
```bash
CF_RADAR_TOKEN=你的token node dev-server.mjs
```

申请：https://dash.cloudflare.com/profile/api-tokens → 创建令牌 → 权限只勾 **User Details:Read**（只读）。

未配置 token 时，全球热区自动回退到内置模拟占比数据，界面照常可用。

### 3. 后端自测

```bash
cd backend
go run ./cmd/main radar --action server --json
go run ./cmd/main radar --action heat --metric target --range 3d --json
go run ./cmd/main radar --action paths --range 3d --json
```

## 构建与打包

```bash
./pack.sh
```

产物：
- ```web_dist.tar.gz```：前端包（解压到 ```app/www/```，无需构建）
- ```backend_dist.tar.gz```：Linux amd64 CLI 二进制包

## 修改前端

改完直接刷新浏览器即可，**没有构建步骤**。

唯一需要注意：```js/Globe.js```、```js/Flat.js```、```js/config.js```、```js/countries.js``` 是从 TypeScript 编译出来的产物。
如需改动这四个文件，改 ```js/*.js``` 即可直接生效；若想保持 TS 源，需要另行维护 .ts 并重新编译：

```bash
npx tsc --ignoreConfig src/Globe.ts src/Flat.ts src/config.ts src/countries.ts \
  --target es2022 --module es2022 --moduleResolution bundler \
  --skipLibCheck --outDir web/js
```

然后修正 ```Globe.js``` 里的裸导入（three / OrbitControls / d3-geo / topojson-client）
指向 ```../vendor/```，并把动态 ```import()``` 换成 ```window.d3 / window.topojson```（见文件内注释）。
