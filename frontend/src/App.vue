<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watchEffect, type Component } from 'vue'

import { Cpu, HardDrive, MemoryStick, Network } from '@lucide/vue'
import LanguageToggle from './components/LanguageToggle.vue'
import ThemeToggle from './components/ThemeToggle.vue'
import RunScorePanel from './components/RunScorePanel.vue'
import RoastCard from './components/RoastCard.vue'
import ScoreCard from './components/ScoreCard.vue'
import TrendChart from './components/TrendChart.vue'
import { useApp } from './composables/useApp'
import { appTheme, isDark, toggleTheme } from './app/theme'
import { currentMessages, locale, toggleLocale, t } from './locales'
import { messages } from './locales/messages'
import { checkPreflight, getHistory, getOverview, probe, runScore, type Part } from './api/score'

type MetricKey = 'cpu' | 'memory' | 'disk' | 'network'
type Metric = { score: number; percent: number; value: string; unit: string; color: string; icon: Component }
type TrendPoint = { label: string; score: number }

const { windowHeight, windowWidth } = useApp()

const metrics = ref<Record<MetricKey, Metric>>({
  cpu: { score: 0, percent: 0, value: '--', unit: '', color: '#2f8cff', icon: Cpu },
  memory: { score: 0, percent: 0, value: '--', unit: 'GB/s', color: '#8b6cff', icon: MemoryStick },
  disk: { score: 0, percent: 0, value: '--', unit: 'GB/s', color: '#24bf8b', icon: HardDrive },
  network: { score: 0, percent: 0, value: '--', unit: 'ms', color: '#f3a642', icon: Network },
})
const totalScore = ref(0)
const lastParts = ref<number[]>([0, 0, 0])
const isRunning = ref(false)
const roastScore = ref<number | null>(null)
const roastOpen = ref(false)
const roastParts = ref<number[]>([])
const activeMetric = ref<MetricKey | null>(null)
const completedAt = ref('')
const trend = ref<TrendPoint[]>([])
const stages: { key: MetricKey; duration: number; title: string }[] = [
  { key: 'cpu', duration: 18, title: 'testingCpu' },
  { key: 'memory', duration: 22, title: 'testingMemory' },
  { key: 'disk', duration: 20, title: 'testingDisk' },
]
const panelKeys: MetricKey[] = ['cpu', 'memory', 'disk']
const host = ref({ cpu: '', memory: '', os: '', disk: '' })
let stageTimer: number | undefined
// overview 请求中：服务器信息面板显示骨架屏
const hostLoading = ref(true)
// overview 失败时：显示错误而不是空白
const hostError = ref('')
// 开始预检：点「开始跑分」→ 面板不动，弹 loading → 调后端 check
// 空闲 → 直接开跑；忙 → 弹二次确认，确认后 force 开跑，取消则退回待机
const startChecking = ref(false)
const startConfirm = ref(false)
// 用户在开始确认框点了「继续」：后续 probe 全部 force，跑分中途不再打断
const forceRun = ref(false)

// 不再调用 $gm.init：GMSSH 真实规范（portMap）是宿主按动作 exec_shell
// 执行后端 CLI，无需 init 也不常驻。


function syncStage(key: MetricKey | null) {
  if (stageTimer) window.clearInterval(stageTimer)
  stageTimer = undefined
  activeMetric.value = key
  if (!key) return
  stageTimer = window.setInterval(() => {
    metrics.value[key].percent = Math.min(98, metrics.value[key].percent + 1)
  }, 280)
}

function applyPart(part: Part) {
  const key = part.key === 'mem' ? 'memory' : (part.key as MetricKey)
  if (!metrics.value[key]) return
  metrics.value[key] = { ...metrics.value[key], score: part.score, percent: part.percent, value: fmtValue(part.value), unit: part.unit }
}

// 真实后端返回长浮点（如 23516.763896567012），按量级截短显示，防止溢出
function fmtValue(v: number): string {
  if (!isFinite(v)) return '--'
  const a = Math.abs(v)
  if (a >= 1000) return String(Math.round(v))
  if (a >= 100) return String(Math.round(v * 10) / 10)
  return String(Math.round(v * 100) / 100)
}

async function loadOverview() {
  hostLoading.value = true
  try {
    const overview = await getOverview()
    host.value = overview.host || { cpu: '', memory: '', os: '', disk: '' }
    if (overview.trend?.length) trend.value = overview.trend.map((point) => ({ label: point.label, score: point.score }))
    overview.latest?.parts?.forEach(applyPart)
    if (overview.latest) totalScore.value = overview.latest.total
  } catch (error) {
    hostError.value = error instanceof Error ? error.message : String(error)
  } finally {
    hostLoading.value = false
  }
}

