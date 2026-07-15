<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSettingsStore, type LayoutMode } from '../stores/settings'
import { useAuthStore, type DeviceSession } from '../stores/auth'
import { isDesktopApp } from '../utils/desktop'
import { useChatStore } from '../stores/chat'
import { parseError } from '../api/errors'
import { ensureNotifyPermission } from '../utils/notify'

const router = useRouter()
const settings = useSettingsStore()
const auth = useAuthStore()
const chat = useChatStore()
const desktop = isDesktopApp()

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const pwdLoading = ref(false)
const pwdErr = ref('')
const pwdOk = ref('')

const devices = ref<DeviceSession[]>([])
const devicesLoading = ref(false)
const devicesErr = ref('')
const revokingId = ref('')

function goBack() {
  router.push('/')
}

function onLayoutChange(e: Event) {
  const v = (e.target as HTMLSelectElement).value as LayoutMode
  settings.setLayoutMode(v)
  chat.setTransientNotice(
    v === 'detached' ? '已切换为会话独立窗口模式' : '已切换为经典单窗口模式',
  )
}

function onAlwaysOnTop(e: Event) {
  const on = (e.target as HTMLInputElement).checked
  settings.setChatAlwaysOnTop(on)
}

async function loadDevices() {
  devicesLoading.value = true
  devicesErr.value = ''
  try {
    devices.value = await auth.listDevices()
  } catch (e) {
    devicesErr.value = parseError(e)
  } finally {
    devicesLoading.value = false
  }
}

async function revoke(d: DeviceSession) {
  if (d.current) {
    if (!confirm('下线当前设备将退出登录，确定？')) return
  } else if (!confirm(`确定下线「${d.device_name}」？`)) {
    return
  }
  revokingId.value = d.device_id
  try {
    await auth.revokeDevice(d.device_id)
    if (d.current) {
      await auth.logout()
      router.replace('/login')
      return
    }
    chat.setTransientNotice('已下线该设备')
    await loadDevices()
  } catch (e) {
    devicesErr.value = parseError(e)
  } finally {
    revokingId.value = ''
  }
}

