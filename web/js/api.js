/* 数据层：宿主 $gm.request → 本地桥接 → 模拟数据 */
import { GEO_DB, SERVER, RADAR_TOKEN, RADAR_RANGE } from './config.js'

// 初值必须是 false：先走真实后端，只有确认拿不到时才降级。
// 若初值为 true，fetchServer 会跳过 CLI，位置永远停在 config.js 的占位坐标。
const state = { usingMock: false, identity: null }
// confirmed = 是否已从后端拿到真实经纬度；未确认时界面不宣称任何具体国家
export const currentServerLoc = {
  ip: '', country: '', region: '', city: '',
  lat: SERVER.lat, lon: SERVER.lon, isp: '', name: SERVER.name || '本机服务器',
  confirmed: false,
}

export const isMock = () => state.usingMock
export const usingMock = state

/* ---------------- 宿主命令执行 ---------------- */
async function loadIdentity() {
  if (state.identity) return state.identity
  try {
    const r = await fetch('./liteapp.json', { cache: 'no-store' })
    if (r.ok) state.identity = await r.json()
  } catch { /* 浏览器预览无 liteapp.json */ }
  if (!state.identity || !state.identity.user_id) state.identity = { user_id: '', app_name: '' }
  return state.identity
}

const q = (v) => "'" + String(v).replace(/'/g, "'\\''") + "'"

function sdk() {
  try { return window.$gm || null } catch { return null }
}

// 没有宿主时用本地 dev-server.mjs 桥接（把 exec_shell 转成 go run）
function sendExec(cmd_string, timeout) {
  const g = sdk()
  const payload = { method: 'post', data: { cmd_string, timeout, cwd: '', user: '' } }
  if (g && g.request) return g.request('/api/command/exec_shell', payload)
  return fetch('/api/command/exec_shell', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(payload),
  }).then((r) => r.json().then((j) => {
    if (!r.ok) throw new Error((j && j.msg) || 'dev bridge HTTP ' + r.status)
    return j
  }))
}

async function sh(cmd, timeout = '40') {
  const g = sdk()
  if (!g?.request && !location.port) throw new Error('未检测到极目宿主，无法读取服务器数据')
  const r = await sendExec(cmd, timeout)
  const d = r?.data ?? r
  return String(d?.stdout ?? d?.data?.stdout ?? (typeof d?.data === 'string' ? d.data : '') ?? '')
}

export async function callCli(commandName = 'radar', args = {}) {
  const g = sdk()
  const id = await loadIdentity()
  const home = typeof g?.gmsshHome === 'string' ? g.gmsshHome.replace(/\/$/, '') : ''
  let bin = `/.__gmssh/plugin/liteapp/${id.user_id}/${id.app_name}/app/bin/main`
  if (home) bin = bin.replace('/.__gmssh', home)

  const lang = g?.lang || 'zh-CN'
  const flags = Object.entries({ ...args, lang }).map(([k, v]) => `--${k} ${q(String(v))}`).join(' ')
  const cmd_string = `${q(bin)} ${commandName} ${flags} --json`

  const resp = await Promise.race([
    sendExec(cmd_string, '30'),
    new Promise((_, rej) => setTimeout(() => rej(new Error('后端调用超时（30s）')), 30000)),
  ])
  return unwrapExec(resp, bin)
}

function unwrapExec(resp, binPath) {
  if (!resp) throw new Error('exec_shell: response is empty')
  if (typeof resp.code === 'number' && resp.code !== 200000 && resp.code !== 0) {
    throw new Error(resp.msg || resp.message || 'exec_shell failed')
  }
  let text = ''
  if (typeof resp.data === 'string') text = resp.data
  else if (resp.data && typeof resp.data.stdout === 'string') text = resp.data.stdout
  else if (typeof resp.stdout === 'string') text = resp.stdout
  else if (typeof resp === 'string') text = resp

  let stderr = ''
  if (resp.data && typeof resp.data.stderr === 'string') stderr = resp.data.stderr
  else if (resp.stderr) stderr = resp.stderr

  const combined = (stderr + '\n' + text).trim()
  if (combined.includes('No such file') || combined.includes('command not found')) {
    throw new Error(`后端未就绪：请确保已将后端部署至 ${binPath}`)
  }
  if (combined.includes('Permission denied')) {
    throw new Error(`后端无执行权限：请为 ${binPath} 添加执行权限 (chmod +x)`)
  }

  const trimmed = text.trim()
  if (!trimmed) {
    if (stderr.trim()) throw new Error(stderr.trim())
    return {}
  }
  let parsed = null
  try { parsed = JSON.parse(trimmed) } catch {
    const a = trimmed.indexOf('{'), b = trimmed.lastIndexOf('}')
    if (a >= 0 && b > a) { try { parsed = JSON.parse(trimmed.slice(a, b + 1)) } catch { /* ignore */ } }
    if (!parsed) throw new Error(stderr.trim() || trimmed)
  }
  if (parsed && typeof parsed === 'object' && 'ok' in parsed) {
    if (parsed.ok) return parsed.data === undefined ? parsed : parsed.data
    throw new Error(parsed.error?.message || 'CLI command returned error')
  }
  return parsed
}