// 跑分完成后：调后端 score 命令确认最终总分，再调 history 刷新趋势。
// 任一失败都保留面板已有结果，不影响交互。
async function syncAfterScore() {
  try {
    const result = await runScore()
    totalScore.value = result.total
    result.parts?.forEach(applyPart)
    if (result.parts?.length) lastParts.value = result.parts.map((p) => p.score)
    completedAt.value = new Date().toLocaleString([], { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
  } catch { /* 后端不可用时保留本地结果。 */ }
  try {
    const history = await getHistory()
    if (history?.length) trend.value = history.slice(-7).map((entry) => ({ label: entry.label, score: entry.score }))
  } catch { /* 同上。 */ }
}

async function runPanelItem(key: 'cpu' | 'mem' | 'disk') {
  roastScore.value = null
  roastParts.value = []
  roastOpen.value = false
  const metricKey: MetricKey = key === 'mem' ? 'memory' : key
  syncStage(metricKey)
  try {
    let part = await probe(key, forceRun.value)
    // 负载中途升高：自动 force 重测一次，不打断跑分，也不给 0 分
    if (part.status === 'refused' && (part.reasons || []).includes('high_load')) {
      part = await probe(key, true)
    }
    applyPart(part)
    return part.score
  } catch { /* 后端不可用：返回 0，不生成任何假数据。 */ }
  return 0
}
// 开始流程：loading 预检 → 空闲直接开跑 / 忙弹确认
async function askRun() {
  startChecking.value = true
  let busy = false
  try {
    const pf = await checkPreflight()
    busy = [pf.cpu, pf.mem, pf.disk].some((x) => x?.ok === false)
  } catch {
    busy = false // 后端不可用也直接开跑，探针自己会返回 0
  }
  startChecking.value = false
  if (busy) {
    startConfirm.value = true
  } else {
    panelRef.value?.beginRun()
  }
}
function confirmStart(ok: boolean) {
  startConfirm.value = false
  if (!ok) return
  forceRun.value = true // 用户已知风险，本轮全部 probe force，中途不再弹框
  panelRef.value?.beginRun()
}
const panelRef = ref<{ beginRun: () => void } | null>(null)

function onPanelFinish(result: { total: number; parts: number[] }) {
  roastScore.value = result.total
  roastParts.value = result.parts
  syncStage(null)
  totalScore.value = result.total
  lastParts.value = result.parts
  isRunning.value = false
  // 面板本地结果先落地，随后调后端 score/history 校准总分和趋势。
  void syncAfterScore()
}

function setLocale(nextLocale: 'en-US' | 'zh-CN') {
  if (nextLocale !== locale.value) toggleLocale()
  document.documentElement.lang = locale.value
}

// 标签页标题跟随当前语言
watchEffect(() => { document.title = currentMessages.value.appTitle })

onMounted(() => {
  loadOverview()
})
onBeforeUnmount(() => { if (stageTimer) window.clearInterval(stageTimer) })
</script>

<template>
  <div class="app-scroll" :style="{ height: windowHeight ? windowHeight + 'px' : '100vh', width: windowWidth ? windowWidth + 'px' : '100vw' }">
    <main class="app-shell" :data-theme="appTheme">
          <header class="topbar">
            <div class="top-actions">
              <ThemeToggle :dark="isDark" :light-label="currentMessages.lightTheme" :dark-label="currentMessages.theme" @toggle="toggleTheme" />
              <LanguageToggle :locale="locale" @change="setLocale" />
            </div>
          </header>

          <RunScorePanel ref="panelRef" :score="totalScore" :last-parts="lastParts" :has-card="roastScore !== null" :run-item="runPanelItem" :ask-run="true" @finish="onPanelFinish" @open-card="roastOpen = true" @ask-run="askRun" />
          <Transition name="roast">
            <div v-if="startChecking" class="busy-overlay">
              <div class="busy-box" role="dialog" aria-modal="true">
                <div class="busy-mascot">
                  <span class="busy-pixel" v-for="i in 4" :key="i" />
                </div>
                <h3>{{ currentMessages.checkingTitle }}</h3>
                <p>{{ currentMessages.checkingDetail }}</p>
              </div>
            </div>
          </Transition>
          <Transition name="roast">
            <div v-if="startConfirm" class="busy-overlay" @click="confirmStart(false)">
              <div class="busy-box" role="dialog" aria-modal="true" @click.stop>
                <div class="busy-mascot">
                  <span class="busy-pixel" v-for="i in 4" :key="i" />
                </div>
                <h3>{{ currentMessages.startTitle }}</h3>
                <p>{{ currentMessages.startDetail }}</p>
                <div class="busy-actions">
                  <button class="btn-pixel ghost" type="button" @click="confirmStart(false)">{{ currentMessages.cancelText }}</button>
                  <button class="btn-pixel" type="button" @click="confirmStart(true)">{{ currentMessages.startConfirm }}</button>
                </div>
              </div>
            </div>
          </Transition>

          <Transition name="roast">
            <RoastCard v-if="roastOpen" :score="roastScore || 0" :parts="roastParts" :time="completedAt" @close="roastOpen = false" />
          </Transition>

          <section class="metrics-grid">
            <ScoreCard v-for="key in panelKeys" :key="key" :label="currentMessages[key]" :value="metrics[key].value" :unit="metrics[key].unit || currentMessages.unitPts" :percent="metrics[key].percent" :color="metrics[key].color" :icon="metrics[key].icon" :active="activeMetric === key" />
          </section>

          <section class="dashboard-grid">
            <article class="sheet trend-panel">
              <div class="panel-heading"><h2>{{ currentMessages.trend }}</h2><span class="period-pill">{{ currentMessages.last7 }}</span></div>
              <TrendChart :points="trend" />
            </article>
            <article class="sheet server-panel">
              <div class="panel-heading"><h2>{{ currentMessages.server }}</h2><span class="server-dot" /></div>
              <div v-if="hostLoading" class="host-skeleton" aria-busy="true">
                <span class="host-skeleton-line" v-for="n in 4" :key="n" />
              </div>
              <p v-else-if="hostError" class="host-error">{{ currentMessages.backendError }}：{{ hostError }}</p>
              <dl v-else><div><dt>{{ currentMessages.cores }}</dt><dd>{{ host.cpu }}</dd></div><div><dt>{{ currentMessages.memorySize }}</dt><dd>{{ host.memory }}</dd></div><div><dt>{{ currentMessages.operatingSystem }}</dt><dd>{{ host.os }}</dd></div><div><dt>{{ currentMessages.storage }}</dt><dd>{{ host.disk }}</dd></div></dl>
            </article>
          </section>


    </main>
  </div>
</template>