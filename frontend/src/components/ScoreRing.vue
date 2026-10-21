<script setup lang="ts">
import { computed } from 'vue'

defineOptions({ name: 'ScoreRing' })
const props = withDefaults(defineProps<{ value: number; color: string; size?: number }>(), { size: 76 })
const radius = computed(() => (props.size - 10) / 2)
const circumference = computed(() => 2 * Math.PI * radius.value)
const dash = computed(() => (Math.max(0, Math.min(100, props.value)) / 100) * circumference.value)
</script>

<template>
  <div class="score-ring" :style="{ width: size + 'px', height: size + 'px' }">
    <svg :width="size" :height="size" viewBox="0 0 76 76" role="img" :aria-label="value + '%'">
      <circle class="ring-track" cx="38" cy="38" :r="radius" />
      <circle class="ring-value" cx="38" cy="38" :r="radius" :style="{ stroke: color, strokeDasharray: dash + ' ' + circumference }" />
    </svg>
    <strong>{{ Math.round(value) }}%</strong>
  </div>
</template>
