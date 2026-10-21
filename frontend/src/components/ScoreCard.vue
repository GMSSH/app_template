<script setup lang="ts">
import type { Component } from 'vue'
import ScoreRing from './ScoreRing.vue'

defineProps<{
  label: string
  value: string
  unit: string
  percent: number
  color: string
  icon: Component
  active?: boolean
}>()
</script>

<template>
  <article class="metric-card" :class="{ 'is-active': active }">
    <div class="metric-icon" :style="{ color }"><component :is="icon" :size="29" :stroke-width="1.8" /></div>
    <div class="metric-copy">
      <span class="metric-label">{{ label }}</span>
      <div class="metric-value">{{ value }} <small>{{ unit }}</small></div>
      <div class="metric-bar"><span :style="{ width: percent + '%', background: color }" /></div>
    </div>
    <ScoreRing :value="percent" :color="color" />
  </article>
</template>
