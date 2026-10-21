import type { GMProps } from 'gm-app-sdk/types'

declare global {
  interface Window {
    $gm?: GMProps & {
      theme?: 'light' | 'dark'
      gmsshHome?: string
      [key: string]: any
    }
  }
}

export {}
