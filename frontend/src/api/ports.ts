import { callCli } from './request'
import type { PortEntry } from '../types/port'

export async function scanPorts(): Promise<PortEntry[]> {
  return callCli<PortEntry[]>('ports', { action: 'scan' })
}

export async function releasePort(port: number, protocol: string, pid?: number | null): Promise<{ success: boolean; message?: string }> {
  try {
    if (pid && pid > 1) {
      await callCli('ports', { action: 'kill', pid })
    } else {
      await callCli('ports', { action: 'release', port, proto: protocol })
    }
    return { success: true }
  } catch (err: any) {
    return { success: false, message: err?.message || 'Failed' }
  }
}
