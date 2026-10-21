export const DEFAULT_RECT = { width: 900, height: 650 }

export const appConfig = {
  // 后端 CLI 路径 = /.__gmssh/plugin/liteapp/{user_id}/{app_name}/app/bin/main
  // user_id / app_name 来自 web/liteapp.json（平台创建应用时填入），
  // 这里只留 fallback，不要在环境变量里配。
  userId: import.meta.env.VITE_GM_USER_ID || '',
  appName: import.meta.env.VITE_GM_APP || '',
}
