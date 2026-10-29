// GMSSH 执行封装：优先走 window.$gm.command.exec_shell，本地走 dev-server 桥接
function tryParseJSON(text) {
  if (!text) return null
  try {
    return JSON.parse(text)
  } catch {
    return null
  }
}

// CLI 失败时把 {"ok":false,"error":{message}} 写进 stderr；
// 直接 JSON.parse(stdout) 会抛出无意义的 "Unexpected end of JSON input"
export function unwrapCliResult(stdout, stderr) {
  const ok = tryParseJSON(stdout)
  if (ok) {
    if (ok.ok === false) throw new Error(ok.error?.message || '检测失败')
    return ok
  }

  // stderr 里除了 JSON 还可能混有 "exit status 2" 之类的尾行，逐行找那条 JSON
  for (const line of (stderr || '').split('\n')) {
    const parsed = tryParseJSON(line.trim())
    if (parsed?.error?.message) throw new Error(parsed.error.message)
  }

  throw new Error((stderr || '').trim() || '命令未返回任何结果')
}

export async function executeDomainCommand(action, domain = '') {
  let cmdString = `/.__gmssh/plugin/liteapp/jg/domainToolbox/app/bin/main domain --action ${action}`
  if (domain) {
    cmdString += ` --domain ${domain}`
  }
  cmdString += ' --json'

  // If running inside GMSSH host
  if (window.$gm && window.$gm.command && typeof window.$gm.command.exec_shell === 'function') {
    const res = await window.$gm.command.exec_shell({ cmd_string: cmdString })
    const stdout = res?.data?.stdout || res?.stdout || ''
    const stderr = res?.data?.stderr || res?.stderr || ''
    return unwrapCliResult(stdout, stderr)
  }

  // Local dev server fallback
  const res = await fetch('/api/command/exec_shell', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ data: { cmd_string: cmdString } })
  })

  if (!res.ok) {
    throw new Error(`HTTP ${res.status}: ${res.statusText}`)
  }

  const payload = await res.json()
  return unwrapCliResult(payload?.data?.stdout || '', payload?.data?.stderr || '')
}
