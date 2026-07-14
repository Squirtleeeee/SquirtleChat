<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { parseError } from '../api/errors'
import { parseFileContent, useChatStore } from '../stores/chat'
import { useAuthStore } from '../stores/auth'
import { useSettingsStore } from '../stores/settings'
import { idStr, sameId } from '../utils/id'
import { isDesktopApp } from '../utils/desktop'
import { parseReplyContent } from '../utils/reply'
import { mediaUrl } from '../utils/media'

defineOptions({ name: 'ChatPopupView' })

const route = useRoute()
const chat = useChatStore()
const auth = useAuthStore()
const settings = useSettingsStore()
const desktop = isDesktopApp()

const loading = ref(true)
const input = ref('')
const sending = ref(false)
const alwaysOnTop = ref(settings.chatAlwaysOnTop)
const maximized = ref(false)
const listEl = ref<HTMLElement | null>(null)

const popupType = computed(() => String(route.query.type || 'friend'))
const popupId = computed(() => idStr(String(route.query.id || '')))
const titleHint = computed(() => String(route.query.title || ''))
const title = computed(() => chat.activeTitle || titleHint.value || '会话')
const messages = computed(() => chat.messages[chat.activeConvId] || [])

onMounted(async () => {
  document.documentElement.classList.add('popup-window')
  document.body.classList.add('popup-window')
  try {
    if (!auth.user) await auth.restoreSession()
    if (!auth.ws) auth.connectWS()
    chat.bindWS()
    await Promise.allSettled([chat.loadFriends(), chat.loadGroups(), chat.loadConversations()])
    if (popupType.value === 'group') {
      const g = chat.groups.find((x) => sameId(x.id, popupId.value))
      if (g) await chat.openGroup(g)
      else chat.setError('群聊不存在或未加入')
    } else {
      const f = chat.friends.find((x) => sameId(x.id, popupId.value))
      if (f) await chat.openDirect(f)
      else chat.setError('好友不存在')
    }
    document.title = title.value
    if (settings.chatAlwaysOnTop && window.squirtleDesktop?.setAlwaysOnTop) {
      await window.squirtleDesktop.setAlwaysOnTop(true)
    }
    if (window.squirtleDesktop?.isMaximized) {
      maximized.value = await window.squirtleDesktop.isMaximized()
    }
    await scrollBottom()
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    loading.value = false
  }
})

onUnmounted(() => {
  document.documentElement.classList.remove('popup-window')
  document.body.classList.remove('popup-window')
})

watch(title, (t) => {
  document.title = t
})

watch(
  () => messages.value.length,
  async () => {
    await scrollBottom()
  },
)

async function scrollBottom() {
  await nextTick()
  const el = listEl.value
  if (el) el.scrollTop = el.scrollHeight
}

async function togglePin() {
  alwaysOnTop.value = !alwaysOnTop.value
  settings.setChatAlwaysOnTop(alwaysOnTop.value)
  if (window.squirtleDesktop?.setAlwaysOnTop) {
    await window.squirtleDesktop.setAlwaysOnTop(alwaysOnTop.value)
  }
}

async function minimize() {
  await window.squirtleDesktop?.windowMinimize()
}

async function maximize() {
  const res = await window.squirtleDesktop?.windowMaximize()
  if (res && typeof res.maximized === 'boolean') maximized.value = res.maximized
  else if (window.squirtleDesktop?.isMaximized) maximized.value = await window.squirtleDesktop.isMaximized()
}

async function closeWin() {
  await window.squirtleDesktop?.windowClose()
  if (!desktop) window.close()
}

function send() {
  const text = input.value.trim()
  if (!text || sending.value) return
  sending.value = true
  chat.sendText(text)
  input.value = ''
  window.setTimeout(() => {
    sending.value = false
  }, 120)
  void scrollBottom()
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    send()
  }
}

function displayContent(raw: string) {
  const reply = parseReplyContent(raw)
  const text = reply?.text || raw
  const file = parseFileContent(text)
  if (file) return file.filename || '[文件]'
  return text
}

function quoteText(raw: string) {
  const reply = parseReplyContent(raw).reply
  if (!reply) return ''
  return `${reply.n}: ${reply.p}`
}

function isImage(raw: string) {
  const reply = parseReplyContent(raw)
  const text = reply?.text || raw
  const file = parseFileContent(text)
  return !!(file?.content_type?.startsWith('image/') && file.url)
}

function imageUrl(raw: string) {
  const reply = parseReplyContent(raw)
  const text = reply?.text || raw
  const file = parseFileContent(text)
  return file?.url ? mediaUrl(file.url) : ''
}
</script>