/* ---------------- 本机日志 → 事件 ---------------- */
const IP = '(\\d{1,3}(?:\\.\\d{1,3}){3})'
const PHP_PAT = 'eval-stdin|phpunit|/vendor/|allow_url_include|auto_prepend_file|invokefunction|php://input|\\.php\\?[^ ]*(cmd|exec|system|eval|shell)='
export const PHP_KIND = 'PHP 漏洞探测'
const PHP_RE = new RegExp(PHP_PAT)

const SRC = {
  ssh: `grep -hE 'Failed password|Invalid user|authentication failure' /var/log/auth.log /var/log/auth.log.1 /var/log/secure 2>/dev/null | tail -n 1500`,
  web: `grep -hE '\\.env|wp-login|phpmyadmin|/\\.git|\\.\\./|xmlrpc|cgi-bin|etc/passwd|union.+select|eval\\(|${PHP_PAT}' /var/log/nginx/access.log /var/log/nginx/access.log.1 /var/log/apache2/access.log /var/log/apache2/access.log.1 /var/log/httpd/access_log 2>/dev/null | tail -n 5000`,
  f2b: `grep -h ' Ban ' /var/log/fail2ban.log 2>/dev/null | tail -n 500`,
}

function tsSyslog(l) {
  const a = l.match(/^(\d{4}-\d\d-\d\dT[\d:.]+(?:Z|[+-]\d\d:\d\d))/)
  if (a) return Date.parse(a[1])
  const b = l.match(/^(\w{3})\s+(\d+)\s+([\d:]+)/)
  return b ? Date.parse(`${b[1]} ${b[2]} ${new Date().getFullYear()} ${b[3]}`) : 0
}

const P = {
  ssh: (l) => {
    const ip = l.match(new RegExp('(?:from |rhost=)' + IP))?.[1]
    const t = tsSyslog(l)
    return ip && t ? [t, ip] : null
  },
  web: (l) => {
    const m = l.match(new RegExp('^' + IP + '.*\\[(\\d+)/(\\w+)/(\\d+):([\\d:]+) ([+-]\\d{4})\\]'))
    return m ? [Date.parse(`${m[2]} ${m[3]} ${m[4]} ${m[5]} ${m[6]}`), m[1]] : null
  },
  f2b: (l) => {
    const m = l.match(new RegExp('^(\\d{4}-\\d\\d-\\d\\d) ([\\d:]+),\\d+ .*Ban ' + IP))
    return m ? [Date.parse(`${m[1]}T${m[2]}`), m[3]] : null
  },
}

const KIND = { ssh: 'SSH 认证失败', web: 'Web 可疑请求', f2b: 'Fail2ban 封禁' }

const geoCache = new Map()
async function geo(ips) {
  const todo = [...new Set(ips)].filter((i) => !geoCache.has(i) && /^\d+(\.\d+){3}$/.test(i))
  if (!todo.length) return
  const out = await sh(`for i in ${todo.join(' ')}; do echo "== $i"; mmdblookup --file '${GEO_DB}' --ip $i 2>/dev/null; done`)
  for (const blk of out.split('== ').slice(1)) {
    const ip = blk.split('\n')[0].trim()
    const lat = blk.match(/"latitude":\s*([-\d.]+)/)
    const lon = blk.match(/"longitude":\s*([-\d.]+)/)
    const c = blk.match(/"country":[\s\S]*?"en":\s*"([^"]+)"/)
    geoCache.set(ip, lat && lon ? { lat: +lat[1], lon: +lon[1], name: c?.[1] ?? '未知' } : null)
  }
}

