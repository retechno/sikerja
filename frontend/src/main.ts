import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { createAppRouter } from './router'
import { bindSessionExpiry } from './stores/auth'
import { useToastStore } from './stores/toast'
import './components/charts/chart'
import './style.css'

const app = createApp(App)
app.use(createPinia())
const router = createAppRouter()
app.use(router)

// Sesi habis (refresh gagal) → kembali ke halaman login, simpan tujuan semula.
bindSessionExpiry(() => {
  if (router.currentRoute.value.name !== 'login') {
    useToastStore().info('Sesi berakhir. Silakan masuk kembali.')
    void router.replace({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
  }
})

app.mount('#app')
