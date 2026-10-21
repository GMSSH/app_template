import { initializeTheme } from './theme'

// bootstrap 只做不依赖宿主后端的初始化（主题）。
// $gm.init() 在 App.vue 里完成，因为要先显示全局 loading，等 init 结束才渲染页面。
export async function bootstrap() {
  initializeTheme()
}
