<template>
  <section class="panel" :class="[state, 's' + scene, { dark: isDark }]">
    <div class="glow"></div>
    <div class="orbit a"></div>
    <div class="orbit b"></div>
    <div class="sweep"></div>
    <div class="sparks"><i v-for="(p, i) in sparks" :key="i" :style="{ left: p.left + '%', top: p.top + '%', width: p.s + 'px', height: p.s + 'px', animationDuration: p.dur + 's', animationDelay: p.delay + 's' }"></i></div>
  
    <div class="left" :class="state">
      <div class="eyebrow-group">
        <div class="eyebrow">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 17a8 8 0 1 1 16 0M12 17l4-5"/></svg>
          {{ currentMessages.overall }}
        </div>
        <p class="eyebrow-sub">{{ currentMessages.overallSubtitle }}</p>
      </div>
      <div class="hero">
        <div class="rwrap">
          <svg class="rings" viewBox="0 0 144 144">
            <defs>
              <linearGradient id="ring-cpu" x1="0" y1="0" x2="1" y2="1">
                <stop offset="0" stop-color="#3B82F6" />
                <stop offset="1" stop-color="#6388FF" />
              </linearGradient>
              <linearGradient id="ring-mem" x1="0" y1="0" x2="1" y2="1">
                <stop offset="0" stop-color="#8B5CF6" />
                <stop offset="1" stop-color="#A18AFF" />
              </linearGradient>
              <linearGradient id="ring-disk" x1="0" y1="0" x2="1" y2="1">
                <stop offset="0" stop-color="#10B981" />
                <stop offset="1" stop-color="#34D3AD" />
              </linearGradient>
            </defs>
            <g v-for="r in rings" :key="r.key" class="rg" :class="{ act: r.act }" :style="{ '--ic': r.color }">
              <circle class="rt" cx="72" cy="72" :r="r.r" />
              <circle class="rp" cx="72" cy="72" :r="r.r" :stroke="'url(#ring-' + r.key + ')'" :stroke-dasharray="r.c" :stroke-dashoffset="r.off" :opacity="r.has ? 1 : 0" />
              <g v-if="r.act" class="spin"><circle class="rs" cx="72" cy="72" :r="r.r" :stroke="'url(#ring-' + r.key + ')'" :stroke-dasharray="r.c * 0.16 + ' ' + r.c" /></g>
            </g>
          </svg>
          <div class="rc"><b>{{ Math.round(pos) }}%</b><small>{{ currentMessages.scoreRate }}</small></div>
        </div>
        <div class="sb">
          <div class="score" :class="{ bump: bumped }">{{ state === 'idle' ? '--' : Math.round(shown) }}</div>
          <div v-if="state !== 'idle'" class="meta">
            <small class="max">/ {{ max }}</small>
            <div v-if="state !== 'running'" :key="state" class="grade" :class="'g-' + grade.g"><b>{{ grade.g }}</b><span>{{ grade.txt }}</span></div>
            <span v-if="state === 'done'" class="delta" :class="delta > 0 ? 'up' : delta < 0 ? 'down' : 'flat'"><em>{{ currentMessages.vsLast }}</em><b>{{ delta === 0 ? currentMessages.flat : (delta > 0 ? '+' : '−') + Math.abs(delta) }}</b></span>
          </div>
          <div v-else class="meta idle-meta">
            <span class="idle-hint">{{ currentMessages.clickToStart }}</span>
          </div>
        </div>
      </div>
      <div class="status-row">
        <div class="status-main">
          <span v-if="state !== 'idle'" class="dot" :class="{ pulse: state === 'running' }"></span>
          <div class="status-texts">
            <span class="status-head">{{ statusText }}</span>
            <span v-if="state === 'running'" class="status-desc">{{ currentMessages.benchmarkSubtext }}</span>
          </div>
        </div>
        <div v-if="state !== 'idle'" class="pill-timer">{{ currentMessages.elapsed }} {{ elapsed.toFixed(1) }}s</div>
      </div>

      <div class="btn-morph-area" :class="{ solo: state === 'running' || !hasCard }">
        <button class="action-capsule" :class="{ running: state === 'running' }"
                :disabled="state === 'running'" @click="start($event)">
          <div class="capsule-content">
            <div class="progress-ring-icon" v-if="state === 'running'">
              <svg viewBox="0 0 24 24"><circle class="bg" cx="12" cy="12" r="9" /><circle class="bar" cx="12" cy="12" r="9" :stroke-dasharray="56.5" :stroke-dashoffset="56.5 * (1 - progress)" /></svg>
            </div>
            <span class="capsule-label">
              {{ state === 'running' ? currentMessages.running + ' ' + Math.round(progress * 100) + '%' : state === 'done' ? currentMessages.rerun : currentMessages.start }}
            </span>

          </div>
          <div class="capsule-fill" :style="{ width: progress * 100 + '%' }"></div>
        </button>
        <button v-if="hasCard" class="card-open" type="button" @click="openCard"><span class="card-glow"></span><span class="card-label">{{ currentMessages.roastOpen }}</span></button>
      </div>

      <transition name="live-fade">
        <div v-if="state === 'running'" class="live-strip-card">
          <div class="strip-header">
            <div class="strip-title">
              <span class="task-accent">{{ items[phase].label }}</span>
              <span class="strip-dot">·</span>
              <span>{{ taskNames[phase][subIdx % 3] }}</span>
            </div>
            <div class="strip-elapsed">{{ currentMessages.elapsed }} {{ elapsed.toFixed(1) }}s</div>
          </div>
          <div class="strip-bars">
            <i v-for="b in bars" :key="b.i" :style="{ '--d': b.d, '--t': b.t + 's', '--h': b.h }"></i>
          </div>
          <div class="strip-footer">
            <span>{{ currentMessages.realtimeThroughput }}</span>
            <span>{{ currentMessages.past60s }}</span>
          </div>
        </div>
      </transition>
    </div>
  
    <div v-if="banner" :key="banner" class="banner">{{ banner }}</div>
  
    <div class="track">
      <div class="line"><div class="fill" :style="{ width: pos + '%' }"></div></div>
      <div v-for="z in zones" :key="z.key" class="zone"
           :class="{ act: state === 'running' && results[z.i] === null && phase === z.i }"
           :style="{ left: z.l + '%', width: z.w + '%' }"></div>
      <div v-for="m in marks" :key="m.i" class="ms" :class="{ hit: results[m.i] !== null }" :style="{ left: m.pct + '%' }">
        <div v-if="m.i === 2" class="mv"><svg class="flag-red" viewBox="0 0 16 20"><path d="M2 19V1M2 2h11l-2.2 3.2L13 8.4H2" fill="#ef4444" stroke="#b91c1c" stroke-width="1" stroke-linejoin="round"/></svg>{{ m.v }}</div>
        <div v-else class="mv">{{ m.v }}</div>
        <div class="dia"></div>
      </div>

  
      <div class="runner" :style="{ left: pos + '%' }">
        <i v-for="n in 3" :key="n" class="dust" :style="{ animationDelay: n * 0.14 + 's' }"></i>
        <i v-if="gain" :key="'r' + gain.key" class="ring"></i>
        <div v-if="gain" :key="gain.key" class="gain">+{{ gain.v }}</div>
        <div class="body" :class="{ hop: state === 'done', poke: poked }" @click="clickBody" :title="currentMessages.poke">
          <div v-if="state === 'idle'" class="pixel-bubble">
            <b>{{ currentMessages.notStarted }}</b>
            <i></i>
          </div>
          <div v-else-if="state === 'done' && hasCard" class="pixel-bubble card-bubble" :title="currentMessages.roastOpen">
            <b>{{ currentMessages.roastOpen }}</b>
            <i></i>
          </div>
          <div v-else-if="state !== 'idle'" class="bubble">{{ Math.round(shown) }}</div>
          <svg viewBox="0 0 10 14" width="40" height="56" shape-rendering="crispEdges">
            <rect v-for="(p, i) in pixels" :key="i" :x="p.x" :y="p.y" width="1" height="1" :style="{ fill: p.fill }" />
          </svg>
        </div>
      </div>
    </div>
  
    <div class="rail">
      <div v-for="z in zones" :key="z.key" class="card"
           :class="{ act: state === 'running' && results[z.i] === null && phase === z.i, done: results[z.i] !== null }"
           :style="{ left: 'calc(' + z.l + '% + 3px)', width: 'calc(' + z.w + '% - 6px)', '--ic': z.color, animationDelay: 0.3 + z.i * 0.08 + 's' }"
           :title="results[z.i] === null ? z.label + ' ' + currentMessages.fullMark + ' ' + z.max : z.label + ' ' + results[z.i] + ' / ' + z.max + ' (' + Math.round(((results[z.i] || 0) / z.max) * 100) + '%)'">
        <div class="top">
          <span class="ic"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" v-html="icon(z.icon)"></svg></span>
          <span class="nm"><b>{{ z.label }}</b><small>{{ z.sub }}</small></span>
          <span class="sc">
            <span v-if="state === 'running' && results[z.i] === null && phase === z.i" class="scan"><i></i><i></i><i></i></span>
            <em v-else>{{ results[z.i] === null ? '—' : Math.round(partShown[z.i]) }}</em>
            <small>/ {{ z.max }}</small>
          </span>
        </div>
        <div class="bar"><i :style="{ width: ((results[z.i] || 0) / z.max) * 100 + '%' }"></i></div>
      </div>
    </div>
  
    <i v-for="(r, i) in ribbons" :key="i" class="ribbon"
       :style="{ left: r.left + '%', width: r.w + 'px', background: r.color, animationDelay: r.delay + 's', animationDuration: r.dur + 's' }"></i>
  </section>
