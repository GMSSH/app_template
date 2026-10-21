import { appConfig } from '../app/config'
import { locale } from '../locales'

const EXEC_SHELL_URL = '/api/command/exec_shell'

const gm = () => (window as unknown as { $gm?: Record<string, any> }).$gm

// GMSSH 真实规范（参考 portMap）：前端每次动作都由宿主执行一次后端 CLI：
// /api/command/exec_shell + cmd_string，后端 main <命令> --json 打印结果后退出。
// 没有 init、没有 socket、没有常驻服务。
export async function call<T = any>(method: string, params: Record<string, any> = {}): Promise<T> {
  const sdk = gm()
  if (!sdk?.request) throw new Error('GMSSH host unavailable: $gm.request is undefined')

  const identity = await loadCliIdentity()
  const bin = cliBin(sdk, identity)
  if (!bin) throw new Error('identity: liteapp.json unavailable')
  const args = Object.entries(params)
    .map(([key, value]) => '--' + key + ' ' + quote(String(value)))
    .join(' ')
  // 后端 i18n 默认 en，带上界面当前语言，让后端错误文案与 UI 一致
  const cmd = [quote(bin), method, '--json', '--lang', quote(locale.value)].concat(args || []).join(' ')

  const resp = await sdk.request(EXEC_SHELL_URL, {
    method: 'post',
    data: { cmd_string: cmd, timeout: '40', cwd: '', user: '' },
  })
  return unwrapExec<T>(resp)
}

// liteapp.json 由前端页面 fetch 读取（和 portMap 参考项目一致），
// 不依赖宿主注入任何字段；无宿主/读取失败时抛错并回退 mock。
let cliIdentity: Record<string, string> | null = null
async function loadCliIdentity(): Promise<Record<string, string>> {
  if (cliIdentity) return cliIdentity
  const res = await fetch('liteapp.json', { cache: 'no-store' })
  if (!res.ok) throw new Error('identity: liteapp.json missing')
  cliIdentity = (await res.json()) || {}
  return cliIdentity
}

// 二进制路径：/.__gmssh/plugin/liteapp/{user_id}/{app_name}/app/bin/main
// user_id / app_name 来自 web/liteapp.json（平台创建应用时填入），
// 不可用自拼的 developer-name / app-name 代替。
function cliBin(sdk: Record<string, any>, identity: Record<string, string>): string {
  const userId = String(identity.user_id ?? appConfig.userId ?? '').trim()
  const appName = String(identity.app_name ?? appConfig.appName ?? '').trim()
  if (!userId || !appName) return ''
  if (userId.indexOf('/') >= 0 || appName.indexOf('/') >= 0) return ''
  if (userId.indexOf('..') >= 0 || appName.indexOf('..') >= 0) return ''
  const home = typeof sdk.gmsshHome === 'string' ? sdk.gmsshHome.replace(/\/$/, '') : ''
  let bin = '/.__gmssh/plugin/liteapp/' + userId + '/' + appName + '/app/bin/main'
  if (home) bin = bin.replace('/.__gmssh', home)
  return bin
}

function quote(value: string): string {
  return "'" + value.replace(/'/g, "'\\''") + "'"
}

function unwrapExec<T>(resp: any): T {
  if (!resp) throw new Error('empty exec response')
  if (typeof resp.code === 'number' && resp.code !== 200000 && resp.code !== 0) {
    throw new Error(resp.msg || resp.message || 'exec failed')
  }
  let text = ''
  if (typeof resp.data === 'string') text = resp.data
  else if (resp.data && typeof resp.data.stdout === 'string') text = resp.data.stdout
  else if (typeof resp.stdout === 'string') text = resp.stdout
  if (!text) throw new Error('exec response has no stdout')
  const trimmed = text.trim()
  if (!trimmed) return {} as T
  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    const at = text.lastIndexOf('result=')
    if (at >= 0) {
      try { parsed = JSON.parse(text.slice(at + 7).trim()) } catch { /* fallthrough */ }
    }
    if (parsed === undefined) throw new Error('backend stdout is not JSON')
  }
  // 后端 CLI 信封：{"ok":true,"data":...} / {"ok":false,"error":{...}}
  if (parsed && typeof parsed === 'object' && 'ok' in parsed) {
    if (parsed.ok) return (parsed.data === undefined ? parsed : parsed.data) as T
    const err = (parsed as { error?: { message?: string } }).error
    throw new Error(err?.message || 'backend command failed')
  }
  return parsed as T
}
