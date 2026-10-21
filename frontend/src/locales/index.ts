import { createI18n } from 'vue-i18n'
import zh from './zh-CN'
import en from './en-US'

// 初始语言读取只读的 $gm.lang；无宿主时回退英文
const host = window.$gm?.lang?.toLowerCase() || ''
export const initialLocale = host.startsWith('zh') ? 'zh-CN' : 'en-US'

export const i18n = createI18n({
  legacy: false, locale: initialLocale, fallbackLocale: 'en-US',
  messages: { 'zh-CN': zh, 'en-US': en },
})
