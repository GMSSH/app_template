import { ref, onMounted, onUnmounted } from 'vue'

export function useApp() {
  const initial = window.$gm?.getRectSize ? window.$gm.getRectSize() : null
  const windowWidth = ref(initial?.width || window.innerWidth)
  const windowHeight = ref(initial?.height || window.innerHeight)

  const updateFromNative = () => {
    windowWidth.value = window.innerWidth
    windowHeight.value = window.innerHeight
  }

  onMounted(() => {
    window.addEventListener('resize', updateFromNative)

    // GMSSH 官方 SDK 提供的尺寸监听
    if (window.$gm?.childRectListener) {
      window.$gm.childRectListener(({ width, height }: { width: number; height: number }) => {
        if (width > 0) windowWidth.value = width
        if (height > 0) windowHeight.value = height
      })
    }
  })

  onUnmounted(() => {
    window.removeEventListener('resize', updateFromNative)
  })

  return {
    windowWidth,
    windowHeight,
  }
}
