<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useApp } from './composables/useApp'
import { useI18n } from 'vue-i18n'
import { scanPorts } from './api/ports'
import { groupPorts, overviewGraph, searchGraph } from './utils/graph'
import type { PortEntry, PortGroup, ProcessDetail } from './types/port'
import PortGraph from './components/PortGraph.vue'
import DetailPanel from './components/DetailPanel.vue'
import StatCard from './components/StatCard.vue'

const { t, locale } = useI18n()
const { windowWidth, windowHeight } = useApp()

/* ---------- 主题 / 语言 ---------- */
const theme = ref<'light' | 'dark'>(window.$gm?.theme === 'dark' ? 'dark' : 'light')
watch(theme, (v) => (document.documentElement.dataset.theme = v), { immediate: true })
const toggleLang = () => (locale.value = locale.value === 'zh-CN' ? 'en-US' : 'zh-CN')

/* ---------- 数据 ---------- */
const entries = ref<PortEntry[]>([])
const loading = ref(false)
const scanning = ref(false)
const isMockData = ref(true)
const errMsg = ref('')
const groups = computed(() => groupPorts(entries.value))

async function refresh() {
  if (loading.value) return
  loading.value = true
  errMsg.value = ''
  scanning.value = false
  setTimeout(() => (scanning.value = true), 20)
  const hadHit = searchPort.value !== null && matched.value.length > 0
  try {
    const r = await scanPorts()
    entries.value = r.list
    isMockData.value = r.mock
    if (procDetail.value) {
      const stillHas = groups.value.filter(
        (g) => g.pid === procDetail.value?.pid && g.processName === procDetail.value?.name
      )
      if (stillHas.length) {
        procDetail.value.ports = stillHas
      } else {
        procDetail.value = null
      }
    }
    releasedPort.value =
      hadHit && searchPort.value !== null && !groups.value.some((g) => g.port === searchPort.value)
        ? searchPort.value
        : null
  } catch (e: any) {
    errMsg.value = e?.message || t('error')
  } finally {
    loading.value = false
  }
}

/* ---------- 自动刷新 ---------- */
const auto = ref(false)
let timer: number | undefined
watch(auto, (on) => {
  clearInterval(timer)
  if (on) timer = window.setInterval(refresh, 30000)
})
onUnmounted(() => clearInterval(timer))

/* ---------- 搜索（精确匹配，数据在内存中过滤） ---------- */
const query = ref('')
const filter = ref<'all' | 'web' | 'db' | 'container' | 'manager' | 'unmanaged' | 'external'>('all')
const releasedPort = ref<number | null>(null)
const parsed = computed(() => {
  const s = query.value.trim().replace(/^:/, '')
  if (!s) return { state: 'empty' as const }
  if (!/^\d+$/.test(s) || +s < 1 || +s > 65535) return { state: 'invalid' as const }
  return { state: 'ok' as const, port: +s }
})
const searchPort = computed(() => (parsed.value.state === 'ok' ? parsed.value.port! : null))
const matched = computed(() =>
  searchPort.value === null ? [] : groups.value.filter((g) => g.port === searchPort.value)
)
watch(query, () => (releasedPort.value = null))

const resolveCategory = (g: any) => {
  if (g.service?.category && g.service.category !== 'unmanaged') return g.service.category
  const cmd = (g.command || '').toLowerCase()
  // 智能识别基于 Python/Node/Go/Java 运行的 Web 网站服务
  if (
    cmd.includes('http.server') || cmd.includes('flask') || cmd.includes('django') ||
    cmd.includes('uvicorn') || cmd.includes('gunicorn') || cmd.includes('fastapi') ||
    cmd.includes('express') || cmd.includes('koa') || cmd.includes('next') ||
    cmd.includes('nuxt') || cmd.includes('vite') || cmd.includes('serve')
  ) {
    return 'web'
  }
  if (g.service?.category) return g.service.category
  if (g.ownerType === 'docker') return 'container'
  if (g.ownerType === 'systemd') return 'system'
  return 'unmanaged'
}

