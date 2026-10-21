<script setup lang="ts">
import { ref, watch, computed } from 'vue'

const props = withDefaults(
  defineProps<{
    label: string
    value: number
    variant?: 'total' | 'tcp' | 'udp' | 'external'
    subText?: string
    totalCount?: number
  }>(),
  {
    variant: 'total',
    subText: '',
    totalCount: 1,
  }
)

const prog = ref(0)
const shown = ref(0)
let raf = 0

watch(() => props.value, (to) => {
  cancelAnimationFrame(raf)
  const from = shown.value, t0 = performance.now()
  const step = (now: number) => {
    const k = Math.min(1, (now - t0) / 750)
    const e = 1 - Math.pow(1 - k, 3)
    prog.value = e
    shown.value = Math.round(from + (to - from) * e)
    if (k < 1) raf = requestAnimationFrame(step)
  }
  raf = requestAnimationFrame(step)
}, { immediate: true })

const color = computed(() => {
  switch (props.variant) {
    case 'total': return 'var(--tx)'
    case 'tcp': return '#3d8bff'
    case 'udp': return '#f0a81c'
    case 'external': return '#ff5230'
  }
})

const pct = computed(() => {
  if (props.variant === 'total') return 100
  const tot = props.totalCount || 1
  return Math.min(100, Math.max(0, (props.value / tot) * 100))
})

const ringLabel = computed(() => {
  if (props.variant === 'total') return '全部'
  return Math.round(pct.value) + '%'
})

// stroke-dasharray is 182.2 (2 * pi * 29)
const dashOffset = computed(() => {
  return 182.2 * (1 - (pct.value / 100) * prog.value)
})
</script>

<template>
  <div class="stat" :style="{ '--c': color }">
    <div class="stat-main">
      <div class="l"><i></i>{{ label }}</div>
      <div class="n mono">{{ shown }}</div>
      <div class="s">{{ subText }}</div>
    </div>
    <div class="ring">
      <svg viewBox="0 0 80 80">
        <circle class="tk" cx="40" cy="40" r="37" />
        <circle class="tr" cx="40" cy="40" r="29" />
        <circle
          class="fg"
          cx="40"
          cy="40"
          r="29"
          stroke-dasharray="182.2"
          :style="{ strokeDashoffset: dashOffset }"
        />
      </svg>
      <b>{{ ringLabel }}</b>
    </div>
  </div>
</template>

<style scoped>
.stat {
  --c: var(--tx);
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 14px 9px 15px;
  border-radius: 8px;
  border: 1px solid color-mix(in srgb, var(--c) 30%, var(--line));
  background: linear-gradient(125deg, color-mix(in srgb, var(--c) 13%, var(--panel)), var(--panel) 72%);
  box-shadow: var(--sh);
  overflow: hidden;
  flex: 1;
  min-width: 0;
  transition: transform 0.25s cubic-bezier(0.2, 0.8, 0.2, 1), box-shadow 0.25s;
}

.stat:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 16px rgba(15, 20, 25, 0.08);
}

.stat:before,
.stat:after {
  content: "";
  position: absolute;
  width: 14px;
  height: 14px;
  border: 2px solid var(--c);
  pointer-events: none;
}
.stat:before {
  left: -1px;
  top: -1px;
  border-right: 0;
  border-bottom: 0;
  border-top-left-radius: 8px;
}
.stat:after {
  right: -1px;
  bottom: -1px;
  border-left: 0;
  border-top: 0;
  border-bottom-right-radius: 8px;
}

.stat-main {
  flex: 1;
  min-width: 0;
}

.stat .l {
  font-size: 11px;
  color: var(--mu);
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
}
.stat .l i {
  width: 6px;
  height: 6px;
  background: var(--c);
  transform: rotate(45deg);
  box-shadow: 0 0 8px var(--c);
  flex: none;
}

.stat .n {
  font-size: 26px;
  font-weight: 600;
  letter-spacing: -0.03em;
  line-height: 1;
  margin: 4px 0 3px;
  font-variant-numeric: tabular-nums;
  color: var(--c);
}

.stat .s {
  font-size: 12px;
  color: var(--mu);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 刻度雷达环形进度条 */
.ring {
  width: 48px;
  height: 48px;
  flex: none;
  position: relative;
}
.ring svg {
  width: 100%;
  height: 100%;
  transform: rotate(-90deg);
}
.ring .tk {
  fill: none;
  stroke: var(--c);
  stroke-opacity: 0.3;
  stroke-width: 5;
  stroke-dasharray: 1.3 3.4;
}
.ring .tr {
  fill: none;
  stroke: var(--c);
  stroke-opacity: 0.15;
  stroke-width: 3;
}
.ring .fg {
  fill: none;
  stroke: var(--c);
  stroke-width: 3;
  stroke-linecap: round;
  transition: stroke-dashoffset 0.8s cubic-bezier(0.2, 0.8, 0.2, 1);
}
.ring b {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  font-size: 9.5px;
  font-weight: 600;
  color: var(--c);
  font-family: "IBM Plex Mono", ui-monospace, Menlo, monospace;
}
</style>