</template>

<script lang="ts">
import { defineComponent, ref, computed, onBeforeUnmount, watch, type PropType } from 'vue'
import { isDark } from '../app/theme'
import { currentMessages } from '../locales'

type State = 'idle' | 'running' | 'done'
interface Item { key: 'cpu' | 'mem' | 'disk'; label: string; sub: string; icon: string; color: string }

// 像素小人:10x14,字符 -> 颜色
const PALETTE: Record<string, string> = {
  h: '#2b1d3a', s: '#f6c9a0', e: '#111827', b: 'var(--ac)', p: '#1d2a5c', w: '#e5ebf8',
}
const BODY = [
  '..hhhhh...', '.hhhhhhh..', '.hsssesh..', '..ssssss..', '...bbbb...',
  '..bbbbbb..', '.sbbbbbbs.', '..bbbbbb..', '..pppppp..',
]
const LEGS: Record<string, string[]> = {
  idle: ['..pp..pp..', '..pp..pp..', '..pp..pp..', '..ww..ww..', '..........'],
  a: ['..pp..pp..', '.pp....pp.', 'pp......pp', 'ww......ww', '..........'],
  b: ['...pppp...', '...pp.pp..', '...pp.pp..', '..ww..ww..', '..........'],
  c: ['...pp.pp..', '..pp...pp.', '..pp...ww.', '..ww......', '..........'],
}
const CYCLE = ['a', 'b', 'c', 'b'] // 跑步循环帧

// 线性图标(静态内联 SVG 片段)
const ICONS: Record<string, string> = {
  cpu: '<rect x="6" y="6" width="12" height="12" rx="2"/><rect x="10" y="10" width="4" height="4"/><path d="M9 2v4M15 2v4M9 18v4M15 18v4M2 9h4M2 15h4M18 9h4M18 15h4"/>',
  mem: '<rect x="2" y="7" width="20" height="10" rx="1.5"/><path d="M6 11v2M10 11v2M14 11v2M18 11v2M5 17v3M9 17v3M13 17v3M17 17v3"/>',
  disk: '<ellipse cx="12" cy="6" rx="8" ry="3"/><path d="M4 6v6c0 1.7 3.6 3 8 3s8-1.3 8-3V6M4 12v6c0 1.7 3.6 3 8 3s8-1.3 8-3v-6"/>',
  flag: '<path d="M5 22V3M5 4h12l-2 4 2 4H5"/>',
}
const ITEM_META: { key: Item['key']; icon: string; color: string }[] = [
  { key: 'cpu', icon: 'cpu', color: '#3b82f6' },
  { key: 'mem', icon: 'mem', color: '#8b6cff' },
  { key: 'disk', icon: 'disk', color: '#22c08a' },
]
const COLORS = ['#ffd23f', '#ffe066', '#ffb703', '#f9c74f', '#fff3b0']
interface Ribbon { left: number; delay: number; dur: number; w: number; color: string }
const sleep = (ms: number) => new Promise<void>(r => setTimeout(r, ms))
const sparks = Array.from({ length: 14 }, () => ({
  left: Math.random() * 100, top: 50 + Math.random() * 45, s: 2 + Math.floor(Math.random() * 3),
  dur: 5 + Math.random() * 6, delay: -Math.random() * 8,
}))
const bars = Array.from({ length: 36 }, (_, i) => ({ i, d: Math.random(), t: 0.45 + Math.random() * 0.6, h: 0.35 + Math.random() * 0.65 }))
const MIN_MS = 2400 // 每一项最少展示时长,保证动画观感

