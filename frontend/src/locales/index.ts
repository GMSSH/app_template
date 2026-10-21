import { computed, ref } from 'vue'
import { messages, type Locale } from './messages'

// 宿主未给出语言（或给出非中文）时回退英文；无 lang 时默认中文。
function normalize(lang?: string): Locale {
  return lang && !lang.toLowerCase().startsWith('zh') ? 'en-US' : 'zh-CN'
}

export const locale = ref<Locale>(normalize(window.$gm?.lang))
export const currentMessages = computed(() => messages[locale.value])

export function toggleLocale() {
  locale.value = locale.value === 'en-US' ? 'zh-CN' : 'en-US'
}

export function t(key: keyof typeof messages['en-US']) {
  return currentMessages.value[key]
}
