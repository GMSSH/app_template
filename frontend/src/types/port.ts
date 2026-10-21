export interface DockerInfo {
  containerId?: string
  containerName?: string
  image?: string
  internalPort?: string
  cmd?: string
}

export interface SystemdInfo {
  unitName?: string
  description?: string
  activeState?: string
  subState?: string
  mainPid?: number
  execStart?: string
  fragmentPath?: string
  workingDirectory?: string
  memoryCurrent?: string
  cpuUsageNSec?: string
}

export interface ServiceMeta {
  category: 'web' | 'db' | 'container' | 'manager' | 'unmanaged' | 'system'
  serviceName: string
  summary?: string
  uptime?: string
  memoryRss?: string
  docker?: DockerInfo
  systemd?: SystemdInfo
}

export interface PortEntry {
  protocol: 'tcp' | 'udp'
  port: number
  address: string
  state: string
  pid: number | null
  processName: string
  user: string
  command: string
  workingDirectory: string
  ownerType: 'process' | 'docker' | 'systemd' | 'unknown'
  ownerName: string
  scannedAt: string
  service?: ServiceMeta
}

/** 同端口同协议合并后的结果（多地址合并为一条） */
export interface PortGroup {
  key: string
  protocol: 'tcp' | 'udp'
  port: number
  addresses: string[]
  state: string
  pid: number | null
  processName: string
  user: string
  command: string
  workingDirectory: string
  ownerType: PortEntry['ownerType']
  scannedAt: string
  external: boolean
  service?: ServiceMeta
}

/** 进程节点详情：归集同一进程下的所有端口 */
export interface ProcessDetail {
  name: string
  pid: number | null
  user: string
  command: string
  workingDirectory: string
  ownerType: PortEntry['ownerType']
  ports: PortGroup[]
  scannedAt: string
  service?: ServiceMeta
  selectedPortKey?: string | null
}
