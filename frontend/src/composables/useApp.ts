import { ref, onMounted, onUnmounted } from 'vue'

export function useApp() {
  const windowWidth = ref(window.innerWidth || 900)
  const windowHeight = ref(window.innerHeight || 650)

  let dispose: (() => void) | undefined

  onMounted(() => {
    // 1. 尝试从 GMSSH 宿主获取初始尺寸
    const rect = window.$gm?.getRectSize?.()
    if (rect?.width && rect.width > 0) windowWidth.value = rect.width
    if (rect?.height && rect.height > 0) windowHeight.value = rect.height

    // 2. 监听宿主动态 resize
    const unsub = window.$gm?.childRectListener?.((r) => {
      if (r.width > 0) windowWidth.value = r.width
      if (r.height > 0) windowHeight.value = r.height
    })
    dispose = typeof unsub === 'function' ? unsub : undefined

    // 3. 原生 window resize 回退
    const onResize = () => {
      if (!window.$gm?.getRectSize) {
        windowWidth.value = window.innerWidth
        windowHeight.value = window.innerHeight
      }
    }
    window.addEventListener('resize', onResize)

    const origDispose = dispose
    dispose = () => {
      origDispose?.()
      window.removeEventListener('resize', onResize)
    }
  })

  onUnmounted(() => {
    dispose?.()
  })

  return { windowWidth, windowHeight }
}
