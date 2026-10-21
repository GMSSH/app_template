<script setup lang="ts">
import { currentMessages } from '../locales'

const props = defineProps<{ locale: 'en-US' | 'zh-CN' }>()
const emit = defineEmits<{ change: [locale: 'en-US' | 'zh-CN'] }>()
</script>

<template>
  <button
    class="language-control"
    type="button"
    :aria-label="currentMessages.languageAria"
    @click="emit('change', props.locale === 'en-US' ? 'zh-CN' : 'en-US')"
  >
    <span class="language-option" :class="{ active: props.locale === 'en-US' }" aria-hidden="true">EN</span>
    <span class="language-option" :class="{ active: props.locale === 'zh-CN' }" aria-hidden="true">中</span>
  </button>
</template>
