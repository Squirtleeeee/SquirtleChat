<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { parseError } from '../api/errors'
import { parseFileContent, useChatStore, type ChatMessage } from '../stores/chat'
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
const fileInput = ref<HTMLInputElement | null>(null)
const recording = ref(false)
const recordSecs = ref(0)
const previewImage = ref('')
const notice = ref('')

let mediaRecorder: MediaRecorder | null = null
let recordChunks: Blob[] = []
let recordStartedAt = 0
let recordTimer = 0
let noticeTimer = 0

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
  cancelVoiceRecord()
  window.clearTimeout(noticeTimer)
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

function flashNotice(msg: string) {
  notice.value = msg
  window.clearTimeout(noticeTimer)
  noticeTimer = window.setTimeout(() => {
    notice.value = ''
  }, 2800)
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
  if (!text || sending.value || recording.value) return
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

function pickFile() {
  if (chat.uploading || recording.value) return
  fileInput.value?.click()
}

async function onFileChange(e: Event) {
  const el = e.target as HTMLInputElement
  const file = el.files?.[0]
  el.value = ''
  if (!file) return
  try {
    await chat.uploadAndSend(file)
    await scrollBottom()
  } catch (err) {
    flashNotice(parseError(err))
  }
}

async function onPaste(e: ClipboardEvent) {
  const items = e.clipboardData?.items
  if (!items || chat.uploading || recording.value) return
  for (const item of items) {
    if (item.kind === 'file' && item.type.startsWith('image/')) {
      e.preventDefault()
      const file = item.getAsFile()
      if (!file) return
      try {
        await chat.uploadAndSend(file)
        await scrollBottom()
      } catch (err) {
        flashNotice(parseError(err))
      }
      return
    }
  }
}

async function startVoiceRecord() {
  if (recording.value || chat.uploading || !chat.activeConvId) return
  if (!navigator.mediaDevices?.getUserMedia) {
    flashNotice('当前环境不支持录音')
    return
  }
  try {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    recordChunks = []
    const mime = MediaRecorder.isTypeSupported('audio/webm;codecs=opus')
      ? 'audio/webm;codecs=opus'
      : MediaRecorder.isTypeSupported('audio/webm')
        ? 'audio/webm'
        : ''
    mediaRecorder = mime ? new MediaRecorder(stream, { mimeType: mime }) : new MediaRecorder(stream)
    mediaRecorder.ondataavailable = (ev) => {
      if (ev.data.size > 0) recordChunks.push(ev.data)
    }
    mediaRecorder.onstop = async () => {
      stream.getTracks().forEach((t) => t.stop())
      window.clearInterval(recordTimer)
      const duration = Math.max(1, Math.round((Date.now() - recordStartedAt) / 1000))
      const type = mediaRecorder?.mimeType || 'audio/webm'
      const blob = new Blob(recordChunks, { type })
      mediaRecorder = null
      recording.value = false
      recordSecs.value = 0
      if (blob.size < 200) {
        flashNotice('录音太短')
        return
      }
      try {
        await chat.sendVoice(blob, duration)
        await scrollBottom()
      } catch (err) {
        flashNotice(parseError(err))
      }
    }
    mediaRecorder.start(200)
    recording.value = true
    recordStartedAt = Date.now()
    recordSecs.value = 0
    recordTimer = window.setInterval(() => {
      recordSecs.value = Math.round((Date.now() - recordStartedAt) / 1000)
      if (recordSecs.value >= 60) stopVoiceRecord()
    }, 250)
  } catch {
    flashNotice('无法访问麦克风')
  }
}

function stopVoiceRecord() {
  if (!recording.value || !mediaRecorder) return
  if (mediaRecorder.state !== 'inactive') mediaRecorder.stop()
}

function cancelVoiceRecord() {
  if (!mediaRecorder) {
    recording.value = false
    window.clearInterval(recordTimer)
    return
  }
  const rec = mediaRecorder
  rec.ondataavailable = null
  rec.onstop = () => {
    rec.stream.getTracks().forEach((t) => t.stop())
    mediaRecorder = null
  }
  if (rec.state !== 'inactive') rec.stop()
  else rec.stream.getTracks().forEach((t) => t.stop())
  window.clearInterval(recordTimer)
  recording.value = false
  recordSecs.value = 0
  recordChunks = []
}

function formatFileSize(bytes?: number) {
  if (!bytes || bytes <= 0) return ''
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function quoteText(raw: string) {
  const reply = parseReplyContent(raw).reply
  if (!reply) return ''
  return `${reply.n}: ${reply.p}`
}

function voiceMeta(m: ChatMessage) {
  const f = parseFileContent(m.content)
  return {
    url: m.localPreview || (f?.url ? mediaUrl(f.url) : ''),
    duration: f?.duration || 1,
  }
}

function isImageMsg(m: ChatMessage) {
  if (m.msg_type === 5) return false
  if (m.msg_type === 2 || m.localPreview) return true
  const f = parseFileContent(m.content)
  return !!(f?.content_type?.startsWith('image/') && f.url)
}

function imageSrc(m: ChatMessage) {
  if (m.localPreview) return m.localPreview
  const f = parseFileContent(m.content)
  return f?.url ? mediaUrl(f.url) : ''
}

function fileMeta(m: ChatMessage) {
  return parseFileContent(m.content)
}

function displayText(m: ChatMessage) {
  if (m.msg_type === 4 || m.content === '[已撤回]') return m.content || '[已撤回]'
  const reply = parseReplyContent(m.content)
  return reply?.text || m.content
}

function hasMedia(m: ChatMessage) {
  return !!(parseFileContent(m.content) || m.localPreview)
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

    <div v-if="notice" class="wx-notice" role="status">{{ notice }}</div>

    <div v-if="loading" class="wx-loading">加载中…</div>
    <div v-else ref="listEl" class="wx-messages" role="log">
      <template v-for="m in messages" :key="m.client_msg_id">
        <div class="wx-row" :class="{ me: sameId(m.from_user_id, auth.user?.id) }">
          <div class="wx-bubble" :class="{ media: hasMedia(m) && m.msg_type !== 4 }">
            <div v-if="quoteText(m.content)" class="wx-quote">{{ quoteText(m.content) }}</div>

            <template v-if="hasMedia(m) && m.msg_type !== 4">
              <div v-if="m.msg_type === 5" class="wx-voice">
                <audio class="wx-audio" controls preload="metadata" :src="voiceMeta(m).url" />
                <span class="wx-voice-dur">{{ voiceMeta(m).duration }}″</span>
              </div>
              <div v-else-if="isImageMsg(m)" class="wx-img-wrap">
                <img
                  class="wx-img"
                  :src="imageSrc(m)"
                  alt="图片"
                  loading="lazy"
                  @click="previewImage = imageSrc(m)"
                />
                <div v-if="m.status === 'uploading'" class="wx-upload">
                  <div class="wx-upload-bar">
                    <div class="wx-upload-fill" :style="{ width: `${m.uploadProgress || 0}%` }" />
                  </div>
                  <span>{{ m.uploadProgress || 0 }}%</span>
                </div>
              </div>
              <a
                v-else-if="fileMeta(m)"
                class="wx-file"
                :href="mediaUrl(fileMeta(m)!.url)"
                target="_blank"
                rel="noopener"
              >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                  <path d="M14 2H7a2 2 0 00-2 2v16a2 2 0 002 2h10a2 2 0 002-2V8l-5-6z" stroke-linejoin="round" />
                  <path d="M14 2v6h6" stroke-linejoin="round" />
                </svg>
                <span class="wx-file-name">{{ fileMeta(m)!.filename || '[文件]' }}</span>
                <span v-if="fileMeta(m)!.size" class="wx-file-size">{{ formatFileSize(fileMeta(m)!.size) }}</span>
                <span v-if="m.status === 'uploading'" class="wx-file-up">上传中 {{ m.uploadProgress || 0 }}%</span>
              </a>
            </template>
            <span v-else class="wx-text" :class="{ recalled: m.msg_type === 4 }">{{ displayText(m) }}</span>
          </div>
        </div>
      </template>
      <p v-if="!messages.length" class="wx-empty">暂无消息</p>
    </div>

    <footer class="wx-composer">
      <input ref="fileInput" type="file" class="hidden-file" @change="onFileChange" />
      <div v-if="!recording" class="wx-tools">
        <button
          type="button"
          class="tool-btn"
          title="发送文件/图片"
          aria-label="发送文件"
          :disabled="chat.uploading || !chat.activeConvId"
          @click="pickFile"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
            <path d="M21.4 11.6l-8.5 8.5a5 5 0 01-7.1-7.1l8.5-8.5a3.2 3.2 0 014.5 4.5L10.2 17.6a1.4 1.4 0 01-2-2l7.4-7.4" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </button>
        <button
          type="button"
          class="tool-btn"
          title="语音消息"
          aria-label="语音消息"
          :disabled="chat.uploading || !chat.activeConvId"
          @click="startVoiceRecord"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
            <rect x="9" y="2" width="6" height="12" rx="3" />
            <path d="M5 11a7 7 0 0014 0M12 18v3" stroke-linecap="round" />
          </svg>
        </button>
      </div>
      <div v-else class="wx-rec-bar">
        <span class="wx-rec-dot" aria-hidden="true" />
        <span>录音中 {{ recordSecs }}s</span>
        <button type="button" class="rec-btn ghost" @click="cancelVoiceRecord">取消</button>
        <button type="button" class="rec-btn primary" @click="stopVoiceRecord">发送</button>
      </div>
      <textarea
        v-show="!recording"
        v-model="input"
        class="wx-input"
        rows="3"
        placeholder="输入消息，可粘贴图片"
        @keydown="onKeydown"
        @paste="onPaste"
      />
      <div v-show="!recording" class="wx-composer-bar">
        <span class="wx-hint">Enter 发送 · Shift+Enter 换行</span>
        <button
          type="button"
          class="wx-send"
          :disabled="sending || chat.uploading || !input.trim()"
          @click="send"
        >
          发送
        </button>
      </div>
    </footer>

    <div v-if="previewImage" class="wx-lightbox" role="dialog" @click="previewImage = ''">
      <img :src="previewImage" alt="预览" @click.stop />
      <button type="button" class="lightbox-close" aria-label="关闭" @click="previewImage = ''">×</button>
    </div>
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
  position: relative;
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

.wx-notice {
  position: absolute;
  top: 48px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 20;
  padding: 6px 14px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.72);
  color: #fff;
  font-size: 12px;
  pointer-events: none;
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

.wx-bubble.media {
  padding: 6px;
  background: transparent;
  box-shadow: none;
}

.wx-row.me .wx-bubble:not(.media) {
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

.wx-img-wrap {
  position: relative;
  display: inline-block;
}

.wx-img {
  display: block;
  max-width: 220px;
  max-height: 220px;
  border-radius: 4px;
  cursor: zoom-in;
  background: #fff;
}

.wx-upload {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  background: rgba(0, 0, 0, 0.45);
  color: #fff;
  font-size: 12px;
  border-radius: 4px;
}

.wx-upload-bar {
  width: 70%;
  height: 4px;
  background: rgba(255, 255, 255, 0.35);
  border-radius: 2px;
  overflow: hidden;
}

.wx-upload-fill {
  height: 100%;
  background: #07c160;
}

.wx-voice {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 6px;
  background: #fff;
  border-radius: 6px;
  max-width: 280px;
}

.wx-row.me .wx-voice {
  background: #95ec69;
}

.wx-audio {
  width: 180px;
  height: 32px;
}

.wx-voice-dur {
  font-size: 12px;
  color: #666;
  flex-shrink: 0;
}

.wx-file {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  background: #fff;
  border-radius: 6px;
  color: #111;
  text-decoration: none;
  max-width: 260px;
}

.wx-row.me .wx-file {
  background: #95ec69;
}

.wx-file-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
}

.wx-file-size,
.wx-file-up {
  font-size: 11px;
  color: #888;
  flex-shrink: 0;
}

.wx-text.recalled {
  color: #999;
  font-style: italic;
}

.wx-composer {
  background: #f7f7f7;
  border-top: 1px solid #e0e0e0;
  padding: 8px 12px 10px;
}

.hidden-file {
  display: none;
}

.wx-tools {
  display: flex;
  gap: 4px;
  margin-bottom: 4px;
}

.tool-btn {
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: #666;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.tool-btn:hover:not(:disabled) {
  background: rgba(0, 0, 0, 0.06);
  color: #111;
}

.tool-btn:disabled {
  opacity: 0.4;
  cursor: default;
}

.wx-rec-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 64px;
  font-size: 13px;
  color: #333;
}

.wx-rec-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #e81123;
  animation: pulse 1s ease-in-out infinite;
}

@keyframes pulse {
  50% {
    opacity: 0.35;
  }
}

.rec-btn {
  height: 28px;
  padding: 0 12px;
  border: none;
  border-radius: 4px;
  font-size: 13px;
  cursor: pointer;
}

.rec-btn.ghost {
  background: transparent;
  color: #666;
}

.rec-btn.ghost:hover {
  background: rgba(0, 0, 0, 0.06);
}

.rec-btn.primary {
  background: #07c160;
  color: #fff;
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

.wx-lightbox {
  position: fixed;
  inset: 0;
  z-index: 50;
  background: rgba(0, 0, 0, 0.82);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: zoom-out;
}

.wx-lightbox img {
  max-width: 92vw;
  max-height: 88vh;
  object-fit: contain;
  border-radius: 4px;
  cursor: default;
}

.lightbox-close {
  position: absolute;
  top: 12px;
  right: 16px;
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.12);
  color: #fff;
  font-size: 22px;
  line-height: 1;
  cursor: pointer;
}

.lightbox-close:hover {
  background: rgba(255, 255, 255, 0.22);
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