const visible = computed(() =>
  groups.value.filter((g) => {
    const cat = resolveCategory(g)
    switch (filter.value) {
      case 'all': return true
      case 'web': return cat === 'web'
      case 'db': return cat === 'db'
      case 'container': return cat === 'container'
      case 'manager': return cat === 'manager'
      case 'unmanaged': return cat === 'unmanaged' || cat === 'system'
      case 'external': return g.external
      default: return true
    }
  })
)

const graph = computed(() =>
  searchPort.value !== null ? searchGraph(matched.value) : overviewGraph(visible.value)
)
const layoutKey = computed(() => `${searchPort.value}|${filter.value}|${graph.value.nodes.length}`)

const stats = computed(() => ({
  total: groups.value.length,
  tcp: groups.value.filter((g) => g.protocol === 'tcp').length,
  udp: groups.value.filter((g) => g.protocol === 'udp').length,
  external: groups.value.filter((g) => g.external).length,
}))

/* ---------- 进程详情抽屉 ---------- */
const procDetail = ref<ProcessDetail | null>(null)
const selectedProcKey = computed(() =>
  procDetail.value ? `${procDetail.value.name || 'unknown'}#${procDetail.value.pid ?? 'x'}` : null
)
const selectedPortKey = computed(() => procDetail.value?.selectedPortKey ?? null)

const resolveProcDisplayName = (g: PortGroup) => {
  if (g.service?.serviceName && !['python 3', 'python', 'node.js', 'node'].includes(g.service.serviceName.toLowerCase())) {
    return g.service.serviceName
  }
  // 提取工作目录末级作为站点/项目名
  const cwd = (g.workingDirectory || '').replace(/\/+$/, '')
  if (cwd && !['/root', '/home', '/tmp', '/', '/home/ubuntu'].includes(cwd)) {
    const parts = cwd.split('/')
    const folder = parts[parts.length - 1]
    if (folder && !['ubuntu', 'root', 'home'].includes(folder)) {
      return folder
    }
  }
  // 提取脚本文件名
  const cmd = g.command || ''
  const m = cmd.match(/([a-zA-Z0-9_-]+\.(py|js|ts|jar|go))/i)
  if (m && m[1]) return m[1]
  return g.service?.serviceName || g.processName || 'unknown'
}

function onSelectProc(ports: PortGroup[], portKey?: string) {
  if (!ports.length) return
  const first = ports[0]
  const displayName = resolveProcDisplayName(first)
  const cat = resolveCategory(first)
  const mergedService = first.service ? {
    ...first.service,
    category: cat,
    serviceName: displayName,
    summary: displayName !== first.processName ? `基于 ${first.service.serviceName || first.processName} 的应用服务` : first.service.summary,
  } : undefined

  procDetail.value = {
    name: displayName,
    pid: first.pid,
    user: first.user,
    command: first.command,
    workingDirectory: first.workingDirectory,
    ownerType: first.ownerType,
    scannedAt: first.scannedAt,
    service: mergedService,
    ports,
    selectedPortKey: portKey ?? null,
  }
}

watch(matched, (m) => {
  if (searchPort.value !== null && m.length > 0) {
    onSelectProc(m, m[0].key)
  }
})

/* ---------- 状态提示（静默搜索，仅在致命错误或释放时提示） ---------- */
const notice = computed(() => {
  if (errMsg.value) return { kind: 'err', text: errMsg.value }
  if (releasedPort.value !== null) {
    return { kind: 'warn', text: t('released', { p: releasedPort.value }) }
  }
  // 输入搜索时彻底静默，由画布直接实时响应过滤与居中，不再弹出横幅吐司干扰输入
  return null
})

onMounted(refresh)
</script>