/* ---------------- 持久化 ---------------- */
const KEY = 'attack-radar:v1', GKEY = 'attack-radar:global:v1'
export function loadStore() {
  try { return JSON.parse(localStorage.getItem(KEY) || '[]') } catch { return [] }
}
export function saveStore(a) {
  const lim = Date.now() - 7 * 864e5
  a = a.filter((e) => e.ts > lim).sort((x, y) => y.ts - x.ts).slice(0, 6000)
  try { localStorage.setItem(KEY, JSON.stringify(a)) } catch { /* quota */ }
  return a
}

/* ---------------- 本机事件 ---------------- */
export async function fetchServer(known) {
  if (!isMock()) {
    try {
      const cliData = await callCli('radar', { action: 'server' })
      let rows = []
      if (cliData && typeof cliData === 'object' && !Array.isArray(cliData) && 'location' in cliData) {
        // 只有后端明确标注 located 才采用坐标；cip.cc 兜底只有 IP/名称、没有坐标
        const L = cliData.location
        if (L) {
          Object.assign(currentServerLoc, L)
          currentServerLoc.confirmed = L.located === true && !!L.lat && !!L.lon
          if (currentServerLoc.confirmed) {
            SERVER.lat = L.lat
            SERVER.lon = L.lon
            SERVER.name = L.name || [L.country, L.region, L.city].filter(Boolean).join(' ')
          }
        }
        rows = cliData.events || []
        state.usingMock = !cliData.is_real
      } else if (Array.isArray(cliData)) {
        rows = cliData
        state.usingMock = false
      }
      if (rows.length > 0) {
        const map = new Map(known.map((e) => [e.id, e]))
        for (const row of rows) map.set(row.id, row)
        return [...map.values()]
      }
    } catch (e) {
      console.log('[radar] backend cli unavailable, fallback:', e?.message)
    }
  }

  try {
    known.forEach((e) => e.src && geoCache.set(e.ip, e.src))
    const keys = Object.keys(SRC)
    const res = await Promise.allSettled(keys.map((k) => sh(SRC[k])))
    if (res.every((r) => r.status === 'rejected')) {
      state.usingMock = true
      return mockServer()
    }
    const map = new Map(known.map((e) => [e.id, e]))
    res.forEach((r, i) => {
      if (r.status !== 'fulfilled') return
      for (const l of r.value.split('\n')) {
        const p = l && P[keys[i]](l)
        if (!p) continue
        const id = `${p[0]}|${p[1]}|${keys[i]}`
        const kind = keys[i] === 'web' && PHP_RE.test(l) ? PHP_KIND : KIND[keys[i]]
        const o = map.get(id)
        if (o) o.kind = kind
        else map.set(id, { id, ts: p[0], ip: p[1], kind })
      }
    })
    const all = [...map.values()]
    await geo(all.map((e) => e.ip)).catch(() => {})
    all.forEach((e) => { if (!e.src) e.src = geoCache.get(e.ip) ?? undefined })
    state.usingMock = false
    return all
  } catch {
    state.usingMock = true
    return mockServer()
  }
}

export function agg(rows) {
  const m = new Map()
  for (const r of rows) {
    const k = r.ip + '|' + r.kind
    const e = m.get(k)
    if (e) { e.n++; e.ts = Math.max(e.ts, r.ts) }
    else m.set(k, { id: k, ts: r.ts, ip: r.ip, kind: r.kind, n: 1, level: 1, src: r.src })
  }
  const l = [...m.values()].sort((a, b) => b.ts - a.ts).slice(0, 80)
  l.forEach((e) => (e.level = e.n >= 50 ? 3 : e.n >= 10 ? 2 : 1))
  return l
}

