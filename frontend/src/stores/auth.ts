import { defineStore } from 'pinia'
import http, { unwrapApiData } from '../api/http'
import { WSClient } from '../api/ws'
import { idStr } from '../utils/id'

export type UserPrivacy = {
  show_nickname: boolean
  show_gender: boolean
  show_birthday: boolean
  show_avatar: boolean
}

export type User = {
  id: string
  username: string
  nickname: string
  avatar?: string
  status_text?: string
  status_emoji?: string
  gender?: number
  birthday?: string
  privacy?: UserPrivacy
}

export type PublicProfile = {
  id: string
  username: string
  nickname?: string
  avatar?: string
  status_text?: string
  status_emoji?: string
  gender?: number
  birthday?: string
  remark?: string
}

type LoginResult = { user: User; tokens: { access_token: string; refresh_token: string } }

export type DeviceSession = {
  device_id: string
  device_name: string
  last_active_at: string
  current: boolean
}

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null as User | null,
    accessToken: localStorage.getItem('access_token') || '',
    refreshToken: localStorage.getItem('refresh_token') || '',
    deviceId: localStorage.getItem('device_id') || crypto.randomUUID(),
    ws: null as WSClient | null,
  }),
  getters: {
    isLogin: (s) => !!s.accessToken,
  },
  actions: {
    normalizeUser(u: User): User {
      return {
        ...u,
        id: idStr(u.id),
        privacy: u.privacy || {
          show_nickname: true,
          show_gender: false,
          show_birthday: false,
          show_avatar: true,
        },
      }
    },
    toPublicProfile(u: User): PublicProfile {
      return {
        id: u.id,
        username: u.username,
        nickname: u.nickname,
        avatar: u.avatar,
        status_text: u.status_text,
        status_emoji: u.status_emoji,
        gender: u.gender,
        birthday: u.birthday,
      }
    },
    async register(username: string, password: string, nickname: string) {
      const { data } = await http.post('/auth/register', { username, password, nickname })
      this.applyLogin(unwrapApiData<LoginResult>(data))
    },
    async login(username: string, password: string) {
      const deviceName = window.squirtleDesktop?.isElectron ? 'SquirtleChat Desktop' : 'SquirtleChat Web'
      const { data } = await http.post('/auth/login', {
        username,
        password,
        device_id: this.deviceId,
        device_name: deviceName,
      })
      this.applyLogin(unwrapApiData<LoginResult>(data))
    },
    async listDevices() {
      const { data } = await http.get('/users/me/devices')
      return unwrapApiData<{ devices: DeviceSession[] }>(data).devices || []
    },
    async revokeDevice(deviceId: string) {
      await http.delete(`/users/me/devices/${encodeURIComponent(deviceId)}`)
    },
    applyLogin(res: LoginResult) {
      this.user = this.normalizeUser(res.user)
      this.accessToken = res.tokens.access_token
      this.refreshToken = res.tokens.refresh_token || this.refreshToken
      localStorage.setItem('access_token', this.accessToken)
      if (this.refreshToken) localStorage.setItem('refresh_token', this.refreshToken)
      localStorage.setItem('device_id', this.deviceId)
      this.connectWS()
    },
    async updateProfile(fields: {
      nickname?: string
      avatar?: string
      status_text?: string
      status_emoji?: string
      gender?: number
      birthday?: string
    }) {
      const { data } = await http.put('/users/me', fields)
      const res = unwrapApiData<{ user: User }>(data)
      this.user = this.normalizeUser(res.user)
      return this.user
    },
    async updatePrivacy(privacy: UserPrivacy) {
      const { data } = await http.put('/users/me/privacy', privacy)
      const res = unwrapApiData<{ user: User }>(data)
      this.user = this.normalizeUser(res.user)
      return this.user
    },
    async changePassword(oldPassword: string, newPassword: string) {
      await http.put('/users/me/password', {
        old_password: oldPassword,
        new_password: newPassword,
      })
    },
    async uploadAvatar(blob: Blob) {
      const form = new FormData()
      form.append('file', blob, 'avatar.jpg')
      const { data } = await http.post('/users/me/avatar', form)
      const res = unwrapApiData<{ user: User; url: string }>(data)
      this.user = this.normalizeUser(res.user)
      return res.url
    },
    async fetchPublicProfile(userId: string) {
      const { data } = await http.get(`/users/${idStr(userId)}`)
      const res = unwrapApiData<{ user: PublicProfile }>(data)
      return { ...res.user, id: idStr(res.user.id) }
    },
    async restoreSession() {
      if (!this.accessToken) return false
      try {
        const { data } = await http.get('/users/me')
        const res = unwrapApiData<{ user: User }>(data)
        this.user = this.normalizeUser(res.user)
        if (!this.ws) this.connectWS()
        return true
      } catch {
        if (this.refreshToken) {
          try {
            const { data } = await http.post('/auth/refresh', { refresh_token: this.refreshToken })
            this.applyLogin(unwrapApiData<LoginResult>(data))
            return true
          } catch {
            this.logout()
          }
        }
        return false
      }
    },
    connectWS() {
      this.ws?.close()
      const wsBase = import.meta.env.VITE_WS_BASE || 'ws://localhost:8081'
      const client = new WSClient()
      client.connect(wsBase, this.accessToken, this.deviceId)
      client.onTokenRefresh(() => this.accessToken)
      this.ws = client
    },
    reconnectWS() {
      if (!this.accessToken) return
      if (this.ws) {
        this.ws.forceReconnect()
        return
      }
      this.connectWS()
    },
    async logout() {
      try {
        if (this.accessToken) await http.post('/auth/logout')
      } catch {
        /* ignore */
      }
      this.ws?.close()
      this.ws = null
      this.user = null
      this.accessToken = ''
      this.refreshToken = ''
      localStorage.removeItem('access_token')
      localStorage.removeItem('refresh_token')
    },
  },
})