<template>
    <div
      class="page"
      :style="{
        width: windowWidth ? windowWidth + 'px' : '100vw',
        height: windowHeight ? windowHeight + 'px' : '100vh',
      }"
    >
      <!-- 顶部工具栏 -->
      <header class="top">
        <div class="searchwrap" :class="{ bad: parsed.state === 'invalid' }">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
            <circle cx="11" cy="11" r="7" />
            <path d="m20 20-3.5-3.5" />
          </svg>
          <input
            v-model="query"
            :placeholder="t('searchPh')"
            inputmode="numeric"
            maxlength="7"
            spellcheck="false"
            class="mono"
          />
          <button v-if="query" class="clear" @click="query = ''">✕</button>
        </div>

        <div class="tools">
          <button class="ibtn" :title="t('lang')" @click="toggleLang">
            {{ locale === 'zh-CN' ? 'EN' : '中' }}
          </button>
          <button class="ibtn" :title="t('theme')" @click="theme = theme === 'dark' ? 'light' : 'dark'">
            <svg v-if="theme === 'light'" width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
            </svg>
            <svg v-else width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <circle cx="12" cy="12" r="4" />
              <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
            </svg>
          </button>

          <div class="seg" :title="t('auto')">
            <span class="seg-l">{{ t('auto') }}</span>
            <button :class="{ on: !auto }" @click="auto = false">{{ t('autoOff') }}</button>
            <button :class="{ on: auto }" @click="auto = true">{{ t('auto30') }}</button>
          </div>

          <!-- 端口雷达实心刷新按钮 -->
          <button class="solid" :class="{ busy: loading }" :disabled="loading" @click="refresh">
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round">
              <path d="M21 12a9 9 0 1 1-3-6.7L21 8" />
              <path d="M21 3v5h-5" />
            </svg>
            <span>{{ t('refresh') }}</span>
          </button>
        </div>
      </header>

      <!-- 状态通知 -->
      <Transition name="fade" mode="out-in">
        <div v-if="notice" :key="notice.text" class="notice" :class="notice.kind">
          <span>{{ notice.text }}</span>
          <small v-if="(notice as any).hint">{{ (notice as any).hint }}</small>
        </div>
      </Transition>

      <!-- 端口雷达四指标卡片 (带环形刻度进度 + 呼吸点 + 角标折角) -->
      <section class="stats">
        <StatCard
          :label="t('stats.total')"
          :value="stats.total"
          :total-count="stats.total"
          :sub-text="'来自 ' + groups.length + ' 个监听服务'"
          variant="total"
        />
        <StatCard
          :label="t('stats.tcp')"
          :value="stats.tcp"
          :total-count="stats.total"
          :sub-text="'面向连接的服务'"
          variant="tcp"
        />
        <StatCard
          :label="t('stats.udp')"
          :value="stats.udp"
          :total-count="stats.total"
          :sub-text="'无连接的数据报'"
          variant="udp"
        />
        <StatCard
          :label="t('stats.external')"
          :value="stats.external"
          :total-count="stats.total"
          :sub-text="stats.external ? '建议检查是否公开' : '没有对外暴露的端口'"
          variant="external"
        />
      </section>

      <!-- 端口雷达 Tab 分类栏 + 图例说明 -->
      <div class="bar">
        <div class="tabs" :class="{ dim: searchPort !== null }">
          <button
            v-for="f in (['all', 'web', 'db', 'container', 'manager', 'unmanaged', 'external'] as const)"
            :key="f"
            :class="{ on: filter === f }"
            @click="filter = f"
          >
            {{ t('filter.' + f) }}
          </button>
        </div>

        <div class="legend">
          <span><i class="kd" style="--c: var(--c-native)" />原生应用</span>
          <span><i class="kd" style="--c: var(--c-systemd)" />Systemd 服务</span>
          <span><i class="kd" style="--c: var(--c-docker)" />Docker 容器</span>
          <span><i class="kd" style="--c: var(--c-daemon)" />守护进程</span>
          <span style="margin-left: 8px"><i class="k" />TCP</span>
          <span><i class="k udp" />UDP</span>
        </div>
      </div>

      <!-- Vue Flow 拓扑主面板 -->
      <section class="board">
        <PortGraph
          :nodes="graph.nodes"
          :edges="graph.edges"
          :selected-proc-key="selectedProcKey"
          :selected-port-key="selectedPortKey"
          :layout-key="layoutKey"
          :scanning="scanning"
          @select-proc="onSelectProc"
          @pane-click="procDetail = null"
        />

        <Transition name="fade">
          <div v-if="loading" class="veil">
            <div class="radar-spinner" />
          </div>
        </Transition>

        <div v-if="!graph.nodes.length && !loading" class="empty">
          <div>
            <b>{{ t('empty') }}</b>
            <span>换个关键词，或切换上方的筛选条件</span>
          </div>
        </div>

        <!-- 进程详情抽屉 -->
        <DetailPanel
          :detail="procDetail"
          :refreshing="loading"
          @close="procDetail = null"
          @requery="refresh"
        />
      </section>
    </div>
