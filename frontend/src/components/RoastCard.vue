<script setup lang="ts">
import { computed, ref } from 'vue'
import { currentMessages } from '../locales'

const props = defineProps<{ score: number; parts: number[]; time: string }>()
const emit = defineEmits<{ close: [] }>()
const bands = [9800, 9500, 9200, 8800, 8500, 8200, 7800, 7500, 7200, 7000, 6800, 6500, 6000, 5500, 5000, 4000, 3000, 0] as const
const weights = [4000, 3000, 3000]
const text = computed(() => {
  const m = currentMessages.value as Record<string, string>
  return m['roast' + (bands.find((n) => props.score >= n) ?? 0)]
})
const tone = computed(() => props.score >= 9000 ? 'godlike' : props.score >= 8000 ? 'legendary' : props.score >= 7000 ? 'epic' : props.score >= 6000 ? 'rare' : 'common')
const rank = computed(() => {
  const m = currentMessages.value as Record<string, string>
  const keys: Record<string, string> = { godlike: 'cardRankGodlike', legendary: 'cardRankLegendary', epic: 'cardRankEpic', rare: 'cardRankRare', common: 'cardRankCommon' }
  return m[keys[tone.value]]
})
const tilt = ref('rotateX(0deg) rotateY(0deg)')

function move(e: PointerEvent) {
  if (e.pointerType === 'touch') return
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const x = (e.clientX - r.left) / r.width - 0.5
  const y = (e.clientY - r.top) / r.height - 0.5
  tilt.value = 'rotateX(' + (-y * 17.5) + 'deg) rotateY(' + (x * 17.5) + 'deg) scale(1.04)'

}
function reset() { tilt.value = 'rotateX(0deg) rotateY(0deg)' }
const rows = computed(() => [
  ['CPU', props.parts[0] ?? 0, weights[0]],
  ['MEM', props.parts[1] ?? 0, weights[1]],
  ['DISK', props.parts[2] ?? 0, weights[2]],
])
</script>

<template>
  <div class="share-overlay" @click="emit('close')">
    <div class="share-modal" @click.stop>
      <article class="card-preview" :class="tone" :style="{ transform: tilt }" @pointermove="move" @pointerleave="reset">
        <i class="corner t-l"></i><i class="corner t-r"></i><i class="corner b-l"></i><i class="corner b-r"></i>
        <header class="card-header"><span class="brand">GMSSH</span><span class="tag">{{ currentMessages.cardBenchmark }}</span></header>
        <div class="card-body">
          <div class="rank-text" :class="tone">{{ rank }}</div>
          <div class="score-box"><div class="score-num">{{ score }}</div><div class="score-label">{{ currentMessages.cardTotalPoints }}</div></div>
          <div class="roast-box"><span class="quote-mark">“</span>{{ text }}<span class="quote-mark">”</span></div>
          <ul class="part-row"><li v-for="row in rows" :key="String(row[0])">{{ row[0] }} {{ row[1] }}/{{ row[2] }}</li></ul>
        </div>
        <footer class="card-footer"><div class="sys-info"><span>{{ currentMessages.cardDate }}: {{ time }}</span></div><div class="footer-brand">{{ currentMessages.cardPoweredBy }}</div></footer>
      </article>
      <button class="cyber-btn" type="button" @click="emit('close')"><span>{{ currentMessages.roastClose }}</span></button>
    </div>
  </div>
</template>
