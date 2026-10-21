import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { viteSingleFile } from 'vite-plugin-singlefile'

// SINGLE=1 仅用于生成单文件预览页，正式构建走默认配置
export default defineConfig({
  base: './',
  plugins: [vue(), ...(process.env.SINGLE ? [viteSingleFile()] : [])],
  server: {
    host: '0.0.0.0',
    port: 5185,
    strictPort: true,
    allowedHosts: ['dev.adde.cn', 'localhost', '127.0.0.1'],
  },
})