</template>

<style scoped>
.app-provider {
  width: 100%;
  height: 100%;
  overflow: hidden;
}

.page {
  box-sizing: border-box;
  padding: 12px 18px 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  overflow: hidden;
  background: var(--bg);
}

/* 顶部搜索与操作栏 */
.top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.searchwrap {
  flex: 1;
  max-width: 460px;
  display: flex;
  align-items: center;
  gap: 10px;
  height: 38px;
  padding: 0 12px 0 14px;
  border-radius: 10px;
  background: var(--panel);
  border: 1px solid var(--line);
  color: var(--mu);
  box-shadow: var(--sh);
  transition: border-color 0.2s, box-shadow 0.2s;
  flex-shrink: 1;
}

.searchwrap:focus-within {
  border-color: var(--tx);
  color: var(--tx);
}

.searchwrap.bad {
  border-color: var(--ac);
  animation: shake 0.35s;
}

@keyframes shake {
  25% { transform: translateX(-4px); }
  75% { transform: translateX(4px); }
}

.searchwrap input {
  flex: 1;
  border: 0;
  outline: 0;
  background: transparent;
  font: inherit;
  font-size: 14px;
  font-weight: 500;
  color: var(--tx);
  min-width: 0;
}

.searchwrap input::placeholder {
  color: var(--mu);
  font-weight: 400;
  font-size: 13.5px;
}

.clear {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--p2);
  color: var(--mu);
  font-size: 11px;
}

.tools {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.ibtn {
  height: 38px;
  width: 38px;
  border-radius: 10px;
  border: 1px solid var(--line);
  background: var(--panel);
  display: grid;
  place-items: center;
  font-size: 13px;
  font-weight: 600;
  color: var(--tx);
  box-shadow: var(--sh);
  transition: border-color 0.2s;
}

.ibtn:hover {
  border-color: var(--line2);
}

.seg {
  display: flex;
  align-items: center;
  gap: 2px;
  height: 38px;
  padding: 3px 4px;
  border-radius: 10px;
  background: var(--panel);
  border: 1px solid var(--line);
  box-shadow: var(--sh);
}

.seg-l {
  font-size: 11.5px;
  color: var(--mu);
  padding: 0 6px;
}

.seg button {
  padding: 4px 8px;
  border-radius: 7px;
  font-size: 12px;
  color: var(--mu);
  transition: all 0.2s;
}

.seg button.on {
  background: var(--tx);
  color: var(--bg);
  font-weight: 600;
}

/* 端口雷达实心刷新按钮 */
.solid {
  height: 38px;
  padding: 0 16px;
  border: 0;
  border-radius: 10px;
  background: var(--tx);
  color: var(--bg);
  font-weight: 600;
  font-size: 14px;
  display: flex;
  gap: 8px;
  align-items: center;
  box-shadow: var(--sh);
  transition: opacity 0.2s;
}

.solid:hover {
  opacity: 0.85;
}

.solid svg {
  transition: transform 0.6s cubic-bezier(0.2, 0.8, 0.2, 1);
}

.solid.busy svg {
  transform: rotate(360deg);
}

/* 状态提示 (浮动 Toast，实体高对比底色 + 毛玻璃遮挡底层) */
.notice {
  position: absolute;
  top: 68px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 100;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  padding: 8px 22px;
  border-radius: 12px;
  font-size: 13px;
  font-weight: 600;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15), 0 1px 3px rgba(0, 0, 0, 0.1);
  pointer-events: none;
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  max-width: 90vw;
  text-align: center;
}

