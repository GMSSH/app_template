<script setup lang="ts">
import { computed } from 'vue'
import { currentMessages } from '../locales'

type Point = { label: string; score: number }
const props = defineProps<{ points: Point[] }>()
const width = 780
const height = 220
const padX = 46
const padY = 24
const max = computed(() => Math.max(10000, ...props.points.map((point) => point.score + 500)))
const min = 0
const x = (index: number) => padX + (index * (width - padX * 2)) / Math.max(props.points.length - 1, 1)
const y = (score: number) => height - padY - ((score - min) / (max.value - min)) * (height - padY * 2)
const coords = computed(() => props.points.map((point, index) => ({ ...point, x: x(index), y: y(point.score) })))
const line = computed(() => coords.value.map((point) => point.x + ',' + point.y).join(' '))
const area = computed(() => {
  const values = coords.value
  if (!values.length) return ''
  return values[0].x + ',' + (height - padY) + ' ' + line.value + ' ' + values[values.length - 1].x + ',' + (height - padY)
})
const ticks = [0, 3000, 6000, 9000, 12000]
</script>

<template>
  <div class="trend-chart">
    <svg :viewBox="'0 0 ' + width + ' ' + height" preserveAspectRatio="none" role="img" :aria-label="currentMessages.chartTrendAria">
      <defs>
        <linearGradient id="trend-fill" x1="0" x2="0" y1="0" y2="1">
          <stop offset="0" stop-color="#2f8cff" stop-opacity=".28" />
          <stop offset="1" stop-color="#2f8cff" stop-opacity="0" />
        </linearGradient>
        
      </defs>
      <g class="chart-grid">
        <g v-for="tick in ticks" :key="tick"><line :x1="padX" :x2="width - padX" :y1="y(tick)" :y2="y(tick)" /><text :x="padX - 8" :y="y(tick) + 4" text-anchor="end">{{ tick === 0 ? '0' : tick / 1000 + 'k' }}</text></g>
      </g>
      <polygon :points="area" fill="url(#trend-fill)" />
      <polyline :points="line" fill="none" stroke="#3189ff" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" />
      <g v-for="point in coords" :key="point.label">
        <circle :cx="point.x" :cy="point.y" r="5" fill="var(--app-card)" stroke="#3189ff" stroke-width="3" />
        <text :x="point.x" :y="height - 3" text-anchor="middle">{{ point.label }}</text>
      </g>
    </svg>
  </div>
</template>
