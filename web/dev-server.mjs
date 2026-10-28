// 本地联调服务器：静态文件 + /api/command/exec_shell 桥接 + Cloudflare Radar 代理
// 用法：node dev-server.mjs   →  http://localhost:5185
import http from 'node:http'
import fs from 'node:fs'
import path from 'node:path'
import { spawn } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const ROOT = path.dirname(fileURLToPath(import.meta.url))
const PORT = 5185
const BACKEND_DIR = path.resolve(ROOT, '../backend')
const RADAR_BASE = 'https://api.cloudflare.com'

const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.mjs': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.ico': 'image/x-icon',
}

const sendJson = (res, code, obj) => {
  res.writeHead(code, { 'content-type': 'application/json; charset=utf-8' })
  res.end(JSON.stringify(obj))
}

// 只放行 radar 命令，参数白名单后直接 spawn（不经过 shell）
function parseCmd(cmd) {
  const m = cmd.match(/\sradar\s+(.+)$/)
  if (!m) return null
  const args = m[1]
  if (!/^[A-Za-z0-9 _.,/'=:-]+--json$/.test(args)) return null
  return args.split(/\s+/).map((s) => s.replace(/^'|'$/g, ''))
}

function execShell(req, res) {
  let raw = ''
  req.on('data', (c) => (raw += c))
  req.on('end', () => {
    let cmd = ''
    try { cmd = JSON.parse(raw)?.data?.cmd_string ?? '' } catch { return sendJson(res, 400, { code: 400, msg: 'bad json' }) }
    const tokens = parseCmd(cmd)
    if (!tokens) return sendJson(res, 400, { code: 400, msg: 'dev bridge: unsupported cmd_string' })

    const child = spawn('go', ['run', './cmd/main', 'radar', ...tokens], { cwd: BACKEND_DIR })
    let stdout = '', stderr = ''
    child.stdout.on('data', (d) => (stdout += d))
    child.stderr.on('data', (d) => (stderr += d))
    child.on('error', (e) => sendJson(res, 500, { code: 500, msg: 'spawn failed: ' + e.message }))
    child.on('close', () => sendJson(res, 200, { code: 200000, data: { stdout, stderr } }))
  })
}

// 浏览器预览时把 Cloudflare Radar 请求转出去（token 从环境变量或本地文件读）
async function radarProxy(req, res, url) {
  const apiPath = url.pathname.replace(/^\/cf-radar/, '')
  let token = process.env.CF_RADAR_TOKEN || ''
  if (!token) {
    for (const f of ['/etc/gm-radar/radar.token', path.join(ROOT, '.radar.token')]) {
      try { token = fs.readFileSync(f, 'utf8').trim(); break } catch { /* next */ }
    }
  }
  if (!token) return sendJson(res, 503, { success: false, errors: [{ message: '开发代理未配置 RADAR token' }] })
  try {
    const r = await fetch(RADAR_BASE + apiPath, { headers: { Authorization: 'Bearer ' + token, accept: 'application/json' } })
    const body = await r.text()
    res.writeHead(r.status, { 'content-type': 'application/json; charset=utf-8' })
    res.end(body)
  } catch (e) {
    sendJson(res, 502, { success: false, errors: [{ message: '代理请求失败: ' + e.message }] })
  }
}

const server = http.createServer((req, res) => {
  const url = new URL(req.url, 'http://localhost')

  if (url.pathname.startsWith('/cf-radar')) {
    if (req.method !== 'GET') return sendJson(res, 405, { msg: 'GET only' })
    return radarProxy(req, res, url)
  }

  if (url.pathname.startsWith('/api/command/exec_shell')) {
    if (req.method !== 'POST') return sendJson(res, 405, { code: 405, msg: 'POST only' })
    return execShell(req, res)
  }

  const rel = url.pathname === '/' ? 'index.html' : decodeURIComponent(url.pathname.replace(/^\//, ''))
  const file = path.join(ROOT, rel)
  if (!file.startsWith(ROOT)) { res.writeHead(403); return res.end('forbidden') }

  fs.readFile(file, (err, data) => {
    if (err) {
      res.writeHead(404, { 'content-type': 'text/plain; charset=utf-8' })
      return res.end('404 ' + rel)
    }
    res.writeHead(200, { 'content-type': MIME[path.extname(file)] || 'application/octet-stream' })
    res.end(data)
  })
})

server.listen(PORT, '0.0.0.0', () => {
  console.log('攻击雷达（原生 HTML/CSS/JS）: http://localhost:' + PORT)
})
