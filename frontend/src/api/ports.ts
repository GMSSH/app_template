import { callCli, isMock } from './request'
import { mockScan, mockReleasePort, delay } from '../mock/data'
import type { PortEntry } from '../types/port'

export async function scanPorts(): Promise<{ list: PortEntry[]; mock: boolean }> {
  if (await isMock()) {
    await delay(600)
    return { list: mockScan(), mock: true }
  }
  return { list: await callCli<PortEntry[]>('ports', { action: 'scan' }), mock: false }
}

export async function releasePort(port: number, protocol: string, pid?: number | null): Promise<{ success: boolean; message?: string }> {
  if (await isMock()) {
    await delay(700)
    const ok = mockReleasePort(port, protocol)
    return { success: ok }
  }
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