export default defineComponent({
  name: 'RunScorePanel',
  props: {
    /** 上次综合分:待机时小人停在对应位置 */
    score: { type: Number, default: 0 },
    /** 上次三项得分(待机时展示),合计应等于 score */
    lastParts: { type: Array as PropType<number[]>, default: () => [0, 0, 0] },
    /** 三项满分权重 [CPU, 内存, 磁盘],合计即赛道总分 */
    weights: { type: Array as PropType<number[]>, default: () => [4000, 3000, 3000] },
    /** 接入真实测试:返回该项得分;不传则模拟 */
    runItem: { type: Function as PropType<(key: Item['key']) => Promise<number>> },
    hasCard: { type: Boolean, default: false },
    /** 弹窗等待确认时暂停动画（小人/计时冻结） */
    paused: { type: Boolean, default: false },
    /** 开始跑分时先弹确认框：面板不自己开跑，只发 askRun 让 App 弹窗 */
    askRun: { type: Boolean, default: false },
  },
  emits: { finish: (r: { total: number; parts: number[] }) => !!r, openCard: () => true, askRun: () => true },
  setup(props, { emit }) {
    const state = ref<State>('idle')
    const shown = ref(props.score) // 当前总分(小人位置 = 总分 / 满分)
    const phase = ref(0) // 当前测试项 0~2
    const progress = ref(0) // 按钮进度 0~1
    const results = ref<(number | null)[]>(props.lastParts.slice())
    const frameIdx = ref(0)
    const ribbons = ref<Ribbon[]>([])
    const banner = ref('')
    const gain = ref<{ v: number; key: number } | null>(null)
    const poked = ref(false)
    const hold = ref(false) // 弹窗等待确认：面板退回 idle 展示，确认后恢复跑分
    const final = ref(props.score) // 最近一次综合分
    const prev = ref(props.score)
    const partShown = ref<number[]>(props.lastParts.slice())
    const bumped = ref(false)
    const rip = ref<{ x: number; y: number; k: number } | null>(null)
    const elapsed = ref(0)
    const subIdx = ref(0)
    let gainKey = 0
    let ripKey = 0
    let alive = true
    let raf = 0
    let timer = 0
    let clearT = 0

    const max = computed(() => props.weights.reduce((a, b) => a + b, 0))
    const pos = computed(() => Math.min(100, (shown.value / max.value) * 100))
    // 背景场景:待机/第1项=0,第2项=1,第3项=2,完成=3
    const scene = computed(() => (state.value === 'idle' ? 0 : state.value === 'done' ? 3 : phase.value))

    const copy = () => currentMessages.value
    const items = computed<Item[]>(() => {
      const m = copy()
      return [
        { key: 'cpu', label: m.cpu, sub: m.cpuAbility, icon: 'cpu', color: '#3b82f6' },
        { key: 'mem', label: m.memory, sub: m.memAbility, icon: 'mem', color: '#8b6cff' },
        { key: 'disk', label: m.disk, sub: m.diskAbility, icon: 'disk', color: '#22c08a' },
      ]
    })
    const taskNames = computed(() => {
      const m = copy() as Record<string, string>
      const safe = (key: string, fallback: string) => (typeof m[key] === 'string' && m[key] ? m[key] : fallback)
      return [
        [safe('cpuTask1', 'cpu-1'), safe('cpuTask2', 'cpu-2'), safe('cpuTask3', 'cpu-3')],
        [safe('memTask1', 'mem-1'), safe('memTask2', 'mem-2'), safe('memTask3', 'mem-3')],
        [safe('diskTask1', 'disk-1'), safe('diskTask2', 'disk-2'), safe('diskTask3', 'disk-3')],
      ]
    })
    const zones = computed(() => {
      let acc = 0
      return items.value.map((it, i) => {
        const w = props.weights[i]
        const z = { ...it, i, max: w, l: (acc / max.value) * 100, w: (w / max.value) * 100 }
        acc += w
        return z
      })
    })
    const RR = [64, 52, 40] // 144px 基准下的三环半径:CPU 在最外圈
    const rings = computed(() => items.value.map((it, i) => {
      const c = 2 * Math.PI * RR[i]
      const ratio = Math.min(1, Math.max(0, partShown.value[i] / props.weights[i]))
      return { key: it.key, color: it.color, r: RR[i], c, off: c * (1 - ratio), has: ratio > 0.002,
        act: state.value === 'running' && results.value[i] === null && phase.value === i }
    }))
    const marks = computed(() => {
      let acc = 0
      return props.weights.map((w, i) => ({ i, v: (acc += w), pct: (acc / max.value) * 100 }))
    })
    const pixels = computed(() => {
      const key = state.value === 'running' ? CYCLE[frameIdx.value % CYCLE.length] : 'idle'
      const out: { x: number; y: number; fill: string }[] = []
      ;[...BODY, ...LEGS[key]].forEach((row, y) =>
        [...row].forEach((c, x) => { if (PALETTE[c]) out.push({ x, y, fill: PALETTE[c] }) }))
      return out
    })
    const grade = computed(() => {
      const r = final.value / max.value
      const m = copy()
      return r >= 0.9 ? { g: 'S', txt: m.gradeS } : r >= 0.75 ? { g: 'A', txt: m.gradeA }
        : r >= 0.6 ? { g: 'B', txt: m.gradeB } : r >= 0.4 ? { g: 'C', txt: m.gradeC } : { g: 'D', txt: m.gradeD }
    })
    const delta = computed(() => Math.round(final.value - prev.value))
    const deltaText = computed(() => {
      const m = copy()
      return delta.value > 0 ? `▲ ${m.vsLast} +${delta.value}` : delta.value < 0 ? `▼ ${m.vsLast} ${delta.value}` : `— ${m.flat}`
    })
    function clickBody() {
      if (state.value === 'done' && props.hasCard) {
        openCard()
        return
      }
      if (state.value === 'running' || poked.value) return
      poked.value = true
      setTimeout(() => { poked.value = false }, 520)
    }
    const liveTitle = computed(() =>
      state.value === 'running' ? `${items.value[phase.value].label} · ${taskNames.value[phase.value][subIdx.value % 3]}`
        : state.value === 'done' ? copy().runFinished : copy().readyNow)
    const liveMeta = computed(() =>
      state.value === 'idle' ? copy().clickToStart : `${state.value === 'running' ? copy().elapsed : copy().duration} ${elapsed.value.toFixed(1)}s`)
    const icon = (n?: string) => (n ? ICONS[n] : '')
    const statusText = computed(() => {
      if (state.value === 'running') return `${copy().testing} ${items.value[phase.value].label}…`
      if (state.value === 'done') return copy().completed
      return copy().notStarted
    })

    function tween(from: number, to: number, set: (v: number) => void, ms = 500) {
      return new Promise<void>(res => {
        const t0 = performance.now()
        const step = (now: number) => {
          if (!alive) return res()
          const k = Math.min(1, (now - t0) / ms)
          set(from + (to - from) * (1 - Math.pow(1 - k, 3)))
          if (k < 1) raf = requestAnimationFrame(step)
          else res()
        }
        raf = requestAnimationFrame(step)
      })
    }

    async function measure(i: number): Promise<number> {
      const key = ITEM_META[i].key
      try {
        const [raw] = await Promise.all([
          props.runItem ? props.runItem(key) : Promise.resolve(props.weights[i] * (0.4 + Math.random() * 0.55)),
          sleep(MIN_MS),
        ])
        return Math.max(0, Math.min(props.weights[i], Math.round(raw)))
      } catch {
        return 0
      }
    }

    // launch 是纯开跑入口（App 弹窗确认后调用）；start 是按钮入口（askRun 模式只发事件）
    async function launch(e?: MouseEvent) {
      if (state.value === 'running' || hold.value) return
      if (e) {
        const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
        rip.value = e.detail === 0
          ? { x: r.width / 2, y: r.height / 2, k: ++ripKey }
          : { x: e.clientX - r.left, y: e.clientY - r.top, k: ++ripKey }
      }
      state.value = 'running'
      prev.value = final.value
      gain.value = null
      shown.value = 0
      progress.value = 0
      phase.value = 0
      results.value = [null, null, null]
      partShown.value = [0, 0, 0]
      ribbons.value = []
      banner.value = copy().go
      clearTimeout(clearT)
      const t0 = performance.now()
      elapsed.value = 0
      subIdx.value = 0
      timer = window.setInterval(() => {
        frameIdx.value++
        elapsed.value = (performance.now() - t0) / 1000
        if (frameIdx.value % 7 === 0) subIdx.value++
      }, 110)
      await sleep(600)

      let total = 0
      for (let i = 0; i < ITEM_META.length; i++) {
        phase.value = i
        progress.value = (i + 0.85) / ITEM_META.length
        const v = await measure(i)
        if (!alive) return
        total += v
        const next = results.value.slice()
        next[i] = v
        results.value = next
        banner.value = `✓ ${items.value[i].label} ${copy().done}`
        gain.value = { v, key: ++gainKey }
        progress.value = (i + 1) / ITEM_META.length
        bumped.value = true
        setTimeout(() => { bumped.value = false }, 450)
        // 小人只按这一项的真实得分前进,卡片数字同步滚动
        await Promise.all([
          tween(shown.value, total, x => { shown.value = x }),
          tween(0, v, x => { const a = partShown.value.slice(); a[i] = x; partShown.value = a }),
        ])
        await sleep(450)
        if (!alive) return
      }

      clearInterval(timer)
      elapsed.value = (performance.now() - t0) / 1000
      final.value = total
      state.value = 'done'
      emit('finish', { total, parts: results.value.map(v => v ?? 0) })
      ribbons.value = Array.from({ length: 42 }, () => ({
        left: Math.random() * 100, delay: Math.random() * 1.2, dur: 2.6 + Math.random() * 1.8,
        w: 6 + Math.random() * 6, color: COLORS[Math.floor(Math.random() * COLORS.length)],
      }))
      clearT = window.setTimeout(() => { ribbons.value = [] }, 6000)
    }


    onBeforeUnmount(() => {
      alive = false
      cancelAnimationFrame(raf)
      clearInterval(timer)
      clearTimeout(clearT)
    })
    function openCard() { emit('openCard') }
    function start(e?: MouseEvent) {
      if (state.value === 'running' || hold.value) return
      if (props.askRun) { emit('askRun'); return }
      void launch(e)
    }
    return { state, shown, pos, max, phase, progress, results, pixels, items, statusText, start, scene, ribbons, banner, icon, zones, marks, gain, poked, poke: clickBody, clickBody, grade, delta, deltaText, sparks, bars, partShown, rings, bumped, rip, liveTitle, liveMeta, isDark, currentMessages, elapsed, subIdx, taskNames, openCard, beginRun: launch }
  },
})
</script>