.notice small {
  font-size: 11.5px;
  font-weight: 400;
  opacity: 0.92;
}

.notice.ok {
  background: #f0fdf4;
  color: #15803d;
  border: 1px solid #bbf7d0;
}
:root[data-theme="dark"] .notice.ok {
  background: #142a1f;
  color: #4ade80;
  border: 1px solid #1e4632;
}

.notice.warn {
  background: #fffbeb;
  color: #b45309;
  border: 1px solid #fde68a;
}
:root[data-theme="dark"] .notice.warn {
  background: #2a1f10;
  color: #fbbf24;
  border: 1px solid #483416;
}

.notice.err {
  background: #fef2f2;
  color: #b91c1c;
  border: 1px solid #fecaca;
}
:root[data-theme="dark"] .notice.err {
  background: #2d1515;
  color: #f87171;
  border: 1px solid #4c1d1d;
}

.notice.info {
  background: #eff6ff;
  color: #1d4ed8;
  border: 1px solid #bfdbfe;
}
:root[data-theme="dark"] .notice.info {
  background: #13213d;
  color: #60a5fa;
  border: 1px solid #1e3a6e;
}

/* 四个指标卡片网格 */
.stats {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
  flex-shrink: 0;
}

/* 端口雷达 Tab 栏与图例 */
.bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  flex-shrink: 0;
}

.tabs {
  display: flex;
  gap: 4px;
}

.tabs.dim {
  opacity: 0.35;
  pointer-events: none;
}

.tabs button {
  border: 1px solid transparent;
  background: none;
  padding: 6px 14px;
  border-radius: 999px;
  font-size: 13.5px;
  color: var(--mu);
  font-weight: 500;
  transition: 0.2s;
}

.tabs button:hover {
  color: var(--tx);
}

.tabs button.on {
  border-color: var(--line2);
  background: var(--panel);
  color: var(--tx);
  font-weight: 600;
  box-shadow: 0 1px 3px rgba(15, 20, 25, 0.05);
}

.legend {
  display: flex;
  gap: 16px;
  font-size: 12px;
  color: var(--mu);
  align-items: center;
}

.legend span {
  display: flex;
  gap: 6px;
  align-items: center;
}

.kd {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c);
}

.k {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--tx);
}

.k.udp {
  background: none;
  border: 1.5px solid var(--tx);
}

/* Vue Flow 画布容器 */
.board {
  position: relative;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  border-radius: 20px;
}

.veil {
  position: absolute;
  inset: 0;
  z-index: 6;
  display: grid;
  place-items: center;
  background: color-mix(in srgb, var(--panel) 75%, transparent);
  backdrop-filter: blur(2px);
}

.radar-spinner {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  border: 2.5px solid var(--line);
  border-top-color: var(--ac);
  animation: spin 0.75s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.empty {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  text-align: center;
  color: var(--mu);
  z-index: 3;
  font-size: 13px;
  pointer-events: none;
}

.empty b {
  display: block;
  font-size: 16px;
  color: var(--tx);
  margin-bottom: 4px;
  font-weight: 600;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s cubic-bezier(0.2, 0.8, 0.2, 1), transform 0.3s cubic-bezier(0.2, 0.8, 0.2, 1);
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>
