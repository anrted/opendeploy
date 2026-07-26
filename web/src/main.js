import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import i18n from './i18n'
import './style.css'
import { useThemeStore } from './stores/theme'

const app = createApp(App)
app.use(createPinia())
useThemeStore() // Initialize theme before mounting
app.use(router)
app.use(i18n)
app.mount('#app')