function formatActive(iso: string) {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

async function submitPassword() {
  pwdErr.value = ''
  pwdOk.value = ''
  if (!oldPassword.value || !newPassword.value) {
    pwdErr.value = '请填写原密码和新密码'
    return
  }
  if (newPassword.value.length < 6) {
    pwdErr.value = '新密码至少 6 位'
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    pwdErr.value = '两次输入的新密码不一致'
    return
  }
  pwdLoading.value = true
  try {
    await auth.changePassword(oldPassword.value, newPassword.value)
    oldPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    pwdOk.value = '密码已更新'
    chat.setTransientNotice('密码已修改')
  } catch (e) {
    pwdErr.value = parseError(e)
  } finally {
    pwdLoading.value = false
  }
}

const notifyPerm = ref(
  typeof Notification !== 'undefined' ? Notification.permission : ('denied' as NotificationPermission),
)

function onNotifyToggle(e: Event) {
  settings.setNotifyEnabled((e.target as HTMLInputElement).checked)
  void chat.persistChatPrefs()
}

function onQuietToggle(e: Event) {
  settings.setQuietHours((e.target as HTMLInputElement).checked)
  void chat.persistChatPrefs()
}

function onQuietStart(e: Event) {
  settings.setQuietHours(settings.notify.quietHoursEnabled, (e.target as HTMLInputElement).value, undefined)
  void chat.persistChatPrefs()
}

function onQuietEnd(e: Event) {
  settings.setQuietHours(settings.notify.quietHoursEnabled, undefined, (e.target as HTMLInputElement).value)
  void chat.persistChatPrefs()
}

async function requestNotify() {
  const ok = await ensureNotifyPermission()
  notifyPerm.value = typeof Notification !== 'undefined' ? Notification.permission : 'denied'
  if (ok) {
    settings.setNotifyEnabled(true)
    void chat.persistChatPrefs()
    chat.setTransientNotice('已开启桌面通知权限')
  } else {
    chat.setTransientNotice('浏览器未授予通知权限')
  }
}

onMounted(() => {
  void loadDevices()
  if (typeof Notification !== 'undefined') notifyPerm.value = Notification.permission
})
</script>

<template>
  <div class="settings-page">
    <header class="page-header">
      <button type="button" class="btn btn-ghost btn-sm" @click="goBack">← 返回</button>
      <h1>设置</h1>
      <span />
    </header>

    <div class="body">
      <section class="block">
        <h2>账号安全</h2>
        <p class="hint">修改登录密码。修改成功后请妥善保管新密码。</p>
        <form class="pwd-form" @submit.prevent="submitPassword">
          <label class="field">
            <span class="label">原密码</span>
            <input
              v-model="oldPassword"
              class="input"
              type="password"
              autocomplete="current-password"
              placeholder="请输入原密码"
            />
          </label>
          <label class="field">
            <span class="label">新密码</span>
            <input
              v-model="newPassword"
              class="input"
              type="password"
              autocomplete="new-password"
              placeholder="至少 6 位"
            />
          </label>
          <label class="field">
            <span class="label">确认新密码</span>
            <input
              v-model="confirmPassword"
              class="input"
              type="password"
              autocomplete="new-password"
              placeholder="再次输入新密码"
            />
          </label>
          <p v-if="pwdErr" class="form-err" role="alert">{{ pwdErr }}</p>
          <p v-else-if="pwdOk" class="form-ok" role="status">{{ pwdOk }}</p>
          <button type="submit" class="btn btn-primary" :disabled="pwdLoading">
            {{ pwdLoading ? '提交中…' : '修改密码' }}
          </button>
        </form>
      </section>

      <section class="block">
        <h2>登录设备</h2>
        <p class="hint">可查看近期登录设备，并远程下线其它端（类似微信/企业微信）。</p>
        <p v-if="devicesErr" class="form-err">{{ devicesErr }}</p>
        <p v-else-if="devicesLoading" class="meta">加载中…</p>
        <ul v-else class="device-list">
          <li v-for="d in devices" :key="d.device_id" class="device-item">
            <div class="device-meta">
              <strong>{{ d.device_name || '未知设备' }}</strong>
              <span v-if="d.current" class="badge">本机</span>
              <span class="device-time">最近活跃 {{ formatActive(d.last_active_at) }}</span>
            </div>
            <button
              type="button"
              class="btn btn-ghost btn-sm danger"
              :disabled="revokingId === d.device_id"
              @click="revoke(d)"
            >
              {{ revokingId === d.device_id ? '处理中…' : d.current ? '退出本机' : '下线' }}
            </button>
          </li>
          <li v-if="!devices.length" class="meta">暂无设备记录，重新登录后会出现。</li>
        </ul>
      </section>

      <section class="block">
        <h2>消息通知</h2>
        <p class="hint">控制桌面/浏览器通知；免打扰时段与开关会同步到云端，多端一致。会话免打扰仍优先于全局开关。</p>
        <label class="check-row">
          <input type="checkbox" :checked="settings.notify.desktopEnabled" @change="onNotifyToggle" />
          <span>启用桌面通知</span>
        </label>
        <div class="notify-perm">
          <span class="meta">系统权限：{{ notifyPerm }}</span>
          <button type="button" class="btn btn-ghost btn-sm" @click="requestNotify">申请权限</button>
        </div>
        <label class="check-row">
          <input type="checkbox" :checked="settings.notify.quietHoursEnabled" @change="onQuietToggle" />
          <span>免打扰时段</span>
        </label>
        <div v-if="settings.notify.quietHoursEnabled" class="quiet-row">
          <label class="field inline">
            <span class="label">开始</span>
            <input
              class="input"
              type="time"
              :value="settings.notify.quietStart"
              @change="onQuietStart"
            />
          </label>
          <label class="field inline">
            <span class="label">结束</span>
            <input class="input" type="time" :value="settings.notify.quietEnd" @change="onQuietEnd" />
          </label>
        </div>
      </section>

      <section class="block">
        <h2>排版模式</h2>
        <p class="hint">参考微信电脑版：可在单窗口内切换会话，或每个会话单独弹出窗口。</p>
        <label class="field">
          <span class="label">会话布局</span>
          <select class="input" :value="settings.layoutMode" @change="onLayoutChange">
            <option value="embedded">经典单窗口（侧栏 + 聊天区）</option>
            <option value="detached">会话独立窗口（点击会话弹出小窗）</option>
          </select>
        </label>
        <ul class="tips">
          <li>经典模式：与现在一致，所有聊天在同一窗口右侧进行。</li>
          <li>独立窗口：点击好友/群聊时打开独立聊天窗，适合多会话并行。</li>
          <li>任意模式下均可在会话列表右键选择「独立窗口打开」。</li>
        </ul>
      </section>

      <section v-if="desktop" class="block">
        <h2>桌面端</h2>
        <label class="check-row">
          <input type="checkbox" :checked="settings.chatAlwaysOnTop" @change="onAlwaysOnTop" />
          <span>聊天独立窗口默认置顶</span>
        </label>
        <p class="hint">关闭主窗口时会最小化到系统托盘，可在托盘图标右键退出。</p>
      </section>

      <section class="block">
        <h2>关于</h2>
        <p class="meta">SquirtleChat {{ desktop ? '桌面版' : '网页版' }}</p>
      </section>
    </div>
  </div>
</template>

<style scoped>
.settings-page {
  height: 100%;
  min-height: 100%;
  overflow-y: auto;
  background: var(--color-bg-app);
}

.page-header {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  min-height: var(--header-height);
  padding: 0 var(--space-4);
  background: var(--color-bg-surface);
  border-bottom: 1px solid var(--color-border);
}

.page-header h1 {
  margin: 0;
  font-size: var(--text-base);
  text-align: center;
}

.body {
  max-width: 560px;
  margin: 0 auto;
  padding: var(--space-5) var(--space-4);
  display: flex;
  flex-direction: column;
  gap: var(--space-5);
}

.block h2 {
  margin: 0 0 var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.hint {
  margin: 0 0 var(--space-3);
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  line-height: 1.5;
}

.pwd-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.field {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.label {
  font-size: var(--text-sm);
  font-weight: 600;
}

.form-err {
  margin: 0;
  font-size: var(--text-sm);
  color: #b91c1c;
}

.form-ok {
  margin: 0;
  font-size: var(--text-sm);
  color: #0f766e;
}

.device-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.device-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  background: var(--color-bg-surface);
  border: 1px solid var(--color-border);
  border-radius: 8px;
}

.device-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 10px;
  min-width: 0;
}

.device-meta strong {
  font-size: var(--text-sm);
}

.badge {
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 999px;
  background: rgba(13, 148, 136, 0.12);
  color: #0f766e;
}

.device-time {
  width: 100%;
  font-size: 12px;
  color: var(--color-text-muted);
}

.btn.danger {
  color: #b91c1c;
}

.tips {
  margin: var(--space-3) 0 0;
  padding-left: 1.2em;
  font-size: var(--text-xs);
  color: var(--color-text-secondary);
  line-height: 1.6;
}

.check-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-sm);
  margin-bottom: 8px;
}

.notify-perm {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin: 0 0 12px;
}

.quiet-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.field.inline {
  margin: 0;
}

.meta {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
}
</style>
