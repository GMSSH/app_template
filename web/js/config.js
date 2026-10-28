// 飞线终点的占位坐标。真实经纬度由后端 radar --action server 自动探测并覆盖这里，
// 只有探测失败时才会用到占位值 —— 所以不要写成一个具体国家的首都，
// 否则界面会指向一个和实际服务器无关的地方。
export const SERVER = { name: '本机服务器', lat: 31.23, lon: 121.47 };
// DB-IP Lite City 数据库（mmdb）路径，需服务器安装 mmdblookup (libmaxminddb)
export const GEO_DB = '/usr/share/GeoIP/dbip-city-lite.mmdb';
export const POLL_MS = 30000;
// ---- 全球攻击态势：Cloudflare Radar ----
// 【重要】不要把 token 写在这里。config.ts 会被打进前端 JS，任何能打开页面的人都能看到。
// 正确做法：token 只放服务器，前端通过 exec_shell 读取（见 api.ts radarGet）：
//   sudo mkdir -p /etc/gm-radar
//   echo '你的token' | sudo tee /etc/gm-radar/radar.token
//   sudo chmod 600 /etc/gm-radar/radar.token
// 申请：https://dash.cloudflare.com/profile/api-tokens → 创建令牌 → 权限只勾 User Details:Read（只读）
// 这里留空即可；仅当本地临时调试、且不打算上线时才填写。
// token 只放服务器（exec_shell 读取）：/etc/gm-radar/radar.token（权限 600）
// 本地联调：写 web/.radar.token 或环境变量 CF_RADAR_TOKEN，由 dev-server.mjs 代理转发
export const RADAR_TOKEN = '';
// 统计窗口：1d / 2d / 3d / 7d / 14d / 28d（3d 不被接受时 api.ts 会自动退到 7d 并在界面标注实际窗口）
export const RADAR_RANGE = '3d';