/* ---------------- 全球来源 ---------------- */
export const GLOBAL_TTL = 3600e3
export async function fetchGlobal(force = false) {
  try {
    const c = JSON.parse(localStorage.getItem(GKEY) || 'null')
    if (c && !force && Date.now() - c.at < GLOBAL_TTL) return c
  } catch { /* ignore */ }

  if (!isMock()) {
    try {
      const cliEvents = await callCli('radar', { action: 'global' })
      if (Array.isArray(cliEvents) && cliEvents.length > 0) {
        const out = { at: Date.now(), list: cliEvents }
        try { localStorage.setItem(GKEY, JSON.stringify(out)) } catch { /* ignore */ }
        return out
      }
    } catch { /* fallback */ }
  }

  let txt = ''
  try { txt = await sh(`curl -fsSL --max-time 25 https://feeds.dshield.org/feeds/topips.txt`) } catch { /* mock */ }
  const rows = []
  for (const l of txt.split('\n')) {
    if (!l.trim() || l.startsWith('#')) continue
    const m = l.match(new RegExp(IP))
    if (!m) continue
    const nums = l.slice(l.indexOf(m[0]) + m[0].length).split(/\s+/).filter((t) => /^\d+$/.test(t))
    rows.push({ ip: m[1], reports: +nums[0] || 0, targets: +nums[1] || 0 })
  }
  if (!rows.length) {
    const at = Date.now()
    return { at, list: mockGlobal() }
  }
  await geo(rows.map((r) => r.ip)).catch(() => {})
  const at = Date.now()
  const list = rows.map((r, i) => ({
    id: 'g|' + r.ip, ts: at, ip: r.ip, kind: 'DShield 来源统计', n: r.reports,
    level: i < 10 ? 3 : i < 40 ? 2 : 1,
    src: geoCache.get(r.ip) ?? undefined, reports: r.reports, targets: r.targets,
  }))
  try { localStorage.setItem(GKEY, JSON.stringify({ at, list })) } catch { /* ignore */ }
  return { at, list }
}

function mockGlobal() {
  const at = Date.now()
  return [
    { id: 'g|198.51.100.23', ts: at, ip: '198.51.100.23', kind: 'DShield 来源统计', n: 1420, level: 3, src: { lat: 37.77, lon: -122.42, name: 'United States' }, reports: 1420, targets: 85 },
    { id: 'g|203.0.113.88', ts: at, ip: '203.0.113.88', kind: 'DShield 来源统计', n: 1180, level: 3, src: { lat: 52.52, lon: 13.4, name: 'Germany' }, reports: 1180, targets: 64 },
    { id: 'g|198.51.100.104', ts: at, ip: '198.51.100.104', kind: 'DShield 来源统计', n: 890, level: 2, src: { lat: 55.75, lon: 37.62, name: 'Russia' }, reports: 890, targets: 41 },
    { id: 'g|192.0.2.14', ts: at, ip: '192.0.2.14', kind: 'DShield 来源统计', n: 720, level: 2, src: { lat: -23.55, lon: -46.63, name: 'Brazil' }, reports: 720, targets: 33 },
    { id: 'g|203.0.113.19', ts: at, ip: '203.0.113.19', kind: 'DShield 来源统计', n: 360, level: 1, src: { lat: 28.61, lon: 77.2, name: 'India' }, reports: 360, targets: 18 },
  ]
}

/* ---------------- Cloudflare Radar ---------------- */
const HKEY = 'attack-radar:heat:v3', PKEY = 'attack-radar:paths:v1'
export const HEAT_TTL = 1800e3

const TOKEN_CMD = `cat /etc/gm-radar/radar.token /opt/gm-radar/radar.token 2>/dev/null | head -1 | tr -d '\\r\\n '`
const shellToken = (tok) => `TOK=$( ${TOKEN_CMD} ); [ -n "$TOK" ] || TOK='${tok}'; `

async function radarGet(path, range) {
  const apiPath = `/client/v4/radar/attacks/layer3/${path}${path.includes('?') ? '&' : '?'}dateRange=${range}&format=JSON`
  const t0 = Date.now()
  let body = '', status = 0

  if (isMock()) {
    const r = await fetch('/cf-radar' + apiPath, { headers: { accept: 'application/json' } })
    status = r.status
    body = await r.text()
    if (!/json/i.test(r.headers.get('content-type') || '')) throw new Error('DEV_NO_PROXY')
  } else {
    const out = await sh(shellToken(RADAR_TOKEN) + `[ -n "$TOK" ] || { echo '__NOTOKEN__'; exit 0; }; curl -sS --max-time 25 -w '\\n__HTTP:%{http_code}__' -H "Authorization: Bearer $TOK" "https://api.cloudflare.com${apiPath}"`)
    if (out.includes('__NOTOKEN__')) throw new Error('未配置 Radar token：把令牌写入服务器 /etc/gm-radar/radar.token（权限 600）即可，无需改前端代码')
    const m = out.match(/__HTTP:(\d+)__\s*$/)
    status = m ? +m[1] : 0
    body = out.replace(/\n?__HTTP:\d+__\s*$/, '').trim()
  }

  const ms = Date.now() - t0
  let j = null
  try { j = JSON.parse(body) } catch { /* ignore */ }
  if (!j) throw new Error(status === 0 ? 'Radar 无响应（确认能访问 api.cloudflare.com）' : `Radar 返回非 JSON（HTTP ${status}）`)
  if (!j.success) {
    const msg = j?.errors?.[0]?.message || `Radar 请求失败（HTTP ${status}）`
    throw new Error(status === 400 && /date|range/i.test(msg) ? `dateRange 不支持：${msg}` : msg)
  }
  return { j, ms, range }
}

