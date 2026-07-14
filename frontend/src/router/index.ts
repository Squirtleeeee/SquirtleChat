import { createRouter, createWebHistory, createWebHashHistory } from 'vue-router'
import LoginView from '../views/LoginView.vue'
import ChatView from '../views/ChatView.vue'
import ProfileView from '../views/ProfileView.vue'
import EditProfileView from '../views/EditProfileView.vue'
import GroupDetailView from '../views/GroupDetailView.vue'
import SettingsView from '../views/SettingsView.vue'
import ChatPopupView from '../views/ChatPopupView.vue'

const useHash =
  typeof window !== 'undefined' &&
  (!!window.squirtleDesktop?.isElectron || location.protocol === 'file:')

const router = createRouter({
  history: useHash ? createWebHashHistory() : createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginView },
    { path: '/', name: 'chat', component: ChatView, meta: { auth: true } },
    { path: '/popup-chat', name: 'popup-chat', component: ChatPopupView, meta: { auth: true } },
    { path: '/settings', name: 'settings', component: SettingsView, meta: { auth: true } },
    { path: '/profile', name: 'profile', component: ProfileView, meta: { auth: true } },
    { path: '/profile/edit', name: 'profile-edit', component: EditProfileView, meta: { auth: true } },
    { path: '/group/:id', name: 'group', component: GroupDetailView, meta: { auth: true } },
    { path: '/profile/:id', name: 'profile-user', component: ProfileView, meta: { auth: true } },
  ],
})

router.beforeEach((to) => {
  const token = localStorage.getItem('access_token')
  if (to.meta.auth && !token) return '/login'
  if (to.path === '/login' && token) return '/'
})

export default router
