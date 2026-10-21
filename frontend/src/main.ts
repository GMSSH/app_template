import 'gm-app-sdk'
import { createApp } from 'vue'
import App from './App.vue'
import { bootstrap } from './app/bootstrap'
import { initializeGm } from './app/gm'
import './styles.css'

async function main() {
  initializeGm()
  await bootstrap()
  createApp(App).mount('#app')
}

void main()