async function radarFetch(path) {
  const tries = [RADAR_RANGE, '7d'].filter((v, i, a) => a.indexOf(v) === i)
  let last
  for (const range of tries) {
    try { return await radarGet(path, range) } catch (e) {
      last = e
      if (!/dateRange|HTTP 400|不支持/i.test(String(e?.message))) throw e
    }
  }
  throw last
}

function mockHeat(metric) {
  const W = {
    US: ['United States', 18.4], CN: ['China', 15.2], RU: ['Russia', 11.7], NL: ['Netherlands', 7.9],
    BR: ['Brazil', 6.1], IN: ['India', 5.4], DE: ['Germany', 4.8], VN: ['Vietnam', 3.9],
    IR: ['Iran', 3.2], UA: ['Ukraine', 2.7], ID: ['Indonesia', 2.4], TR: ['Turkey', 2.1],
    SG: ['Singapore', 1.8], KR: ['Korea', 1.5], GB: ['United Kingdom', 1.3], FR: ['France', 1.2],
    JP: ['Japan', 1.1], CA: ['Canada', 1.0], AU: ['Australia', 0.9], PL: ['Poland', 0.85],
    IT: ['Italy', 0.8], ES: ['Spain', 0.75], MX: ['Mexico', 0.7], ZA: ['South Africa', 0.65],
    SE: ['Sweden', 0.6], RO: ['Romania', 0.55], TH: ['Thailand', 0.5], PH: ['Philippines', 0.45],
    MY: ['Malaysia', 0.42], AR: ['Argentina', 0.38], CO: ['Colombia', 0.35], CL: ['Chile', 0.3],
    EG: ['Egypt', 0.28], NG: ['Nigeria', 0.26], PK: ['Pakistan', 0.24], BD: ['Bangladesh', 0.22],
    SA: ['Saudi Arabia', 0.2], AE: ['United Arab Emirates', 0.19], IL: ['Israel', 0.18],
    FI: ['Finland', 0.16], CZ: ['Czechia', 0.15], PT: ['Portugal', 0.14], GR: ['Greece', 0.13],
    PE: ['Peru', 0.12], KZ: ['Kazakhstan', 0.11], DZ: ['Algeria', 0.1], KE: ['Kenya', 0.09],
  }
  return {
    at: Date.now(), metric, range: RADAR_RANGE, mock: true, ms: 0,
    list: Object.keys(W).map((cc) => ({ cc, name: W[cc][0], value: W[cc][1] })),
  }
}

function mockPaths() {
  const L = [
    ['CN', 'US', 6.1], ['RU', 'UA', 5.4], ['US', 'CN', 4.7], ['NL', 'DE', 3.6], ['BR', 'US', 3.1],
    ['IR', 'IL', 2.8], ['IN', 'US', 2.4], ['VN', 'SG', 2.0], ['RU', 'US', 1.9], ['TR', 'DE', 1.6],
    ['US', 'GB', 1.5], ['KR', 'JP', 1.3], ['FR', 'BE', 1.1], ['ID', 'AU', 0.9], ['ZA', 'NG', 0.7],
  ]
  return {
    at: Date.now(), range: RADAR_RANGE, mock: true, ms: 0,
    list: L.map(([from, to, value]) => ({ from, to, value, fromName: from, toName: to })),
  }
}

