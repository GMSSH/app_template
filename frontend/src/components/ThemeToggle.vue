<script setup lang="ts">
import { Moon, Sun } from '@lucide/vue'

const props = defineProps<{ dark: boolean; lightLabel: string; darkLabel: string }>()
const emit = defineEmits<{ toggle: [] }>()
</script>

<template>
  <button class="theme-control" type="button" :aria-label="props.dark ? props.lightLabel : props.darkLabel" :title="props.dark ? props.lightLabel : props.darkLabel" @click="emit('toggle')">
    <span class="theme-icon" :class="{ active: !props.dark }"><Sun :size="15" :stroke-width="2.2" /></span>
    <span class="theme-icon" :class="{ active: props.dark }"><Moon :size="15" :stroke-width="2.2" /></span>
  </button>
</template>