<template>
  <div class="wx-popup">
    <header class="wx-titlebar" :class="{ desktop }">
      <div class="titlebar-drag">
        <button
          v-if="desktop"
          type="button"
          class="tb-btn pin"
          :class="{ on: alwaysOnTop }"
          title="置顶"
          @click="togglePin"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
            <path d="M16 12V4h1V2H7v2h1v8l-2 2v2h5v6h2v-6h5v-2l-2-2z" />
          </svg>
        </button>
        <span class="wx-title">{{ title }}</span>
      </div>
      <div v-if="desktop" class="titlebar-controls">
        <button type="button" class="win-btn" title="最小化" @click="minimize">─</button>
        <button type="button" class="win-btn" :title="maximized ? '还原' : '最大化'" @click="maximize">
          {{ maximized ? '❐' : '□' }}
        </button>
        <button type="button" class="win-btn close" title="关闭" @click="closeWin">×</button>
      </div>
      <button v-else type="button" class="win-btn close web-close" title="关闭" @click="closeWin">×</button>
    </header>

    <div v-if="loading" class="wx-loading">加载中…</div>
    <div v-else ref="listEl" class="wx-messages" role="log">
      <template v-for="m in messages" :key="m.client_msg_id">
        <div class="wx-row" :class="{ me: sameId(m.from_user_id, auth.user?.id) }">
          <div class="wx-bubble">
            <div v-if="quoteText(m.content)" class="wx-quote">{{ quoteText(m.content) }}</div>
            <img v-if="isImage(m.content)" class="wx-img" :src="imageUrl(m.content)" alt="" />
            <span v-else class="wx-text">{{ displayContent(m.content) }}</span>
          </div>
        </div>
      </template>
      <p v-if="!messages.length" class="wx-empty">暂无消息</p>
    </div>

    <footer class="wx-composer">
      <textarea
        v-model="input"
        class="wx-input"
        rows="3"
        placeholder="输入消息"
        @keydown="onKeydown"
      />
      <div class="wx-composer-bar">
        <span class="wx-hint">Enter 发送 · Shift+Enter 换行</span>
        <button type="button" class="wx-send" :disabled="sending || !input.trim()" @click="send">发送</button>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.wx-popup {
  height: 100vh;
  height: 100dvh;
  display: grid;
  grid-template-rows: 40px 1fr auto;
  background: #ededed;
  color: #111;
  overflow: hidden;
  user-select: none;
}

.wx-titlebar {
  display: flex;
  align-items: stretch;
  background: #f7f7f7;
  border-bottom: 1px solid #e0e0e0;
}

.titlebar-drag {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 10px;
  min-width: 0;
  -webkit-app-region: drag;
  app-region: drag;
}

.tb-btn {
  -webkit-app-region: no-drag;
  app-region: no-drag;
  width: 28px;
  height: 28px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: #666;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.tb-btn:hover {
  background: rgba(0, 0, 0, 0.06);
}

.tb-btn.on {
  color: #07c160;
}

.wx-title {
  font-size: 14px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.titlebar-controls {
  display: flex;
  -webkit-app-region: no-drag;
  app-region: no-drag;
}

.win-btn {
  width: 46px;
  border: none;
  background: transparent;
  font-size: 14px;
  line-height: 40px;
  color: #333;
  cursor: pointer;
}

.win-btn:hover {
  background: rgba(0, 0, 0, 0.06);
}

.win-btn.close:hover {
  background: #e81123;
  color: #fff;
}

.web-close {
  -webkit-app-region: no-drag;
}

.wx-loading,
.wx-empty {
  margin: 48px auto;
  text-align: center;
  color: #999;
  font-size: 13px;
}

.wx-messages {
  overflow-y: auto;
  padding: 16px 20px 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  user-select: text;
}

.wx-row {
  display: flex;
  justify-content: flex-start;
}

.wx-row.me {
  justify-content: flex-end;
}

.wx-bubble {
  max-width: 72%;
  padding: 10px 12px;
  border-radius: 6px;
  background: #fff;
  font-size: 14px;
  line-height: 1.5;
  word-break: break-word;
  box-shadow: 0 1px 1px rgba(0, 0, 0, 0.04);
  white-space: pre-wrap;
}

.wx-row.me .wx-bubble {
  background: #95ec69;
}

.wx-quote {
  margin-bottom: 6px;
  padding: 6px 8px;
  border-left: 3px solid rgba(0, 0, 0, 0.15);
  background: rgba(0, 0, 0, 0.04);
  font-size: 12px;
  color: #666;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.wx-img {
  display: block;
  max-width: 220px;
  max-height: 220px;
  border-radius: 4px;
}

.wx-composer {
  background: #f7f7f7;
  border-top: 1px solid #e0e0e0;
  padding: 8px 12px 10px;
}

.wx-input {
  width: 100%;
  resize: none;
  border: none;
  outline: none;
  background: transparent;
  font-size: 14px;
  line-height: 1.5;
  font-family: inherit;
  color: #111;
  min-height: 64px;
  user-select: text;
}

.wx-composer-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 6px;
}

.wx-hint {
  font-size: 12px;
  color: #b2b2b2;
}

.wx-send {
  min-width: 72px;
  height: 30px;
  border: none;
  border-radius: 4px;
  background: #e9e9e9;
  color: #07c160;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.wx-send:hover:not(:disabled) {
  background: #d2d2d2;
}

.wx-send:disabled {
  color: #b2b2b2;
  cursor: default;
}
</style>

<style>
html.popup-window,
body.popup-window,
body.popup-window #app {
  margin: 0;
  height: 100%;
  overflow: hidden;
  background: #ededed !important;
}
</style>