export async function fetchHeat(metric, force = false) {
  const ck = `${HKEY}:${metric}:${RADAR_RANGE}`
  try {
    const c = JSON.parse(localStorage.getItem(ck) || 'null')
    if (c && !force && Date.now() - c.at < HEAT_TTL) return { ...c, cached: true }
  } catch { /* ignore */ }

  if (!isMock()) {
    try {
      const cliHeat = await callCli('radar', { action: 'heat', metric, range: RADAR_RANGE })
      if (cliHeat && Array.isArray(cliHeat.list) && cliHeat.list.length > 0) {
        try { localStorage.setItem(ck, JSON.stringify(cliHeat)) } catch { /* ignore */ }
        return cliHeat
      }
    } catch { /* fallback */ }
  }

  try {
    const { j, ms, range } = await radarFetch(`top/locations/${metric}?limit=100`)
    const list = (j.result?.top_0 || []).map((r) => {
      const cc = metric === 'target' ? (r.targetCountryAlpha2 || r.originCountryAlpha2) : (r.originCountryAlpha2 || r.targetCountryAlpha2)
      const name = metric === 'target' ? (r.targetCountryName || r.originCountryName) : (r.originCountryName || r.targetCountryName)
      return { cc: cc || '', name: name || '', value: +r.value || 0 }
    }).filter((r) => r.cc).sort((a, b) => b.value - a.value)

    if (list.length > 0) {
      const out = { at: Date.now(), metric, range, list, ms }
      try { localStorage.setItem(ck, JSON.stringify(out)) } catch { /* ignore */ }
      return out
    }
  } catch { /* fallback */ }

  const m = mockHeat(metric)
  try { localStorage.setItem(ck, JSON.stringify(m)) } catch { /* ignore */ }
  return m
}

export async function fetchPaths(force = false) {
  const ck = `${PKEY}:${RADAR_RANGE}`
  try {
    const c = JSON.parse(localStorage.getItem(ck) || 'null')
    if (c && !force && Date.now() - c.at < HEAT_TTL) return { ...c, cached: true }
  } catch { /* ignore */ }

  if (!isMock()) {
    try {
      const cliPaths = await callCli('radar', { action: 'paths', range: RADAR_RANGE })
      if (cliPaths && Array.isArray(cliPaths.list) && cliPaths.list.length > 0) {
        try { localStorage.setItem(ck, JSON.stringify(cliPaths)) } catch { /* ignore */ }
        return cliPaths
      }
    } catch { /* fallback */ }
  }

  try {
    const { j, ms, range } = await radarFetch('top/attacks?limit=30')
    const list = (j.result?.top_0 || []).map((r) => ({
      from: r.originCountryAlpha2 || '', to: r.targetCountryAlpha2 || '',
      fromName: r.originCountryName || '', toName: r.targetCountryName || '',
      value: +r.value || 0,
    })).filter((p) => p.from && p.to && p.from !== p.to)

    if (list.length > 0) {
      const out = { at: Date.now(), range, list, ms }
      try { localStorage.setItem(ck, JSON.stringify(out)) } catch { /* ignore */ }
      return out
    }
  } catch { /* fallback */ }

  const m = mockPaths()
  try { localStorage.setItem(ck, JSON.stringify(m)) } catch { /* ignore */ }
  return m
}

/* ---------------- 模拟本机事件 ---------------- */
const MOCK = (() => {
  const P = [
    ['美国 (旧金山)', 37.77, -122.42], ['德国 (柏林)', 52.52, 13.4],
    ['俄罗斯 (莫斯科)', 55.75, 37.62], ['巴西 (圣保罗)', -23.55, -46.63],
    ['印度 (新德里)', 28.61, 77.2], ['荷兰 (阿姆斯特丹)', 52.37, 4.89],
    ['越南 (河内)', 21.02, 105.83], ['伊朗 (德黑兰)', 35.68, 51.38],
  ]
  const kinds = ['SSH 认证失败', 'Web 可疑请求', 'PHP 漏洞探测', 'Fail2ban 封禁']
  return Array.from({ length: 120 }, (_, i) => {
    const p = P[i % P.length]
    return {
      id: 'm' + i, ts: Date.now() - i * 45e3, ip: `192.0.2.${(i % 30) + 1}`,
      kind: kinds[i % kinds.length], src: { name: p[0], lat: p[1], lon: p[2] },
    }
  })
})()

function mockServer() { return MOCK }
