// exec_shell 封装：cmd_string 构造、响应解包、错误转换
export interface Identity { user_id: string; app_name: string }

let identity: Identity | null = null

export async function loadIdentity(): Promise<Identity> {
  if (identity) return identity
  try {
    const r = await fetch('./liteapp.json', { cache: 'no-store' })
    if (r.ok) {
      identity = await r.json()
    }
  } catch {
    // ignore
  }
  if (!identity || !identity.user_id) {
    identity = { user_id: 'jg', app_name: 'portMap' }
  }
  return identity!
}

export async function isMock(): Promise<boolean> {
  const sdk = window.$gm
  return !sdk?.request
}

const q = (v: string) => "'" + String(v).replace(/'/g, "'\\''") + "'"

export async function callCli<T>(command: string, args: Record<string, string | number> = {}): Promise<T> {
  const sdk = window.$gm
  if (!sdk?.request) {
    throw new Error('GMSSH SDK not available')
  }

  const id = await loadIdentity()
  const home = typeof (sdk as any).gmsshHome === 'string' ? (sdk as any).gmsshHome.replace(/\/$/, '') : ''
  let bin = `/.__gmssh/plugin/liteapp/${id.user_id}/${id.app_name}/app/bin/main`
  if (home) {
    bin = bin.replace('/.__gmssh', home)
  }

  const lang = document.documentElement.lang || sdk?.lang || 'zh-CN'
  const flags = Object.entries({ ...args, lang }).map(([k, v]) => `--${k} ${q(String(v))}`).join(' ')
  const cmd_string = `${q(bin)} ${command} ${flags} --json`

  const resp = await sdk.request('/api/command/exec_shell', {
    method: 'post',
    data: { cmd_string, timeout: '30', cwd: '', user: '' },
  })

  return unwrapExec<T>(resp, bin)
}

function unwrapExec<T>(resp: any, binPath: string): T {
  if (!resp) throw new Error('exec_shell: response is empty')

  // 如果请求报错或者返回错误码
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
  else if (typeof resp.stderr === 'string') stderr = resp.stderr

  // 如果执行报错 "No such file or directory" 或者找不到文件
  const combined = (stderr + '\n' + text).trim()
  if (combined.includes('No such file') || combined.includes('command not found') || combined.includes('not found')) {
    throw new Error(`Backend not deployed: expected at ${binPath}`)
  }
  if (combined.includes('Permission denied')) {
    throw new Error(`Backend not executable: run chmod +x ${binPath}`)
  }

  const trimmed = text.trim()
  if (!trimmed) {
    if (stderr.trim()) throw new Error(stderr.trim())
    return {} as T
  }

  // 尝试从 stdout 中提取 JSON
  let parsed: any
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    // 兼容可能混杂了前面日志的 stdout
    const startIdx = trimmed.indexOf('{')
    const endIdx = trimmed.lastIndexOf('}')
    if (startIdx >= 0 && endIdx > startIdx) {
      try {
        parsed = JSON.parse(trimmed.slice(startIdx, endIdx + 1))
      } catch {
        // ignore
      }
    }
    if (!parsed) {
      throw new Error(stderr.trim() || trimmed)
    }
  }

  // 校验 gmssh-cli-plugin JSON 信封: {"ok":true,"data":...}
  if (parsed && typeof parsed === 'object' && 'ok' in parsed) {
    if (parsed.ok) {
      return (parsed.data === undefined ? parsed : parsed.data) as T
    }
    const err = parsed.error
    throw new Error(err?.message || 'CLI command returned error')
  }

  return parsed as T
}