import { call } from './request'

export type Part = { key: string; score: number; weight: number; value: number; unit: string; percent: number }
export type ScoreResult = { cpu: number; mem: number; disk: number; total: number; parts: Part[] }
export type TrendPoint = { label: string; score: number }
export type Host = { cpu: string; memory: string; os: string; disk: string }
export type Overview = { host: Host; latest: ScoreResult | null; trend: TrendPoint[] }

export type HistoryEntry = { at: string; label: string; score: number; cpu: number; mem: number; disk: number }

export type ProbeResult = Part & { status: string; reasons: string[]; comparable: boolean; stable: boolean }
export type Preflight = { cpu: { ok: boolean; detail: Record<string, string>; reasons: string[] }; disk: { ok: boolean; detail: Record<string, string>; reasons: string[] }; mem: { ok: boolean; detail: Record<string, string>; reasons: string[] } }
export const getOverview = () => call<Overview>('overview')
export const checkPreflight = () => call<Preflight>('check')
export const probe = (key: 'cpu' | 'mem' | 'disk', force = false) => call<ProbeResult>('probe', { key, ...(force ? { force: true } : {}) })
export const runScore = () => call<ScoreResult>('score')
export const getHistory = () => call<HistoryEntry[]>('history')
export const ping = () => call<string>('ping')