<style scoped>
@property --ac{syntax:'<color>';inherits:true;initial-value:#3b82f6}
@property --ac2{syntax:'<color>';inherits:true;initial-value:#6b8cff}
.panel{--rs-bg:#fbfcfe;--rs-text:#0f1b33;--rs-sub:#6b7a99;--rs-line:rgba(15,30,70,.14);--rs-track:rgba(15,30,70,.08);--rs-card:rgba(255,255,255,.72);--rs-card-b:rgba(15,30,70,.08);--rs-shadow:0 1px 2px rgba(15,30,70,.04),0 14px 34px rgba(15,30,70,.07);
  --ac:#3b82f6;--ac2:#6b8cff;transition:--ac .8s,--ac2 .8s;
  position:relative;display:flex;flex-direction:column;overflow:hidden;isolation:isolate;min-height:420px;border-radius:20px;color:var(--rs-text);background:var(--rs-bg);border:1px solid var(--rs-card-b);box-shadow:var(--rs-shadow);
  font-family:-apple-system,BlinkMacSystemFont,"SF Pro Display","PingFang SC","Microsoft YaHei",system-ui,sans-serif}
.panel.dark{--rs-bg:#111a2b;--rs-text:#f2f5fb;--rs-sub:#8a98b5;--rs-line:rgba(255,255,255,.14);--rs-track:rgba(255,255,255,.1);--rs-card:rgba(255,255,255,.04);--rs-card-b:rgba(255,255,255,.09);--rs-shadow:0 1px 0 rgba(255,255,255,.04) inset}
.s1{--ac:#8b6cff;--ac2:#b79cff}.s2{--ac:#22c08a;--ac2:#5ee0b5}.s3{--ac:#3b82f6;--ac2:#6b8cff}
/* 氛围:单色柔光 + 轨道线(呼应原设计) */
.glow{position:absolute;inset:0;background:radial-gradient(55% 90% at 88% 15%,color-mix(in srgb,var(--ac) 20%,transparent),transparent 70%),radial-gradient(40% 70% at 8% 105%,color-mix(in srgb,var(--ac) 9%,transparent),transparent 70%)}
.orbit{position:absolute;border-radius:50%;border:1px solid color-mix(in srgb,var(--ac) 26%,transparent);pointer-events:none;animation:spin 80s linear infinite}
.orbit.a{--r:-14deg;width:780px;height:300px;right:-140px;top:-70px}
.orbit.b{--r:16deg;width:560px;height:220px;right:-50px;top:30px;animation-direction:reverse}
@keyframes spin{from{transform:rotate(var(--r))}to{transform:rotate(calc(var(--r) + 360deg))}}
.left{position:relative;z-index:3;flex:1;display:flex;flex-direction:column;width:min(440px,100%);padding:30px 34px 16px;box-sizing:border-box}
.eyebrow-group{margin-bottom:4px}
.eyebrow{display:flex;align-items:center;gap:8px;font-size:18px;font-weight:700;letter-spacing:.3px;color:var(--rs-text)}
.eyebrow svg{width:20px;height:20px;color:var(--ac)}
.eyebrow-sub{margin:4px 0 0 28px;font-size:12px;color:var(--rs-sub);letter-spacing:.2px}
.hero{display:flex;align-items:center;gap:24px;margin:16px 0 16px}
.rwrap{position:relative;flex:none;width:132px;height:132px;filter:drop-shadow(0 8px 16px rgba(15,27,51,.08))}
.rings{position:relative;z-index:1;width:100%;height:100%;overflow:visible}
.rt{fill:none;stroke:color-mix(in srgb,var(--ic) 14%,var(--rs-track));stroke-width:8;opacity:.85}
.rp{fill:none;stroke-width:8;stroke-linecap:round;transform:rotate(-90deg);transform-origin:72px 72px;filter:drop-shadow(0 1px 2px color-mix(in srgb,var(--ic) 22%,transparent));transition:stroke-dashoffset .5s cubic-bezier(.2,.8,.2,1),filter .3s ease}
.rg.act .rp{filter:drop-shadow(0 0 8px var(--ic)) drop-shadow(0 1px 3px color-mix(in srgb,var(--ic) 45%,transparent))}
.rs{fill:none;stroke-width:4;stroke-linecap:round;opacity:.68}
.spin{transform-origin:72px 72px;animation:rot 1.1s linear infinite}
@keyframes rot{to{transform:rotate(360deg)}}
.rc{position:absolute;z-index:2;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center;line-height:1.1}
.rc b,.rc small{position:relative}
.rc b{font-size:26px;font-weight:700;letter-spacing:-.5px;font-variant-numeric:tabular-nums;color:var(--rs-text)}
.rc small{font-size:11px;color:var(--rs-sub);margin-top:4px;letter-spacing:.2px}
.sb{min-width:0;display:flex;flex-direction:column;justify-content:center}
.score{font-size:62px;line-height:1;font-weight:800;letter-spacing:-2.5px;font-variant-numeric:tabular-nums;color:transparent;background:linear-gradient(135deg,#5c8eff 0%,#8b9dff 45%,#a899ff 100%);-webkit-background-clip:text;background-clip:text;text-shadow:0 6px 18px rgba(92,142,255,.18)}
.idle-meta{min-height:32px}
.idle-hint{font-size:13px;font-weight:600;color:var(--rs-sub);letter-spacing:.3px}
.meta{display:flex;align-items:center;flex-wrap:wrap;gap:8px 10px;margin-top:8px;min-height:24px}
.max{font-size:15px;font-weight:600;color:var(--rs-sub);letter-spacing:-.3px}
.grade{display:flex;align-items:center;gap:7px;padding:3px 11px 3px 3px;border-radius:999px;font-size:12px;color:var(--rs-sub);border:1px solid var(--rs-card-b);background:var(--rs-card);--gc:#6fb6ff}
.grade b{display:grid;place-items:center;width:22px;height:22px;border-radius:50%;font-size:13px;color:#10203f;background:var(--gc)}
.g-S{--gc:#ffc83d}.g-A{--gc:#5ee0b5}.g-B{--gc:#8bb8ff}.g-C{--gc:#ffb36b}.g-D{--gc:#ff8a8a}
.delta{display:inline-flex;align-items:baseline;gap:5px;font-size:12px;color:var(--rs-sub)}
.delta em{font-style:normal}.delta b{font-size:13px;font-variant-numeric:tabular-nums}
.delta.up b{color:#22c08a}.delta.down b{color:#ff6369}.delta.flat b{color:var(--rs-sub)}
.beat-note{display:flex;align-items:center;gap:5px;margin-top:10px;font-size:12px;color:var(--rs-sub);line-height:1.2}
.info-ic{width:13px;height:13px;color:var(--rs-sub);opacity:.75;cursor:help;flex:none}

.status-row{display:flex;align-items:center;justify-content:space-between;gap:12px;margin:12px 0 14px}
.status-main{display:flex;align-items:center;gap:10px;min-width:0}
.dot{width:9px;height:9px;border-radius:50%;background:#22c08a;box-shadow:0 0 0 3px rgba(34,192,138,.2);flex:none}
.dot.pulse{background:#3b82f6;box-shadow:0 0 0 4px rgba(59,130,246,.25);animation:pulse 1s ease-in-out infinite}
@keyframes pulse{50%{transform:scale(.7);opacity:.55}}
.status-texts{display:flex;flex-direction:column;gap:2px;min-width:0}
.status-head{font-size:14px;font-weight:700;color:var(--rs-text);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.status-desc{font-size:11px;color:var(--rs-sub);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.pill-timer{flex:none;padding:5px 12px;border-radius:999px;font-size:12px;font-weight:600;font-variant-numeric:tabular-nums;color:var(--rs-sub);background:var(--rs-card);border:1px solid var(--rs-card-b);box-shadow:0 1px 2px rgba(15,27,51,.04)}

.btn-morph-area{position:relative;display:flex;align-items:center;gap:8px;margin-bottom:12px;transition:gap .35s ease}
.btn-morph-area.solo{gap:0}
.btn-morph-area .action-capsule{flex:1;width:auto;transition:flex .35s ease,background .2s ease,box-shadow .15s ease,transform .15s ease}
/* 像素风按钮:直角 + 硬描边 + 偏移点阵阴影,按下时阴影回缩位移 */
.card-open{position:relative;flex:none;width:132px;overflow:hidden;white-space:nowrap;height:44px;padding:0 14px;border:2px solid #3b82f6;border-radius:0;background:var(--rs-bg);color:#3b82f6;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-weight:700;font-size:13px;letter-spacing:.5px;cursor:pointer;box-shadow:4px 4px 0 0 rgba(59,130,246,.22);transition:width .35s ease,opacity .35s ease,padding .35s ease,background .2s ease,box-shadow .15s ease,transform .15s ease}
.card-open:not(.solo-off){animation:card-pop .5s cubic-bezier(.2,1.4,.4,1) both}
.card-open:hover{background:color-mix(in srgb,#3b82f6 8%,var(--rs-bg));box-shadow:6px 6px 0 0 rgba(59,130,246,.28);transform:translate(-2px,-2px)}
.card-open:active{box-shadow:0 0 0 0 rgba(59,130,246,.22);transform:translate(4px,4px)}
.btn-morph-area.solo .card-open{width:0;opacity:0;padding:0;border-width:0;pointer-events:none}
.card-glow{position:absolute;inset:0;background:linear-gradient(115deg,transparent 30%,rgba(96,165,250,.65) 50%,transparent 70%);transform:translateX(-130%);pointer-events:none}
.card-open:not(.solo-off) .card-glow{animation:card-shine 2.4s ease-in-out 1.1s infinite}
.card-label{position:relative;z-index:2;display:block;width:132px;text-align:center;animation:card-label-pop .5s cubic-bezier(.2,1.4,.4,1) both}
@keyframes card-pop{0%{transform:scale(.6);opacity:0;box-shadow:0 0 0 0 rgba(59,130,246,0)}55%{transform:scale(1.12);opacity:1}75%{transform:scale(.96)}100%{transform:scale(1);opacity:1;box-shadow:4px 4px 0 0 rgba(59,130,246,.22)}}
@keyframes card-label-pop{0%{opacity:0;transform:translateY(6px)}60%{opacity:0}100%{opacity:1;transform:none}}
@keyframes card-shine{0%{transform:translateX(-130%)}55%{transform:translateX(130%)}100%{transform:translateX(130%)}}
.action-capsule{position:relative;overflow:hidden;display:flex;align-items:center;justify-content:center;width:100%;height:44px;box-sizing:border-box;border:2px solid #1e40af;border-radius:0;background:#2563eb;color:#fff;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-weight:700;cursor:pointer;box-shadow:4px 4px 0 0 rgba(30,64,175,.25);transition:flex .35s ease,background .2s ease,box-shadow .15s ease,transform .15s ease}
.action-capsule:not(.running):hover{background:#1d4ed8;box-shadow:6px 6px 0 0 rgba(30,64,175,.32);transform:translate(-2px,-2px)}
.action-capsule:not(.running):active{box-shadow:0 0 0 0 rgba(30,64,175,.25);transform:translate(4px,4px)}
.action-capsule.running{cursor:default;transform:none;border-color:rgba(99,102,241,.4);background:linear-gradient(135deg,rgba(37,99,235,.22) 0%,rgba(79,70,229,.22) 100%);box-shadow:4px 4px 0 0 rgba(15,27,51,.12),inset 0 1px 0 rgba(255,255,255,.08)}
.capsule-content{position:relative;z-index:2;display:flex;align-items:center;width:100%;height:100%;padding:0 16px;gap:10px}
.capsule-label{font-size:14px;font-weight:700;letter-spacing:.5px;color:#fff;font-family:ui-monospace,SFMono-Regular,Menlo,monospace}

.arrow-right{width:16px;height:16px;margin-left:auto;color:rgba(255,255,255,.75);flex:none}
.progress-ring-icon{position:relative;width:20px;height:20px;flex:none}
.progress-ring-icon svg{width:100%;height:100%;transform:rotate(-90deg)}
.progress-ring-icon .bg{fill:none;stroke:rgba(255,255,255,.25);stroke-width:2.5}
.progress-ring-icon .bar{fill:none;stroke:#60a5fa;stroke-width:2.5;stroke-linecap:round;transition:stroke-dashoffset .3s ease}
.capsule-fill{position:absolute;left:0;top:0;bottom:0;z-index:1;background:linear-gradient(90deg,rgba(59,130,246,.32),rgba(99,102,241,.45));transition:width .4s ease-out;pointer-events:none}

.live-strip-card{position:relative;display:flex;flex-direction:column;gap:8px;padding:12px 14px;border-radius:12px;background:var(--rs-card);border:1px solid var(--rs-card-b);box-shadow:0 6px 16px rgba(15,27,51,.06);backdrop-filter:blur(8px)}
.strip-header{display:flex;align-items:center;justify-content:space-between;font-size:12px}
.strip-title{display:flex;align-items:center;gap:6px;font-weight:600;color:var(--rs-text)}
.task-accent{color:#3b82f6}
.strip-dot{color:var(--rs-sub);opacity:.6}
.strip-elapsed{font-variant-numeric:tabular-nums;color:var(--rs-sub)}
.strip-bars{display:flex;align-items:flex-end;gap:3px;height:28px;padding:2px 0}
.strip-bars i{flex:1;max-width:7px;height:100%;border-radius:2px;background:linear-gradient(180deg,#818cf8 0%,#6366f1 60%,rgba(99,102,241,.25) 100%);transform-origin:bottom;transform:scaleY(.18);animation:stripBreathe 2.4s ease-in-out infinite;animation-delay:calc(var(--d) * -2.4s)}
.running .strip-bars i{animation:stripEq var(--t) ease-in-out infinite alternate;animation-delay:calc(var(--d) * -1s)}
@keyframes stripBreathe{50%{transform:scaleY(.4)}}
@keyframes stripEq{from{transform:scaleY(.15)}to{transform:scaleY(var(--h))}}
.strip-footer{display:flex;align-items:center;justify-content:space-between;font-size:11px;color:var(--rs-sub)}
.live-fade-enter-active,.live-fade-leave-active{transition:all .35s cubic-bezier(.2,.8,.2,1)}
.live-fade-enter-from,.live-fade-leave-to{opacity:0;transform:translateY(8px)}
.banner{position:absolute;z-index:4;top:72px;right:20px;padding:7px 14px;border-radius:999px;font-weight:600;font-size:13px;color:var(--rs-text);background:var(--rs-card);border:1px solid var(--rs-card-b);backdrop-filter:blur(10px);box-shadow:var(--rs-shadow);animation:pop 2s ease forwards}
@keyframes pop{0%{transform:translateY(-8px) scale(.9);opacity:0}12%{transform:none;opacity:1}80%{opacity:1}100%{opacity:0}}
/* 赛道 */
.track{position:absolute;z-index:2;left:40%;right:6%;bottom:100px;height:24px}
.line{position:absolute;left:0;right:0;top:10px;height:6px;border-radius:999px;background:var(--rs-track);box-shadow:inset 0 1px 2px rgba(15,27,51,.08)}
.fill{position:absolute;left:0;top:0;bottom:0;border-radius:999px;background:linear-gradient(90deg,#60a5fa,var(--ac));box-shadow:0 0 8px color-mix(in srgb,var(--ac) 45%,transparent);overflow:hidden;transition:width .4s linear}
.fill::after{content:"";position:absolute;inset:0;background:linear-gradient(90deg,transparent,rgba(255,255,255,.7),transparent);transform:translateX(-100%)}
.running .fill::after{animation:bus .8s linear infinite}
.fill::after{content:"";position:absolute;inset:0;opacity:0;background:repeating-linear-gradient(90deg,transparent 0 18px,rgba(255,255,255,.55) 18px 24px);background-size:24px 100%;animation:bus .5s linear infinite}
.running .fill::after{opacity:1}
@keyframes bus{to{background-position-x:-24px}}
.zone{position:absolute;top:0;height:0}
.zone.act::before{content:"";position:absolute;left:0;right:0;top:-3px;height:10px;border-radius:5px;background:var(--ac);opacity:.2;animation:zp 1.1s ease-in-out infinite}
@keyframes zp{50%{opacity:.06}}
.ms{position:absolute;top:0;width:0}
.dia{position:absolute;left:-5px;top:8px;width:10px;height:10px;border-radius:50%;background:var(--rs-bg);border:2px solid var(--rs-line);box-shadow:0 1px 3px rgba(15,27,51,.08);transition:background-color .4s linear,border-color .4s linear,transform .4s linear}
.flag-red{width:14px;height:16px}

.hit .dia{background:#22c08a;border-color:#22c08a}
.mv{position:absolute;bottom:22px;left:0;transform:translateX(-50%);display:flex;align-items:center;gap:4px;font-size:12px;font-weight:600;color:var(--rs-sub)}
.mv svg{width:14px;height:14px}.hit .mv{color:var(--rs-text)}
.runner{position:absolute;bottom:16px;width:0;transition:left .4s linear}
.body{position:absolute;left:0;bottom:0;transform:translateX(-50%);display:flex;flex-direction:column;align-items:center;cursor:pointer}
.body svg{filter:drop-shadow(0 0 1px rgba(15,27,51,.55)) drop-shadow(0 3px 3px rgba(15,27,51,.18))}
.running .body{animation:bob .24s steps(2) infinite}
@keyframes bob{50%{transform:translate(-50%,-3px)}}
.body.hop{animation:hop .5s ease-out 3}.body.poke{animation:hop .5s ease-out 1}
@keyframes hop{50%{transform:translate(-50%,-26px)}}
.bubble{position:relative;margin-bottom:14px;padding:3px 9px;border-radius:10px;background:var(--rs-bg);color:var(--rs-text);font-weight:700;font-size:12px;line-height:1.2;font-variant-numeric:tabular-nums;border:1px solid var(--rs-card-b);box-shadow:0 4px 10px rgba(15,27,51,.12)}
.bubble::after{content:"";position:absolute;left:50%;bottom:-5px;width:8px;height:8px;background:var(--rs-bg);border-right:1px solid var(--rs-card-b);border-bottom:1px solid var(--rs-card-b);transform:translateX(-50%) rotate(45deg)}
/* 像素风未开始气泡:直角、双层描边、点阵阴影 */
.pixel-bubble{position:relative;margin-bottom:12px;padding:4px 8px 4px;background:var(--rs-bg);color:var(--rs-text);font-size:10px;font-weight:700;line-height:1.3;white-space:nowrap;border:2px solid var(--rs-text);box-shadow:3px 3px 0 0 color-mix(in srgb, var(--rs-text) 25%, transparent);image-rendering:pixelated;animation:pixel-bubble-in .22s steps(4) both}
.card-bubble{color:#2563eb;border-color:#3b82f6;box-shadow:3px 3px 0 0 rgba(59,130,246,.35);cursor:pointer;animation:card-bubble-bob 1s steps(2) infinite}
.card-bubble:hover{background:color-mix(in srgb,#3b82f6 10%,var(--rs-bg))}
@keyframes pixel-bubble-in{from{opacity:0;transform:translateY(5px) scale(.85)}to{opacity:1;transform:none}}
@keyframes card-bubble-bob{0%,100%{transform:translateY(0)}50%{transform:translateY(-3px)}}
.pixel-bubble b{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;letter-spacing:.5px}
.pixel-bubble::after{content:"";position:absolute;left:50%;bottom:-8px;width:10px;height:10px;background:var(--rs-bg);border-right:2px solid var(--rs-text);border-bottom:2px solid var(--rs-text);transform:translateX(-50%) rotate(45deg)}
.pixel-bubble i{position:absolute;inset:2px;border:1px dashed color-mix(in srgb, var(--rs-text) 30%, transparent);pointer-events:none}
.dust{position:absolute;bottom:1px;left:-18px;width:5px;height:5px;background:var(--rs-sub);opacity:0}
.running .dust{animation:dust .5s linear infinite}
@keyframes dust{from{opacity:.6;transform:translate(0,0)}to{opacity:0;transform:translate(-24px,-8px)}}
.gain{position:absolute;left:0;bottom:88px;font-weight:700;font-size:20px;color:var(--ac);white-space:nowrap;animation:gain 1.6s ease-out forwards}
@keyframes gain{0%{opacity:0;transform:translate(-50%,10px)}15%{opacity:1}100%{opacity:0;transform:translate(-50%,-36px)}}
/* 测试项目卡片 */
.rail{position:absolute;z-index:3;left:40%;right:6%;bottom:16px;height:68px}
.card{position:absolute;top:0;height:100%;box-sizing:border-box;padding:9px 12px 8px;border-radius:14px;display:flex;flex-direction:column;justify-content:space-between;
  background:var(--rs-card);border:1px solid var(--rs-card-b);backdrop-filter:blur(8px);transition:transform .2s,border-color .4s,box-shadow .4s}
.card:hover{transform:translateY(-2px)}
.top{display:flex;align-items:center;gap:9px}
.ic{display:grid;place-items:center;flex:none;width:30px;height:30px;border-radius:9px;color:var(--ic);background:color-mix(in srgb,var(--ic) 14%,transparent)}
.ic svg{width:17px;height:17px}
.nm{display:flex;flex-direction:column;min-width:0;line-height:1.2}.nm b{font-size:13px}.nm small{font-size:10px;color:var(--rs-sub);white-space:nowrap}
.sc{margin-left:auto;text-align:right;white-space:nowrap;line-height:1.1}
.sc em{display:block;font-style:normal;font-size:20px;font-weight:700;font-variant-numeric:tabular-nums;color:var(--rs-sub)}
.sc small{display:block;font-size:10px;color:var(--rs-sub);margin-top:1px}
.bar{position:relative;height:4px;border-radius:2px;background:var(--rs-track);overflow:hidden}
.bar i{position:absolute;left:0;top:0;bottom:0;border-radius:2px;background:var(--ic);transition:width .9s cubic-bezier(.2,.8,.2,1)}
.card.act{border-color:color-mix(in srgb,var(--ic) 60%,transparent);box-shadow:0 0 0 3px color-mix(in srgb,var(--ic) 14%,transparent)}
.card.act .bar::after{content:"";position:absolute;top:0;bottom:0;width:40%;background:linear-gradient(90deg,transparent,var(--ic),transparent);animation:scanbar 1.1s linear infinite}
@keyframes scanbar{from{left:-40%}to{left:100%}}
.card.done .sc em{color:var(--rs-text)}
.card.done .nm b::after{content:"✓";margin-left:5px;font-size:11px;color:#22c08a}
.scan{display:inline-flex;gap:3px;height:22px;align-items:center}
.scan i{width:5px;height:5px;border-radius:50%;background:var(--ic);animation:dots .9s ease-in-out infinite}
.scan i:nth-child(2){animation-delay:.15s}.scan i:nth-child(3){animation-delay:.3s}
@keyframes dots{50%{transform:translateY(-4px);opacity:.4}}
.ribbon{position:absolute;top:-20px;height:14px;z-index:5;border-radius:1px;pointer-events:none;opacity:0;animation:fall linear forwards}
@keyframes fall{0%{opacity:1;transform:translate3d(0,0,0) rotate(0) rotateX(0)}50%{transform:translate3d(30px,170px,0) rotate(200deg) rotateX(180deg)}85%{opacity:1}100%{opacity:0;transform:translate3d(-20px,400px,0) rotate(400deg) rotateX(360deg)}}
/* 动效:入场、扫光、粒子、实时卡片 */
@keyframes rise{from{opacity:0;transform:translateY(12px)}}
.eyebrow{animation:rise .7s cubic-bezier(.2,.8,.2,1) .05s backwards}
.hero{animation:rise .7s cubic-bezier(.2,.8,.2,1) .12s backwards}
.status{animation:rise .7s cubic-bezier(.2,.8,.2,1) .2s backwards}
.btn{animation:rise .7s cubic-bezier(.2,.8,.2,1) .28s backwards}
.live{animation:rise .7s cubic-bezier(.2,.8,.2,1) .36s backwards}
.card{animation:rise .7s cubic-bezier(.2,.8,.2,1) backwards}
.sweep{position:absolute;top:0;bottom:0;left:0;width:30%;pointer-events:none;opacity:0;transform:translateX(-100%);background:linear-gradient(90deg,transparent,color-mix(in srgb,var(--ac) 14%,transparent),transparent)}
.running .sweep{opacity:1;animation:sweep 2.8s ease-in-out infinite}
@keyframes sweep{to{transform:translateX(430%)}}
.sparks{position:absolute;inset:0;pointer-events:none}
.sparks i{position:absolute;background:var(--ac);opacity:0;animation:float linear infinite}
@keyframes float{0%{opacity:0;transform:translateY(0)}25%{opacity:.35}100%{opacity:0;transform:translateY(-80px)}}
.score{transform-origin:left bottom}.score.bump{animation:bump .45s ease-out}
@keyframes bump{40%{transform:scale(1.05)}}
.grade{animation:badge .6s cubic-bezier(.3,1.6,.5,1)}
@keyframes badge{from{transform:scale(.6);opacity:0}}
.btn>.rip{position:absolute;width:12px;height:12px;margin:-6px;border-radius:50%;background:rgba(255,255,255,.6);z-index:0;pointer-events:none;animation:rip .65s ease-out forwards}
.btn.busy .rip{display:none}
@keyframes rip{to{transform:scale(28);opacity:0}}
.body{animation:idle 1.4s steps(2) infinite}
@keyframes idle{50%{transform:translate(-50%,-2px)}}
.ring{position:absolute;left:-10px;bottom:2px;width:20px;height:20px;border-radius:50%;border:2px solid var(--ac);pointer-events:none;animation:ring .9s ease-out forwards}
@keyframes ring{from{opacity:.9;transform:scale(.6)}to{opacity:0;transform:scale(5)}}
.hit .dia{animation:ping .8s ease-out}
@keyframes ping{0%{box-shadow:0 0 0 0 rgba(34,192,138,.6)}100%{box-shadow:0 0 0 12px rgba(34,192,138,0)}}
.live{position:relative;flex:none;margin-top:auto;height:68px;box-sizing:border-box;padding:11px 14px 10px;border-radius:14px;display:flex;flex-direction:column;justify-content:space-between;background:var(--rs-card);border:1px solid var(--rs-card-b);backdrop-filter:blur(8px)}
.lhead{display:flex;justify-content:space-between;gap:10px;font-size:12px;color:var(--rs-sub);white-space:nowrap}
.ltag{color:var(--rs-text);font-weight:600;animation:rise .35s ease-out backwards}
.running .ltag::after{content:"▍";margin-left:3px;color:var(--ac);animation:blink 1s steps(2) infinite}
@keyframes blink{50%{opacity:0}}
.lmeta{font-variant-numeric:tabular-nums}
.eq{display:flex;align-items:flex-end;gap:3px;height:24px}
.eq i{flex:1;max-width:8px;height:100%;border-radius:2px;background:linear-gradient(var(--ac),color-mix(in srgb,var(--ac) 30%,transparent));transform-origin:bottom;transform:scaleY(.18);animation:breathe 2.8s ease-in-out infinite;animation-delay:calc(var(--d) * -2.8s)}
.running .eq i{animation:eq var(--t) ease-in-out infinite alternate;animation-delay:calc(var(--d) * -1s)}
@keyframes breathe{50%{transform:scaleY(.42)}}
@keyframes eq{from{transform:scaleY(.15)}to{transform:scaleY(var(--h))}}
@media (max-width:1000px){
  .panel{min-height:0}.left{flex:none;width:auto;padding:22px 20px 14px}
  .track{position:relative;left:auto;right:auto;bottom:auto;height:24px;margin:104px 8% 28px}
  .rail{position:relative;left:auto;right:auto;bottom:auto;display:flex;gap:6px;margin:46px 12px 16px;height:68px}
  .card{position:relative;left:auto!important;width:auto!important;flex:1;padding:8px 10px}
  .ic,.nm small{display:none}.banner{right:12px;top:72px}
}
@media (prefers-reduced-motion:reduce){.panel *,.panel *::before,.panel *::after{animation-duration:.01ms!important;animation-iteration-count:1!important}}
</style>
