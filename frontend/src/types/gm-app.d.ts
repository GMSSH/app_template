export {}

declare global {
  interface GmRect {
    width: number
    height: number
  }

  interface GmRequestOptions {
    method?: string
    headers?: Record<string, string>
    data?: unknown
  }

  interface GmApi {
    lang?: string
    theme?: 'light' | 'dark'
    version?: string
    communicationType?: string
    token?: string
    authToken?: string
    accessToken?: string
    init?: () => Promise<unknown>
    getRectSize: () => GmRect
    childRectListener: (listener: (rect: GmRect) => void) => void | (() => void)
    setAppRectStyle: (rect: { width?: number; height?: number }) => void
    request: (url: string, options?: GmRequestOptions) => Promise<unknown>
  }

  interface Window {
    $gm?: GmApi
  }
}
