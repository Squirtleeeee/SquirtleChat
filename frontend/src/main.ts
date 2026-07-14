import './styles/theme.css'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { useAuthStore } from './stores/auth'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia).use(router)

const auth = useAuthStore(pinia)
auth.restoreSession().finally(() => {
  app.mount('#app')
})
