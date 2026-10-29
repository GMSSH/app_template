/* ============================================================
   命令执行适配层（唯一请求入口）
   ------------------------------------------------------------
   浏览器的 HTML/JS 不能直接执行 Linux 命令。这个文件把
   「执行一次后端 CLI」封装成一个 Promise。

   宿主接入方式（极目 GMSSH）：
     后端部署在 /.__gmssh/plugin/liteapp/{user_id}/{app_name}/app/bin/main
     前端调用 window.$gm.request('/api/command/exec_shell', {...})
     cmd_string 里只出现「二进制路径 + 固定子命令 + 已校验参数」。

   没有 $gm 时：有本地 dev-server 就走它的 /api/command/exec_shell 桥接，
   两者都没有就抛错，界面显示「环境不支持」，绝不伪造结果。
   ============================================================ */

const DEFAULT_IDENTITY = { user_id: '', app_name: '' }
let identityPromise = null

export const isHosted = () => {
  try {
    return !!(window.$gm && typeof window.$gm.request === 'function')
  } catch {
    return false
  }
}

/** 让界面明确知道当前是否真的能执行服务器命令 */
export const envMode = () => (isHosted() ? 'host' : location.port ? 'bridge' : 'none')

async function loadIdentity() {
  if (identityPromise) return identityPromise
  identityPromise = fetch('./liteapp.json', { cache: 'no-store' })
    .then((r) => (r.ok ? r.json() : null))
    .catch(() => null)
    .then((j) => ({
      user_id: (j && j.user_id) || DEFAULT_IDENTITY.user_id,
      app_name: (j && j.app_name) || DEFAULT_IDENTITY.app_name,
    }))
  return identityPromise
}

/** POSIX 单引号转义：把任意字符串变成 shell 里的一个安全单词 */
const shq = (v) => "'" + String(v).replace(/'/g, "'\\''") + "'"

/** 只允许出现在参数里的字符集，双保险（真正的安全来自「参数数组 + 不拼 shell」） */
const SAFE_ARG = /^[A-Za-z0-9 .:/_@\[\]?&=%#~+,;-]*$/

function assertSafe(key, value) {
  if (value === undefined || value === null || value === '') return
  if (!SAFE_ARG.test(String(value))) {
    throw new Error(`参数 ${key} 含有不允许的字符`)
  }
}

async function binPath() {
  const id = await loadIdentity()
  const home = typeof window.$gm?.gmsshHome === 'string' ? window.$gm.gmsshHome.replace(/\/$/, '') : ''
  let p = `/.__gmssh/plugin/liteapp/${id.user_id}/${id.app_name}/app/bin/main`
  if (home) p = p.replace('/.__gmssh', home)
  return p
}

function sendExec(cmdString, timeoutSec) {
  const payload = {
    method: 'post',
    data: { cmd_string: cmdString, timeout: String(timeoutSec), cwd: '', user: '' },
  }
  if (isHosted()) return window.$gm.request('/api/command/exec_shell', payload)

  // 本地联调：dev-server.mjs 会把这条请求转成真实 CLI 调用
  return fetch('/api/command/exec_shell', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(payload),
  }).then(async (r) => {
    const j = await r.json().catch(() => null)
    if (!r.ok) throw new Error((j && j.msg) || `本地桥接返回 HTTP ${r.status}`)
    return j
  })
}

/** 容错解析：CLI 的 stdout 前面可能混入日志，取最外层的对象 */
function parseJsonLoose(text) {
  if (!text) return null
  try {
    return JSON.parse(text)
  } catch {
    const a = text.indexOf('{')
    const b = text.lastIndexOf('}')
    if (a >= 0 && b > a) {
      try {
        return JSON.parse(text.slice(a, b + 1))
      } catch {
        return null
      }
    }
    return null
  }
}

function unwrap(resp, bin) {
  if (!resp) throw new Error('exec_shell 返回为空')
  if (typeof resp.code === 'number' && resp.code !== 200000 && resp.code !== 0) {
    throw new Error(resp.msg || resp.message || 'exec_shell 调用失败')
  }

  let stdout = ''
  let stderr = ''
  const d = resp.data ?? resp
  if (typeof d === 'string') stdout = d
  else {
    stdout = d.stdout ?? d.data?.stdout ?? ''
    stderr = d.stderr ?? d.data?.stderr ?? ''
  }

  const merged = (stderr + '\n' + stdout).trim()
  if (merged.includes('No such file or directory') || merged.includes('not found')) {
    throw new Error(`后端未部署或路径不正确：${bin}`)
  }
  if (merged.includes('Permission denied')) {
    throw new Error(`后端没有执行权限：chmod +x ${bin}`)
  }

  const text = String(stdout).trim()
  // 失败时框架把 JSON 信封写到 stderr，也要取出里面的 message，
  // 否则用户会看到一整行原始 JSON。
  if (!text) {
    const se = String(stderr).trim()
    const env = parseJsonLoose(se)
    if (env && env.error) throw new Error(env.error.message || env.error.key || '后端命令执行失败')
    throw new Error(se || '后端未返回任何数据')
  }

  const parsed = parseJsonLoose(text)
  if (!parsed) throw new Error(`后端输出不是合法 JSON：${text.slice(0, 160)}`)

  if (parsed.ok === false) {
    const e = parsed.error
    throw new Error((e && (e.message || e.key)) || '后端命令执行失败')
  }
  return parsed.data
}

/**
 * 执行一次后端命令。
 * @param {string} action 后端子命令，例如 'dns'
 * @param {object} args   已校验的参数，键值都会做字符白名单检查
 * @param {number} timeoutSec 超时秒数（包含宿主执行超时，宿主上限 30s）
 */
export async function callToolbox(action, args = {}, timeoutSec = 30) {
  if (envMode() === 'none') {
    throw new Error('当前环境无法执行服务器命令：请在极目桌面中打开本应用，或用 web/dev-server.mjs 本地联调。')
  }
  for (const [k, v] of Object.entries(args)) assertSafe(k, v)

  const bin = await binPath()
  const flags = Object.entries(args)
    .filter(([, v]) => v !== undefined && v !== null && v !== '')
    .map(([k, v]) => `--${k} ${shq(v)}`)
    .join(' ')

  // action 只来自代码内的常量，不接受用户输入
  const cmd = `${shq(bin)} toolbox --action ${shq(action)}${flags ? ' ' + flags : ''} --json`

  const resp = await Promise.race([
    sendExec(cmd, timeoutSec),
    new Promise((_, rej) =>
      setTimeout(() => rej(new Error(`宿主执行超时（${timeoutSec}s）`)), timeoutSec * 1000 + 4000),
    ),
  ])
  return unwrap(resp, bin)
}
