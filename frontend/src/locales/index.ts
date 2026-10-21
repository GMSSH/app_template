import { createI18n } from 'vue-i18n'
import zh from './zh-CN'
import en from './en-US'

// 初始语言读取只读的 $gm.lang；无宿主（浏览器预览）时默认中文
const host = window.$gm?.lang?.toLowerCase() || ''
export const initialLocale = host && !host.startsWith('zh') ? 'en-US' : 'zh-CN'

export const i18n = createI18n({
  legacy: false, locale: initialLocale, fallbackLocale: 'en-US',
  messages: { 'zh-CN': zh, 'en-US': en },
})
