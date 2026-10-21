import { computed, ref } from 'vue'
import { $gm } from './gm'

export type AppTheme = 'light' | 'dark'
export const appTheme = ref<AppTheme>('light')

function hostTheme(): AppTheme {
  return $gm?.theme === 'dark' ? 'dark' : 'light'
}

export function initializeTheme() {
  appTheme.value = hostTheme()
  applyTheme()
}

export function toggleTheme() {
  appTheme.value = appTheme.value === 'light' ? 'dark' : 'light'
  applyTheme()
}

export function applyTheme() {
  document.documentElement.dataset.theme = appTheme.value
}

export const isDark = computed(() => appTheme.value === 'dark')
