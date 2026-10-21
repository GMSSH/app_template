import type { Edge, Node } from '@vue-flow/core'
import type { PortEntry, PortGroup } from '../types/port'

const isLocalAddr = (a: string) => a.startsWith('127.') || a === '::1'
export const isExternal = (a: string) => !isLocalAddr(a)

/** 同协议同端口合并：多个绑定地址并成一条 */
export function groupPorts(list: PortEntry[]): PortGroup[] {
  const map = new Map<string, PortGroup>()
  for (const e of list) {
    const key = `${e.port}/${e.protocol}`
    const g = map.get(key)
    if (g) {
      if (!g.addresses.includes(e.address)) g.addresses.push(e.address)
      g.external ||= isExternal(e.address)
      if (!g.service && e.service) g.service = e.service
      continue
    }
    map.set(key, {
      key, protocol: e.protocol, port: e.port, addresses: [e.address], state: e.state, pid: e.pid,
      processName: e.processName, user: e.user, command: e.command, workingDirectory: e.workingDirectory,
      ownerType: e.ownerType, scannedAt: e.scannedAt, external: isExternal(e.address),
      service: e.service,
    })
  }
  return [...map.values()].sort((a, b) => a.port - b.port || a.protocol.localeCompare(b.protocol))
}

const procKey = (g: PortGroup) => {
  const sName = g.service?.serviceName || g.processName || 'unknown'
  return `${sName}#${g.pid ?? 'x'}`
}
const CHIPS_PER_ROW = 3

/** 总览：监听地址 → 服务进程（端口作为节点里的标签） */
export function overviewGraph(groups: PortGroup[]): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = [], edges: Edge[] = []
  let y = 0
  for (const scope of ['external', 'local'] as const) {
    const inScope = groups.filter((g) => (scope === 'external' ? g.external : !g.external))
    if (!inScope.length) continue
    const procs = new Map<string, PortGroup[]>()
    inScope.forEach((g) => procs.set(procKey(g), [...(procs.get(procKey(g)) ?? []), g]))
    const top = y
    const addrId = `addr-${scope}`
    for (const [k, gs] of procs) {
      const h = 100 + Math.ceil(gs.length / CHIPS_PER_ROW) * 36
      const first = gs[0]
      const resolveDisplayName = (g: PortGroup) => {
  if (g.service?.serviceName && g.service.serviceName !== 'Python 3' && g.service.serviceName !== 'Node.js' && g.service.serviceName !== 'python3' && g.service.serviceName !== 'node') {
    return g.service.serviceName
  }
  // 提取项目/站点名：若工作目录是 /home/ubuntu/web9999 则提取出 "web9999"
  const cwd = (g.workingDirectory || '').replace(/\/+$/, '')
  if (cwd && !['/root', '/home', '/tmp', '/', '/home/ubuntu'].includes(cwd)) {
    const parts = cwd.split('/')
    const folder = parts[parts.length - 1]
    if (folder && !['ubuntu', 'root', 'home'].includes(folder)) {
      return folder
    }
  }
  // 提取脚本文件名：如 app.py, main.js
  const cmd = g.command || ''
  const m = cmd.match(/([a-zA-Z0-9_-]+\.(py|js|ts|jar|go))/i)
  if (m && m[1]) return m[1]

  return g.service?.serviceName || g.processName || 'unknown'
}
const displayName = resolveDisplayName(first)
      nodes.push({
        id: `proc-${scope}-${k}`,
        type: 'proc',
        position: { x: 390, y },
        data: {
          name: displayName,
          pid: first.pid,
          ports: gs,
          ownerType: first.ownerType,
          service: first.service,
        },
      })
      edges.push({ id: `e-${addrId}-${k}`, source: addrId, target: `proc-${scope}-${k}`, type: 'default', animated: scope === 'external' })
      y += h + 42
    }
    nodes.push({ id: addrId, type: 'addr', position: { x: 0, y: (top + y - 24) / 2 - 40 }, data: { scope, label: scope === 'external' ? '0.0.0.0' : '127.0.0.1', count: inScope.length } })
    y += 64
  }
  return { nodes, edges }
}

/** 搜索：监听地址 → 端口 → 进程，只留匹配项 */
export function searchGraph(groups: PortGroup[]): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = [], edges: Edge[] = []
  const addrs = [...new Set(groups.flatMap((g) => g.addresses))]
  addrs.forEach((a, i) => nodes.push({ id: `a-${a}`, type: 'addr', position: { x: 0, y: i * 160 + (groups.length * 180 - addrs.length * 160) / 2 }, data: { scope: isExternal(a) ? 'external' : 'local', label: a, count: groups.filter((g) => g.addresses.includes(a)).length } }))
  const seenProc = new Set<string>()
  groups.forEach((g, i) => {
    nodes.push({ id: `p-${g.key}`, type: 'port', position: { x: 340, y: i * 180 + 10 }, data: { group: g } })
    g.addresses.forEach((a) => edges.push({ id: `e-a-${a}-${g.key}`, source: `a-${a}`, target: `p-${g.key}`, type: 'default', animated: true }))
    const pk = `${procKey(g)}-${g.key}`
    if (!seenProc.has(pk)) {
      seenProc.add(pk)
      const displayName = g.service?.serviceName || g.processName || 'unknown'
      nodes.push({
        id: `r-${pk}`,
        type: 'proc',
        position: { x: 690, y: i * 180 },
        data: {
          name: displayName,
          pid: g.pid,
          ports: [g],
          ownerType: g.ownerType,
          service: g.service,
          solo: true,
        },
      })
      edges.push({ id: `e-p-${pk}`, source: `p-${g.key}`, target: `r-${pk}`, type: 'default', animated: true })
    }
  })
  return { nodes, edges }
}
