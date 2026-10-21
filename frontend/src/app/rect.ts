import { reactive } from 'vue'
import { DEFAULT_RECT } from './config'
import { $gm } from './gm'

export const appRect = reactive({ ...DEFAULT_RECT })
let disposeListener: (() => void) | undefined

export function initializeRect() {
  const current = $gm?.getRectSize?.()
  if (current?.width > 0) appRect.width = current.width
  if (current?.height > 0) appRect.height = current.height

  const dispose = $gm?.childRectListener?.((next) => {
    if (next.width > 0) appRect.width = next.width
    if (next.height > 0) appRect.height = next.height
  })
  disposeListener = typeof dispose === 'function' ? dispose : undefined
}

export function disposeRect() {
  disposeListener?.()
  disposeListener = undefined
}
