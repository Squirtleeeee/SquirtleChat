<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { parseError } from '../api/errors'
import AddContactModal from '../components/AddContactModal.vue'
import ImageLightbox from '../components/ImageLightbox.vue'
import UserAvatar from '../components/UserAvatar.vue'
import { useAuthStore } from '../stores/auth'
import {
  friendDisplayName,
  parseFileContent,
  useChatStore,
  type ChatMessage,
  type FriendWithConv,
  type GroupWithConv,
} from '../stores/chat'
import type { PublicProfile } from '../stores/auth'
import { dayKey, formatDateDivider, formatListTime, formatMessageTime } from '../utils/format'
import { ensureNotifyPermission } from '../utils/notify'
import { directConvId, sameId } from '../utils/id'
import { mediaUrl } from '../utils/media'
import { isAgentProfile, AGENT_AVATAR } from '../constants/agent'
import { parseReplyContent, type ReplyMeta } from '../utils/reply'
import { useSettingsStore } from '../stores/settings'
import { openDetachedChat } from '../utils/desktop'

defineOptions({ name: 'ChatView' })

const auth = useAuthStore()
const chat = useChatStore()
const settings = useSettingsStore()
const router = useRouter()
const input = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const messageListEl = ref<HTMLElement | null>(null)
const showAddModal = ref(false)
const filterQuery = ref('')
const previewImage = ref('')
const showSearch = ref(false)
const searchInput = ref('')
const searchInputEl = ref<HTMLInputElement | null>(null)
const skipAutoScroll = ref(false)
let searchDebounce = 0
const DRAFT_KEY = 'squirtlechat_drafts'
const draftsMap = ref<Record<string, string>>(loadDrafts())
const mentionOpen = ref(false)
const mentionQuery = ref('')
const mentionStart = ref(-1)
const composerInputEl = ref<HTMLTextAreaElement | null>(null)
const showEmoji = ref(false)
const sidebarLoading = ref(true)
const historyLoading = ref(false)
const preciseTimeIds = ref<Set<string>>(new Set())
type CtxMenu =
  | { kind: 'friend'; id: string; x: number; y: number }
  | { kind: 'group'; id: string; x: number; y: number }
  | null
const ctxMenu = ref<CtxMenu>(null)
type MsgMenu = {
  message: ChatMessage
  x: number
  y: number
} | null
const msgMenu = ref<MsgMenu>(null)
const mobileSidebarOpen = ref(true)
const nearBottom = ref(true)
const pendingNewCount = ref(0)
const selectMode = ref(false)
const selectedMsgIds = ref<Set<string>>(new Set())
/** seq after which messages are "new" when opening a conversation with unread */
const unreadDividerAfterSeq = ref(0)
const replyTarget = ref<ReplyMeta | null>(null)
const sendingLock = ref(false)
const confirmDialog = ref<{
  title: string
  body: string
  confirmLabel?: string
  danger?: boolean
  onConfirm: () => void | Promise<void>
} | null>(null)
let longPressTimer = 0
let longPressMoved = false
let suppressNextClick = false

const selectedCount = computed(() => selectedMsgIds.value.size)

const hasVisibleUploadBubble = computed(() => {
  if (!chat.uploading || !chat.activeConvId) return false
  const list = chat.messages[chat.activeConvId] || []
  return list.some((m) => m.status === 'uploading')
})

const EMOJIS = [
  '😀', '😁', '😂', '🤣', '😊', '😍', '😘', '😜',
  '🤔', '😅', '😢', '😭', '😡', '👍', '👎', '👏',
  '🙏', '🔥', '✨', '🎉', '❤️', '💔', '⭐', '✅',
  '🐶', '🐱', '🌸', '☀️', '🌙', '☕', '🍕', '🎵',
]

const mentionCandidates = computed(() => {
  if (!chat.activeGroupId || !mentionOpen.value) return [] as PublicProfile[]
  const q = mentionQuery.value.trim().toLowerCase()
  return chat.activeGroupMembers
    .filter((m) => !sameId(m.id, auth.user?.id))
    .filter((m) => {
      if (!q) return true
      const name = chat.mentionName(m).toLowerCase()
      return name.includes(q) || m.username.toLowerCase().includes(q)
    })
    .slice(0, 8)
})

function loadDrafts(): Record<string, string> {
  try {
    const raw = localStorage.getItem(DRAFT_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as Record<string, string>
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

function persistDrafts() {
  localStorage.setItem(DRAFT_KEY, JSON.stringify(draftsMap.value))
}

function saveDraft(convId: string, text: string) {
  if (!convId) return
  const next = { ...draftsMap.value }
  if (!text) {
    if (!(convId in next)) return
    delete next[convId]
  } else {
    next[convId] = text
  }
  draftsMap.value = next
  persistDrafts()
}

function readDraft(convId: string) {
  if (!convId) return ''
  return draftsMap.value[convId] || ''
}

function draftPreview(convId: string) {
  const d = (draftsMap.value[convId] || '').trim()
  if (!d) return ''
  return d.length > 24 ? `${d.slice(0, 24)}…` : d
}

type DisplayItem =
  | { kind: 'divider'; key: string; label: string; variant?: 'date' | 'unread' }
  | {
      kind: 'message'
      key: string
      message: ChatMessage
      /** WeChat-style consecutive merge */
      cluster: 'solo' | 'first' | 'middle' | 'last'
      showSender: boolean
      showTime: boolean
    }

const CLUSTER_GAP_MS = 2 * 60 * 1000

function sameCluster(a: ChatMessage, b: ChatMessage) {
  if (!sameId(a.from_user_id, b.from_user_id)) return false
  if (!a.created_at || !b.created_at) return true
  return Math.abs(new Date(b.created_at).getTime() - new Date(a.created_at).getTime()) <= CLUSTER_GAP_MS
}

const activeTitle = computed(() => chat.activeTitle || 'SquirtleChat')
const filterText = computed(() => filterQuery.value.trim().toLowerCase())
const filteredFriends = computed(() => {
  const q = filterText.value
  if (!q) return chat.sortedFriends
  return chat.sortedFriends.filter((f) => {
    const name = friendDisplayName(f).toLowerCase()
    const user = f.username.toLowerCase()
    return name.includes(q) || user.includes(q) || (f.remark || '').toLowerCase().includes(q)
  })
})
const filteredGroups = computed(() => {
  const q = filterText.value
  if (!q) return chat.sortedGroups
  return chat.sortedGroups.filter((g) => g.name.toLowerCase().includes(q))
})
const displayItems = computed((): DisplayItem[] => {
  const list = chat.messages[chat.activeConvId || ''] || []
  const items: DisplayItem[] = []
  let lastDay = ''
  let unreadInserted = false
  const afterSeq = unreadDividerAfterSeq.value
  /** messages in order with clusterBreakBefore if a divider was inserted before them */
  const rows: { m: ChatMessage; breakBefore: boolean }[] = []

  for (const m of list) {
    let breakBefore = false
    if (
      afterSeq > 0 &&
      !unreadInserted &&
      (m.seq || 0) > afterSeq &&
      !sameId(m.from_user_id, auth.user?.id)
    ) {
      items.push({ kind: 'divider', key: 'unread-sep', label: '以下为新消息', variant: 'unread' })
      unreadInserted = true
      breakBefore = true
    }
    const dk = dayKey(m.created_at)
    if (dk && dk !== lastDay) {
      items.push({ kind: 'divider', key: `d-${dk}`, label: formatDateDivider(m.created_at), variant: 'date' })
      lastDay = dk
      breakBefore = true
    }
    rows.push({ m, breakBefore })
  }

  for (let i = 0; i < rows.length; i++) {
    const { m, breakBefore } = rows[i]
    const prev = i > 0 && !breakBefore ? rows[i - 1].m : null
    const nextBreak = i < rows.length - 1 ? rows[i + 1].breakBefore : true
    const next = i < rows.length - 1 && !nextBreak ? rows[i + 1].m : null

    const withPrev = !!(prev && sameCluster(prev, m))
    const withNext = !!(next && sameCluster(m, next))

    let cluster: 'solo' | 'first' | 'middle' | 'last' = 'solo'
    if (withPrev && withNext) cluster = 'middle'
    else if (!withPrev && withNext) cluster = 'first'
    else if (withPrev && !withNext) cluster = 'last'

    items.push({
      kind: 'message',
      key: m.client_msg_id,
      message: m,
      cluster,
      showSender:
        !!chat.activeGroupId &&
        !sameId(m.from_user_id, auth.user?.id) &&
        (cluster === 'solo' || cluster === 'first'),
      showTime: cluster === 'solo' || cluster === 'last',
    })
  }
  return items
})
const wsStatusLabel = computed(() => {
  if (chat.wsStatus === 'open') {
    return chat.syncingAfterReconnect ? '同步中…' : '已连接'
  }
  if (chat.wsStatus === 'connecting') {
    const n = chat.reconnectAttempt
    return n > 0 ? `重连中（第 ${n} 次）…` : '连接中…'
  }
  const n = chat.reconnectAttempt
  return n > 0 ? `未连接，即将重试…` : '未连接'
})
const wsStatusClass = computed(() => ({
  online: chat.wsStatus === 'open' && !chat.syncingAfterReconnect,
  connecting: chat.wsStatus === 'connecting' || chat.syncingAfterReconnect,
  offline: chat.wsStatus === 'closed',
}))
const showReconnectBanner = computed(() => {
  if (chat.syncingAfterReconnect) return true
  if (chat.wsStatus === 'open') return false
  return chat.wasDisconnected || chat.reconnectAttempt > 0
})
const reconnectBannerText = computed(() => {
  if (chat.syncingAfterReconnect) return '连接已恢复，正在同步离线消息…'
  if (chat.wsStatus === 'connecting') {
    const n = chat.reconnectAttempt
    return n > 0 ? `连接断开，正在重连（第 ${n} 次）…` : '正在连接…'
  }
  return '连接已断开，将自动重连'
})
const selfName = computed(() => auth.user?.nickname || auth.user?.username || '')
const selfAvatar = computed(() => avatarUrl(auth.user?.avatar))

function avatarUrl(url?: string) {
  return mediaUrl(url)
}

function friendAvatarUrl(f: { username?: string; avatar?: string }) {
  if (isAgentProfile(f)) return AGENT_AVATAR
  return avatarUrl(f.avatar)
}

function onGlobalKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    if (confirmDialog.value) {
      closeConfirm()
      e.preventDefault()
      return
    }
    if (msgMenu.value) {
      msgMenu.value = null
      e.preventDefault()
      return
    }
    if (ctxMenu.value) {
      closeCtxMenu()
      e.preventDefault()
      return
    }
    if (mobileSidebarOpen.value && chat.activeConvId) {
      mobileSidebarOpen.value = false
      e.preventDefault()
      return
    }
    if (showEmoji.value) {
      showEmoji.value = false
      e.preventDefault()
      return
    }
    if (showSearch.value) {
      toggleSearch()
      e.preventDefault()
      return
    }
    if (mentionOpen.value) {
      mentionOpen.value = false
      e.preventDefault()
      return
    }
    if (replyTarget.value) {
      replyTarget.value = null
      e.preventDefault()
      return
    }
    if (showAddModal.value) {
      showAddModal.value = false
      e.preventDefault()
      return
    }
    if (previewImage.value) {
      previewImage.value = ''
      e.preventDefault()
    }
    return
  }
  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter' && chat.activeConvId) {
    e.preventDefault()
    send()
  }
}

onMounted(async () => {
  document.documentElement.classList.add('chat-route-lock')
  if (!auth.isLogin) {
    router.replace('/login')
    return
  }
  if (window.squirtleDesktop?.setShellMode) {
    void window.squirtleDesktop.setShellMode('main')
  }
  if (!auth.user) await auth.restoreSession()
  window.addEventListener('keydown', onGlobalKeydown)
  window.addEventListener('click', closeCtxMenu)
  sidebarLoading.value = true
  try {
    if (!auth.ws) auth.connectWS()
    chat.bindWS()
    void ensureNotifyPermission()
    await Promise.allSettled([
      chat.ensureAgent(),
      chat.loadFriends(),
      chat.loadGroups(),
      chat.loadPending(),
      chat.loadConversations(),
    ])
    await chat.pullSync()
    setInterval(() => chat.pullSync(), 3000)
    setInterval(() => chat.loadPending(), 8000)
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    sidebarLoading.value = false
  }
})

onUnmounted(() => {
  document.documentElement.classList.remove('chat-route-lock')
  window.removeEventListener('keydown', onGlobalKeydown)
  window.removeEventListener('click', closeCtxMenu)
  clearLongPress()
})

function send() {
  if (mentionOpen.value && mentionCandidates.value.length) {
    insertMention(mentionCandidates.value[0])
    return
  }
  const text = input.value.trim()
  if (!text || sendingLock.value) return
  sendingLock.value = true
  chat.sendText(text, replyTarget.value)
  input.value = ''
  replyTarget.value = null
  mentionOpen.value = false
  if (chat.activeConvId) saveDraft(chat.activeConvId, '')
  nearBottom.value = true
  void scrollToBottom(true, true)
  void nextTick(() => {
    autoResizeComposer()
    window.setTimeout(() => {
      sendingLock.value = false
    }, 280)
  })
}

function onComposerKeydown(e: KeyboardEvent) {
  if (e.key !== 'Enter') return
  if (e.shiftKey) return
  e.preventDefault()
  send()
}

function autoResizeComposer() {
  const el = composerInputEl.value
  if (!el) return
  const prev = el.style.height
  el.style.height = 'auto'
  const next = `${Math.min(el.scrollHeight, 120)}px`
  if (prev === next) {
    el.style.height = next
    return
  }
  el.style.height = next
}

function onComposerInput() {
  if (chat.activeConvId) saveDraft(chat.activeConvId, input.value)
  if (input.value.trim()) chat.notifyTyping(true)
  else chat.notifyTyping(false)
  updateMentionState()
  autoResizeComposer()
}

function updateMentionState() {
  if (!chat.activeGroupId) {
    mentionOpen.value = false
    return
  }
  const el = composerInputEl.value
  const caret = el?.selectionStart ?? input.value.length
  const before = input.value.slice(0, caret)
  const at = before.lastIndexOf('@')
  if (at < 0) {
    mentionOpen.value = false
    return
  }
  if (at > 0 && !/\s/.test(before[at - 1])) {
    mentionOpen.value = false
    return
  }
  const query = before.slice(at + 1)
  if (/\s/.test(query)) {
    mentionOpen.value = false
    return
  }
  mentionStart.value = at
  mentionQuery.value = query
  mentionOpen.value = true
}

function insertMention(user: PublicProfile) {
  const name = chat.mentionName(user)
  const start = mentionStart.value
  const el = composerInputEl.value
  const caret = el?.selectionStart ?? input.value.length
  if (start < 0) return
  const before = input.value.slice(0, start)
  const after = input.value.slice(caret)
  const insert = `@${name} `
  input.value = before + insert + after
  mentionOpen.value = false
  if (chat.activeConvId) saveDraft(chat.activeConvId, input.value)
  void nextTick(() => {
    const pos = before.length + insert.length
    el?.focus()
    el?.setSelectionRange(pos, pos)
  })
}

function insertEmoji(emoji: string) {
  const el = composerInputEl.value
  const caret = el?.selectionStart ?? input.value.length
  const before = input.value.slice(0, caret)
  const after = input.value.slice(caret)
  input.value = before + emoji + after
  if (chat.activeConvId) saveDraft(chat.activeConvId, input.value)
  if (input.value.trim()) chat.notifyTyping(true)
  void nextTick(() => {
    const pos = before.length + emoji.length
    el?.focus()
    el?.setSelectionRange(pos, pos)
  })
}

function toggleEmoji() {
  showEmoji.value = !showEmoji.value
  if (showEmoji.value) mentionOpen.value = false
  void nextTick(() => composerInputEl.value?.focus())
}

function messageBody(m: ChatMessage): string {
  if (m.msg_type !== 1) return m.content
  return parseReplyContent(m.content).text
}

function messageReply(m: ChatMessage): ReplyMeta | null {
  if (m.msg_type !== 1) return null
  return parseReplyContent(m.content).reply
}

function renderMessageHtml(content: string) {
  const body = parseReplyContent(content).text
  const escaped = body
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
  const withLinks = escaped.replace(
    /(https?:\/\/[^\s<]+)/g,
    (url) => {
      const clean = url.replace(/[.,;:!?)]+$/, '')
      const trailing = url.slice(clean.length)
      return `<a class="msg-link" href="${clean}" target="_blank" rel="noopener noreferrer">${clean}</a>${trailing}`
    },
  )
  return withLinks
    .replace(/@([^\s@]+)/g, '<span class="mention">@$1</span>')
    .replace(/\n/g, '<br>')
}

function canReply(m: ChatMessage) {
  return m.msg_type !== 4 && m.content !== '[已撤回]'
}

function formatFileSize(bytes?: number) {
  if (bytes == null || !Number.isFinite(bytes) || bytes < 0) return ''
  if (bytes < 1024) return `${Math.round(bytes)} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(bytes < 10 * 1024 ? 1 : 0)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function startReply(m: ChatMessage) {
  if (!canReply(m)) return
  let preview = messageBody(m)
  if (m.msg_type === 2 || m.localPreview) preview = '[图片]'
  else if (m.msg_type === 3 || parseFileContent(m.content)) preview = '[文件]'
  else if (parseFileContent(m.content)?.content_type?.startsWith('image/')) preview = '[图片]'
  replyTarget.value = {
    n: senderLabel(m),
    p: preview.slice(0, 80),
    c: m.client_msg_id,
  }
  void nextTick(() => composerInputEl.value?.focus())
}

function cancelReply() {
  replyTarget.value = null
  void nextTick(() => composerInputEl.value?.focus())
}

async function jumpToQuoted(m: ChatMessage) {
  const reply = messageReply(m)
  const targetId = reply?.c
  if (!targetId || !chat.activeConvId) return
  const list = chat.messages[chat.activeConvId] || []
  const found = list.find((x) => x.client_msg_id === targetId)
  if (!found) {
    chat.setTransientNotice('原消息不在当前已加载范围')
    return
  }
  skipAutoScroll.value = true
  chat.setHighlight(targetId)
  await nextTick()
  const el = messageListEl.value?.querySelector(
    `[data-client-msg-id="${CSS.escape(targetId)}"]`,
  ) as HTMLElement | null
  el?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  window.setTimeout(() => {
    skipAutoScroll.value = false
  }, 400)
}

async function acceptFriend(reqId: string) {
  try {
    await chat.acceptFriend(reqId)
  } catch (e) {
    chat.setError(parseError(e))
  }
}

async function acceptGroupInvite(inviteId: string) {
  try {
    await chat.acceptGroupInvitation(inviteId)
  } catch (e) {
    chat.setError(parseError(e))
  }
}

async function rejectGroupInvite(inviteId: string) {
  try {
    await chat.rejectGroupInvitation(inviteId)
  } catch (e) {
    chat.setError(parseError(e))
  }
}

async function rejectFriend(reqId: string) {
  try {
    await chat.rejectFriend(reqId)
  } catch (e) {
    chat.setError(parseError(e))
  }
}

async function deleteFriend(fid: string, e: Event) {
  e.stopPropagation()
  confirmDialog.value = {
    title: '删除好友',
    body: '确定删除该好友？',
    confirmLabel: '删除',
    danger: true,
    onConfirm: async () => {
      try {
        await chat.deleteFriend(fid)
      } catch (err) {
        chat.setError(parseError(err))
      }
    },
  }
}

function pickFile() {
  fileInput.value?.click()
}

async function onFileChange(e: Event) {
  const el = e.target as HTMLInputElement
  const file = el.files?.[0]
  el.value = ''
  if (!file) return
  try {
    await chat.uploadAndSend(file)
  } catch (err) {
    chat.setError(parseError(err))
  }
}

function fileUrl(url: string) {
  return avatarUrl(url)
}

function openImagePreview(url: string) {
  previewImage.value = fileUrl(url)
}

async function onComposerPaste(e: ClipboardEvent) {
  const items = e.clipboardData?.items
  if (!items) return
  for (const item of items) {
    if (item.type.startsWith('image/')) {
      e.preventDefault()
      const file = item.getAsFile()
      if (file) await chat.uploadAndSend(file)
      return
    }
  }
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    chat.setTransientNotice('已复制')
  } catch {
    chat.setError('复制失败')
  }
}

function togglePreciseTime(clientMsgId: string) {
  const next = new Set(preciseTimeIds.value)
  if (next.has(clientMsgId)) next.delete(clientMsgId)
  else next.add(clientMsgId)
  preciseTimeIds.value = next
}

function onBubbleDblClick(m: ChatMessage) {
  if (!isTextMessage(m)) return
  void copyText(messageBody(m))
}

function clampCtxPos(x: number, y: number, w = 168, h = 140) {
  const pad = 8
  return {
    x: Math.min(Math.max(pad, x), window.innerWidth - w - pad),
    y: Math.min(Math.max(pad, y), window.innerHeight - h - pad),
  }
}

function openFriendCtx(f: FriendWithConv, e: MouseEvent) {
  e.preventDefault()
  const p = clampCtxPos(e.clientX, e.clientY)
  ctxMenu.value = { kind: 'friend', id: f.id, x: p.x, y: p.y }
}

function openGroupCtx(g: GroupWithConv, e: MouseEvent) {
  e.preventDefault()
  const p = clampCtxPos(e.clientX, e.clientY)
  ctxMenu.value = { kind: 'group', id: g.id, x: p.x, y: p.y }
}

function clearLongPress() {
  if (longPressTimer) {
    window.clearTimeout(longPressTimer)
    longPressTimer = 0
  }
}

function onFriendTouchStart(f: FriendWithConv, e: TouchEvent) {
  longPressMoved = false
  clearLongPress()
  const t = e.touches[0]
  if (!t) return
  const x = t.clientX
  const y = t.clientY
  longPressTimer = window.setTimeout(() => {
    longPressTimer = 0
    if (longPressMoved) return
    suppressNextClick = true
    const p = clampCtxPos(x, y)
    ctxMenu.value = { kind: 'friend', id: f.id, x: p.x, y: p.y }
  }, 480)
}

function onGroupTouchStart(g: GroupWithConv, e: TouchEvent) {
  longPressMoved = false
  clearLongPress()
  const t = e.touches[0]
  if (!t) return
  const x = t.clientX
  const y = t.clientY
  longPressTimer = window.setTimeout(() => {
    longPressTimer = 0
    if (longPressMoved) return
    suppressNextClick = true
    const p = clampCtxPos(x, y)
    ctxMenu.value = { kind: 'group', id: g.id, x: p.x, y: p.y }
  }, 480)
}

function onListTouchMove() {
  longPressMoved = true
  clearLongPress()
}

function onListTouchEnd() {
  clearLongPress()
}

function closeCtxMenu() {
  ctxMenu.value = null
  msgMenu.value = null
}

function openMsgMenu(m: ChatMessage, x: number, y: number) {
  if (selectMode.value) return
  const p = clampCtxPos(x, y, 160, 160)
  msgMenu.value = { message: m, x: p.x, y: p.y }
  ctxMenu.value = null
}

function onBubbleContextMenu(m: ChatMessage, e: MouseEvent) {
  e.preventDefault()
  openMsgMenu(m, e.clientX, e.clientY)
}

let msgLongPressTimer = 0
let bubbleTouchMsg: ChatMessage | null = null
let bubbleTouchX = 0
let bubbleTouchY = 0
/** pending = watching; active = horizontal swipe-to-reply */
let bubbleSwipeMode: false | 'pending' | 'active' = false
const swipeMsgId = ref('')
const swipeOffset = ref(0)
const SWIPE_REPLY_THRESHOLD = 56
const SWIPE_REPLY_MAX = 72

function clearMsgLongPress() {
  if (msgLongPressTimer) {
    window.clearTimeout(msgLongPressTimer)
    msgLongPressTimer = 0
  }
}

function resetBubbleSwipe() {
  bubbleSwipeMode = false
  bubbleTouchMsg = null
  swipeMsgId.value = ''
  swipeOffset.value = 0
}

function onBubbleTouchStart(m: ChatMessage, e: TouchEvent) {
  if (selectMode.value) return
  clearMsgLongPress()
  longPressMoved = false
  resetBubbleSwipe()
  const t = e.touches[0]
  if (!t) return
  bubbleTouchMsg = m
  bubbleTouchX = t.clientX
  bubbleTouchY = t.clientY
  bubbleSwipeMode = canReply(m) ? 'pending' : false
  swipeMsgId.value = m.client_msg_id
  const x = t.clientX
  const y = t.clientY
  msgLongPressTimer = window.setTimeout(() => {
    msgLongPressTimer = 0
    if (longPressMoved || bubbleSwipeMode === 'active') return
    suppressNextClick = true
    resetBubbleSwipe()
    openMsgMenu(m, x, y)
  }, 480)
}

function onBubbleTouchMove(e: TouchEvent) {
  const t = e.touches[0]
  if (!t || !bubbleTouchMsg) return
  const dx = t.clientX - bubbleTouchX
  const dy = t.clientY - bubbleTouchY

  if (bubbleSwipeMode === 'pending') {
    if (Math.abs(dy) > 12 && Math.abs(dy) >= Math.abs(dx)) {
      bubbleSwipeMode = false
      longPressMoved = true
      clearMsgLongPress()
      swipeOffset.value = 0
      return
    }
    if (dx > 14 && Math.abs(dx) > Math.abs(dy) * 1.15) {
      bubbleSwipeMode = 'active'
      longPressMoved = true
      clearMsgLongPress()
    }
  }

  if (bubbleSwipeMode === 'active') {
    swipeOffset.value = Math.max(0, Math.min(SWIPE_REPLY_MAX, dx))
    return
  }

  if (Math.abs(dx) > 8 || Math.abs(dy) > 8) {
    longPressMoved = true
    clearMsgLongPress()
  }
}

function onBubbleTouchEnd() {
  clearMsgLongPress()
  const m = bubbleTouchMsg
  const shouldReply =
    bubbleSwipeMode === 'active' &&
    swipeOffset.value >= SWIPE_REPLY_THRESHOLD &&
    m &&
    canReply(m)
  if (shouldReply && m) {
    suppressNextClick = true
    startReply(m)
  }
  resetBubbleSwipe()
}

function runMsgMenu(action: 'reply' | 'copy' | 'recall' | 'select') {
  const menu = msgMenu.value
  msgMenu.value = null
  if (!menu) return
  const m = menu.message
  if (action === 'reply') startReply(m)
  else if (action === 'copy') void copyText(messageBody(m))
  else if (action === 'recall') void recallMsg(m)
  else if (action === 'select') {
    selectMode.value = true
    toggleSelectMsg(m.client_msg_id)
  }
}

function msgMenuReply() {
  runMsgMenu('reply')
}
function msgMenuCopy() {
  runMsgMenu('copy')
}
function msgMenuRecall() {
  runMsgMenu('recall')
}
function msgMenuSelect() {
  runMsgMenu('select')
}

function ctxGroup() {
  if (ctxMenu.value?.kind !== 'group') return null
  return chat.groups.find((g) => g.id === ctxMenu.value!.id) || null
}

async function ctxOpenChat() {
  const menu = ctxMenu.value
  closeCtxMenu()
  if (!menu) return
  if (menu.kind === 'friend') {
    const f = chat.sortedFriends.find((x) => sameId(x.id, menu.id))
    if (f) await openFriend(f)
  } else {
    const g = chat.sortedGroups.find((x) => x.id === menu.id)
    if (g) await openGroupChat(g)
  }
}

function ctxTogglePin() {
  const menu = ctxMenu.value
  closeCtxMenu()
  if (!menu) return
  if (menu.kind === 'friend') chat.togglePinFriend(menu.id)
  else chat.togglePinGroup(menu.id)
}

function ctxToggleMute() {
  const menu = ctxMenu.value
  closeCtxMenu()
  if (!menu) return
  if (menu.kind === 'friend') {
    chat.toggleMute(directConvId(auth.user?.id || '', menu.id))
  } else {
    const g = chat.groups.find((x) => x.id === menu.id)
    if (g) chat.toggleMute(g.conversation_id)
  }
}

function ctxOpenProfile() {
  const menu = ctxMenu.value
  closeCtxMenu()
  if (!menu) return
  if (menu.kind === 'friend') router.push(`/profile/${menu.id}`)
  else router.push(`/group/${menu.id}`)
}

async function clearLocalChat() {
  if (!chat.activeConvId) return
  confirmDialog.value = {
    title: '清空本地缓存',
    body: '清空本机该会话消息缓存？不会删除服务器上的聊天记录。',
    confirmLabel: '清空',
    danger: true,
    onConfirm: () => {
      chat.clearLocalMessages()
      selectMode.value = false
      selectedMsgIds.value = new Set()
    },
  }
}

async function reloadHistory() {
  historyLoading.value = true
  try {
    await chat.reloadActiveHistory()
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    historyLoading.value = false
  }
}

function openAddModal() {
  showAddModal.value = true
}

function pinFriend(id: string, e: Event) {
  e.stopPropagation()
  chat.togglePinFriend(id)
}

function pinGroup(id: string, e: Event) {
  e.stopPropagation()
  chat.togglePinGroup(id)
}

function isTextMessage(m: ChatMessage) {
  return m.msg_type === 1 && !parseFileContent(m.content) && m.content !== '[已撤回]'
}

function toggleSelectMode() {
  selectMode.value = !selectMode.value
  if (!selectMode.value) selectedMsgIds.value = new Set()
}

function toggleSelectMsg(clientMsgId: string) {
  const next = new Set(selectedMsgIds.value)
  if (next.has(clientMsgId)) next.delete(clientMsgId)
  else next.add(clientMsgId)
  selectedMsgIds.value = next
}

function isMsgSelected(clientMsgId: string) {
  return selectedMsgIds.value.has(clientMsgId)
}

function selectedMessagesInOrder(): ChatMessage[] {
  const list = chat.messages[chat.activeConvId || ''] || []
  return list.filter((m) => selectedMsgIds.value.has(m.client_msg_id))
}

function formatForwardLine(m: ChatMessage) {
  const who = sameId(m.from_user_id, auth.user?.id)
    ? '我'
    : (() => {
        const f = chat.friends.find((x) => sameId(x.id, m.from_user_id))
        return f ? friendDisplayName(f) : '对方'
      })()
  const time = formatMessageTime(m.created_at)
  let body = messageBody(m)
  if (m.msg_type === 2) body = '[图片]'
  else if (m.msg_type === 3) body = '[文件]'
  else if (m.msg_type === 4) body = '[已撤回]'
  return `${who}${time ? ` ${time}` : ''}: ${body}`
}

async function copySelectedMessages() {
  const msgs = selectedMessagesInOrder()
  if (!msgs.length) return
  const text = msgs.map(formatForwardLine).join('\n')
  await copyText(text)
  selectMode.value = false
  selectedMsgIds.value = new Set()
}

function canRecall(m: ChatMessage) {
  if (!sameId(m.from_user_id, auth.user?.id)) return false
  if (m.msg_type === 4 || m.content === '[已撤回]') return false
  if (!m.msg_id) return false
  if (!m.created_at) return true
  return Date.now() - new Date(m.created_at).getTime() < 2 * 60 * 1000
}

async function recallMsg(m: ChatMessage) {
  await chat.recallMessage(m)
}

function goProfile(id?: string) {
  router.push(id ? `/profile/${id}` : '/profile')
}

function openChatHeader() {
  if (chat.activeGroupId) {
    router.push(`/group/${chat.activeGroupId}`)
  } else if (chat.activeToUser) {
    router.push(`/profile/${chat.activeToUser}`)
  }
}

function logout() {
  confirmDialog.value = {
    title: '退出登录',
    body: '确定退出登录？',
    confirmLabel: '退出',
    danger: true,
    onConfirm: () => {
      void auth.logout()
      router.push('/login')
    },
  }
}

async function runConfirm() {
  const d = confirmDialog.value
  if (!d) return
  confirmDialog.value = null
  await d.onConfirm()
}

function closeConfirm() {
  confirmDialog.value = null
}

async function loadOlder() {
  if (!chat.activeConvId || historyLoading.value) return
  const el = messageListEl.value
  const prevHeight = el?.scrollHeight ?? 0
  const prevTop = el?.scrollTop ?? 0
  historyLoading.value = true
  skipAutoScroll.value = true
  try {
    await chat.loadMoreHistory(chat.activeConvId)
    await nextTick()
    if (el) {
      el.scrollTop = prevTop + (el.scrollHeight - prevHeight)
    }
  } finally {
    historyLoading.value = false
    window.setTimeout(() => {
      skipAutoScroll.value = false
    }, 80)
  }
}

function captureUnreadDivider(convId: string) {
  const conv = chat.conversations.find((c) => c.conversation_id === convId)
  if (conv && (conv.unread_count || 0) > 0) {
    unreadDividerAfterSeq.value = conv.last_read_seq || 0
  } else {
    unreadDividerAfterSeq.value = 0
  }
}

async function openFriend(f: FriendWithConv) {
  if (suppressNextClick) {
    suppressNextClick = false
    return
  }
  if (settings.layoutMode === 'detached') {
    await openDetachedChat({
      type: 'friend',
      id: String(f.id),
      title: friendDisplayName(f),
    })
    return
  }
  const convId = directConvId(auth.user?.id || '', f.id)
  const hasCached = (chat.messages[convId] || []).length > 0
  if (!hasCached) historyLoading.value = true
  try {
    captureUnreadDivider(convId)
    await chat.openDirect(f)
    mobileSidebarOpen.value = false
    await scrollToLatestOnOpen()
  } finally {
    historyLoading.value = false
  }
}

async function openGroupChat(g: GroupWithConv) {
  if (suppressNextClick) {
    suppressNextClick = false
    return
  }
  if (settings.layoutMode === 'detached') {
    await openDetachedChat({
      type: 'group',
      id: String(g.id),
      title: g.name,
    })
    return
  }
  const hasCached = (chat.messages[g.conversation_id] || []).length > 0
  if (!hasCached) historyLoading.value = true
  try {
    captureUnreadDivider(g.conversation_id)
    await chat.openGroup(g)
    mobileSidebarOpen.value = false
    await scrollToLatestOnOpen()
  } finally {
    historyLoading.value = false
  }
}

async function ctxOpenDetached() {
  const menu = ctxMenu.value
  closeCtxMenu()
  if (!menu) return
  if (menu.kind === 'friend') {
    const f = chat.sortedFriends.find((x) => sameId(x.id, menu.id))
    if (f) {
      await openDetachedChat({
        type: 'friend',
        id: String(f.id),
        title: friendDisplayName(f),
      })
    }
  } else {
    const g = chat.sortedGroups.find((x) => x.id === menu.id)
    if (g) {
      await openDetachedChat({
        type: 'group',
        id: String(g.id),
        title: g.name,
      })
    }
  }
}

function showMobileSidebar() {
  mobileSidebarOpen.value = true
}

function isNearBottom(el: HTMLElement, threshold = 80) {
  return el.scrollHeight - el.scrollTop - el.clientHeight <= threshold
}

function onMessageListScroll() {
  const el = messageListEl.value
  if (!el) return
  nearBottom.value = isNearBottom(el)
  if (nearBottom.value) pendingNewCount.value = 0
}

async function scrollToBottom(force = false, smooth = false) {
  await nextTick()
  await new Promise<void>((resolve) => {
    requestAnimationFrame(() => requestAnimationFrame(() => resolve()))
  })
  const el = messageListEl.value
  if (!el) return
  if (!force && !nearBottom.value && !skipAutoScroll.value) return
  const top = el.scrollHeight
  if (smooth) {
    el.scrollTo({ top, behavior: 'smooth' })
  } else {
    el.scrollTop = top
  }
  nearBottom.value = true
  pendingNewCount.value = 0
}

/** 打开会话或历史加载后，多次尝试滚到最新消息（等待 DOM 布局完成） */
async function scrollToLatestOnOpen() {
  nearBottom.value = true
  pendingNewCount.value = 0
  skipAutoScroll.value = true
  try {
    for (let i = 0; i < 4; i++) {
      await scrollToBottom(true, false)
      await new Promise<void>((r) => requestAnimationFrame(() => r()))
    }
  } finally {
    window.setTimeout(() => {
      skipAutoScroll.value = false
    }, 80)
  }
}

function jumpToLatest() {
  void scrollToBottom(true, true)
}

function toggleSearch() {
  showSearch.value = !showSearch.value
  if (!showSearch.value) {
    searchInput.value = ''
    chat.clearSearch()
  } else {
    void nextTick(() => searchInputEl.value?.focus())
  }
}

function onSearchInput() {
  window.clearTimeout(searchDebounce)
  searchDebounce = window.setTimeout(() => {
    void chat.searchMessages(searchInput.value)
  }, 300)
}

async function jumpToSearchResult(m: ChatMessage) {
  skipAutoScroll.value = true
  try {
    await chat.jumpToMessage(m)
    await nextTick()
    const el = messageListEl.value?.querySelector(
      `[data-client-msg-id="${CSS.escape(m.client_msg_id)}"]`,
    ) as HTMLElement | null
    el?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  } finally {
    window.setTimeout(() => {
      skipAutoScroll.value = false
    }, 400)
  }
}

function senderLabel(m: ChatMessage) {
  if (sameId(m.from_user_id, auth.user?.id)) return '我'
  const friend = chat.friends.find((f) => sameId(f.id, m.from_user_id))
  if (friend) return friendDisplayName(friend)
  if (chat.activeGroupId) {
    const member = chat.activeGroupMembers.find((x) => sameId(x.id, m.from_user_id))
    if (member) return chat.mentionName(member)
  }
  return '对方'
}

function messageScrollSignature(list?: ChatMessage[]) {
  if (!list?.length) return '0'
  const last = list[list.length - 1]
  return `${list.length}:${last.client_msg_id}:${last.content}:${last.msg_type}:${last.status === 'failed' ? 'f' : 'ok'}`
}

watch(
  () => messageScrollSignature(chat.messages[chat.activeConvId || '']),
  (sig, prevSig) => {
    if (skipAutoScroll.value || !sig || sig === prevSig) return
    const list = chat.messages[chat.activeConvId || ''] || []
    const last = list[list.length - 1]
    const ownMsg = !!(last && sameId(last.from_user_id, auth.user?.id))
    const nextLen = Number(sig.split(':')[0] || 0)
    const prevLen = Number((prevSig || '0').split(':')[0] || 0)
    if (nearBottom.value || ownMsg) {
      void scrollToBottom(true, ownMsg)
      return
    }
    if (nextLen > prevLen) {
      pendingNewCount.value += nextLen - prevLen
    }
  },
)

watch(
  () => chat.activeConvId,
  (convId, prev) => {
    if (prev) saveDraft(prev, input.value)
    showSearch.value = false
    searchInput.value = ''
    mentionOpen.value = false
    showEmoji.value = false
    preciseTimeIds.value = new Set()
    pendingNewCount.value = 0
    nearBottom.value = true
    selectMode.value = false
    selectedMsgIds.value = new Set()
    replyTarget.value = null
    if (!convId) unreadDividerAfterSeq.value = 0
    input.value = convId ? readDraft(convId) : ''
    if (convId) {
      void scrollToLatestOnOpen()
    }
  },
)
</script>

<template>
  <div class="app-shell" :class="{ 'chat-open': !!chat.activeConvId && !mobileSidebarOpen }">
    <Transition name="fade">
      <div
        v-if="mobileSidebarOpen && chat.activeConvId"
        class="sidebar-backdrop"
        aria-hidden="true"
        @click="mobileSidebarOpen = false"
      />
    </Transition>
    <aside class="sidebar" :class="{ open: mobileSidebarOpen }">
      <header class="sidebar-header">
        <button type="button" class="user-info user-info-btn" @click="goProfile()">
          <UserAvatar :src="selfAvatar" :name="selfName" :size="40" />
          <strong class="user-name">{{ selfName }}</strong>
        </button>
        <button type="button" class="btn btn-ghost btn-sm" aria-label="设置" @click="router.push('/settings')">设置</button>
        <button type="button" class="btn btn-ghost btn-sm" aria-label="退出登录" @click="logout">退出</button>
      </header>

      <Transition name="stack">
        <div v-if="chat.error" key="error" class="alert alert-error sidebar-alert" role="alert">
          <span>{{ chat.error }}</span>
          <button type="button" class="alert-dismiss" aria-label="关闭" @click="chat.clearError()">×</button>
        </div>
      </Transition>

      <Transition name="stack">
        <div v-if="chat.pending.length" class="pending-box" key="pending">
          <p class="section-label">好友申请</p>
          <TransitionGroup name="pending-item" tag="div" class="pending-list">
            <div v-for="p in chat.pending" :key="p.id" class="pending-item">
              <UserAvatar :src="avatarUrl(p.avatar)" :name="p.display_name" :size="32" />
              <span>{{ p.display_name }}</span>
              <div class="pending-actions">
                <button type="button" class="btn btn-primary btn-sm" @click="acceptFriend(p.id)">接受</button>
                <button type="button" class="btn btn-secondary btn-sm" @click="rejectFriend(p.id)">拒绝</button>
              </div>
            </div>
          </TransitionGroup>
        </div>
      </Transition>

      <Transition name="stack">
        <div v-if="chat.groupInvitations.length" class="pending-box" key="group-invites">
          <p class="section-label">群聊邀请</p>
          <TransitionGroup name="pending-item" tag="div" class="pending-list">
            <div v-for="inv in chat.groupInvitations" :key="inv.id" class="pending-item">
              <UserAvatar :src="avatarUrl(inv.from_avatar)" :name="inv.from_name" :size="32" />
              <div class="pending-meta">
                <span>{{ inv.group_name }}</span>
                <span class="pending-sub">群号 {{ inv.group_no }} · 来自 {{ inv.from_name }}</span>
              </div>
              <div class="pending-actions">
                <button type="button" class="btn btn-primary btn-sm" @click="acceptGroupInvite(inv.id)">接受</button>
                <button type="button" class="btn btn-secondary btn-sm" @click="rejectGroupInvite(inv.id)">拒绝</button>
              </div>
            </div>
          </TransitionGroup>
        </div>
      </Transition>

      <div class="sidebar-search">
        <input
          v-model="filterQuery"
          class="input"
          type="search"
          placeholder="搜索好友或群聊"
          aria-label="搜索会话"
        />
      </div>

      <div class="sidebar-tabs">
        <button
          type="button"
          class="sidebar-tab"
          :class="{ active: chat.sidebarTab === 'friends' }"
          @click="chat.sidebarTab = 'friends'"
        >
          好友
          <span v-if="chat.friendsUnread" class="tab-badge">{{ chat.friendsUnread > 99 ? '99+' : chat.friendsUnread }}</span>
        </button>
        <button
          type="button"
          class="sidebar-tab"
          :class="{ active: chat.sidebarTab === 'groups' }"
          @click="chat.sidebarTab = 'groups'"
        >
          群聊
          <span v-if="chat.groupsUnread" class="tab-badge">{{ chat.groupsUnread > 99 ? '99+' : chat.groupsUnread }}</span>
        </button>
        <button
          v-if="chat.friendsUnread + chat.groupsUnread > 0"
          type="button"
          class="btn btn-ghost btn-sm mark-all-btn"
          title="全部标为已读"
          @click="chat.markAllRead()"
        >
          全读
        </button>
      </div>

      <div class="sidebar-list-pane" role="navigation" aria-label="会话列表">
      <Transition name="tab-fade" mode="out-in">
      <ul
        v-if="chat.sidebarTab === 'friends'"
        :key="'friends'"
        class="friend-list"
        role="list"
      >
        <li v-if="sidebarLoading" class="skeleton-list" aria-busy="true" aria-label="加载中">
          <div v-for="n in 5" :key="n" class="skeleton-row">
            <div class="skeleton-avatar" />
            <div class="skeleton-lines">
              <div class="skeleton-line w60" />
              <div class="skeleton-line w40" />
            </div>
          </div>
        </li>
        <li v-else-if="!filteredFriends.length" class="empty-hint">
          <template v-if="filterText">无匹配好友</template>
          <template v-else>
            当前没有好友，
            <button type="button" class="link-btn" @click="openAddModal">添加好友</button>
          </template>
        </li>
        <li
          v-for="f in filteredFriends"
          :key="f.id"
          role="listitem"
          class="friend-item"
          :class="{
            active: sameId(chat.activeToUser, f.id),
            pinned: chat.isPinnedFriend(f.id),
            'has-unread': !!chat.unreadForFriend(f.id) && !chat.isMuted(directConvId(auth.user?.id || '', f.id)),
            'is-agent': isAgentProfile(f),
          }"
          @click="openFriend(f)"
          @contextmenu="openFriendCtx(f, $event)"
          @touchstart.passive="onFriendTouchStart(f, $event)"
          @touchmove.passive="onListTouchMove"
          @touchend.passive="onListTouchEnd"
          @touchcancel.passive="onListTouchEnd"
        >
          <button type="button" class="avatar-btn" @click.stop="goProfile(f.id)">
            <UserAvatar :src="friendAvatarUrl(f)" :name="friendDisplayName(f)" :size="40" />
          </button>
          <div class="friend-meta">
            <div class="friend-row-top">
              <span class="friend-name">{{ friendDisplayName(f) }}</span>
              <span v-if="isAgentProfile(f)" class="agent-badge" title="AI 助手">龟</span>
              <time v-if="f.updatedAt" class="list-time">{{ formatListTime(f.updatedAt) }}</time>
            </div>
            <span v-if="draftPreview(directConvId(auth.user?.id || '', f.id))" class="friend-preview draft">
              <span class="draft-tag">草稿</span>{{ draftPreview(directConvId(auth.user?.id || '', f.id)) }}
            </span>
            <span v-else-if="f.lastPreview" class="friend-preview">{{ f.lastPreview }}</span>
          </div>
          <span
            v-if="chat.isMuted(directConvId(auth.user?.id || '', f.id))"
            class="mute-dot"
            title="已免打扰"
            aria-label="已免打扰"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
              <path d="M6 8a6 6 0 0112 0c0 7 3 7 3 7H3s3 0 3-7" stroke-linecap="round" />
              <path d="M10 19a2 2 0 004 0M3 3l18 18" stroke-linecap="round" />
            </svg>
          </span>
          <span
            v-else-if="chat.unreadForFriend(f.id)"
            class="badge"
          >{{ chat.unreadForFriend(f.id) }}</span>
          <button
            type="button"
            class="pin-btn"
            :class="{ pinned: chat.isPinnedFriend(f.id) }"
            :aria-label="chat.isPinnedFriend(f.id) ? '取消置顶' : '置顶'"
            @click="pinFriend(f.id, $event)"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path d="M14.5 3.5l6 6-2.1 2.1-1.4-1.4-3.3 7.4-1.8-.8 1.5-5.3-2.5-2.5-1.4 1.4L7 8.5l6-6 1.5 1z" />
            </svg>
          </button>
          <button type="button" class="btn btn-ghost btn-sm del-btn" aria-label="删除好友" @click="deleteFriend(f.id, $event)">×</button>
        </li>
      </ul>

      <ul v-else :key="'groups'" class="friend-list" role="list">
        <li v-if="sidebarLoading" class="skeleton-list" aria-busy="true" aria-label="加载中">
          <div v-for="n in 4" :key="n" class="skeleton-row">
            <div class="skeleton-avatar" />
            <div class="skeleton-lines">
              <div class="skeleton-line w60" />
              <div class="skeleton-line w40" />
            </div>
          </div>
        </li>
        <li v-else-if="!filteredGroups.length" class="empty-hint">
          <template v-if="filterText">无匹配群聊</template>
          <template v-else>
            暂无群聊，
            <button type="button" class="link-btn" @click="openAddModal">创建或加入</button>
          </template>
        </li>
        <li
          v-for="g in filteredGroups"
          :key="g.id"
          role="listitem"
          class="friend-item"
          :class="{
            active: chat.activeGroupId === g.id,
            pinned: chat.isPinnedGroup(g.id),
            'has-unread': !!chat.unreadForGroup(g.conversation_id) && !chat.isMuted(g.conversation_id),
          }"
          @click="openGroupChat(g)"
          @contextmenu="openGroupCtx(g, $event)"
          @touchstart.passive="onGroupTouchStart(g, $event)"
          @touchmove.passive="onListTouchMove"
          @touchend.passive="onListTouchEnd"
          @touchcancel.passive="onListTouchEnd"
        >
          <UserAvatar name="群" :size="40" />
          <div class="friend-meta">
            <div class="friend-row-top">
              <span class="friend-name">{{ g.name }}</span>
              <time v-if="g.updatedAt" class="list-time">{{ formatListTime(g.updatedAt) }}</time>
            </div>
            <span v-if="draftPreview(g.conversation_id)" class="friend-preview draft">
              <span class="draft-tag">草稿</span>{{ draftPreview(g.conversation_id) }}
            </span>
            <span v-else-if="g.lastPreview" class="friend-preview">{{ g.lastPreview }}</span>
          </div>
          <span
            v-if="chat.isMuted(g.conversation_id)"
            class="mute-dot"
            title="已免打扰"
            aria-label="已免打扰"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
              <path d="M6 8a6 6 0 0112 0c0 7 3 7 3 7H3s3 0 3-7" stroke-linecap="round" />
              <path d="M10 19a2 2 0 004 0M3 3l18 18" stroke-linecap="round" />
            </svg>
          </span>
          <span
            v-else-if="chat.unreadForGroup(g.conversation_id)"
            class="badge"
          >{{ chat.unreadForGroup(g.conversation_id) }}</span>
          <button
            type="button"
            class="pin-btn"
            :class="{ pinned: chat.isPinnedGroup(g.id) }"
            :aria-label="chat.isPinnedGroup(g.id) ? '取消置顶' : '置顶'"
            @click="pinGroup(g.id, $event)"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path d="M14.5 3.5l6 6-2.1 2.1-1.4-1.4-3.3 7.4-1.8-.8 1.5-5.3-2.5-2.5-1.4 1.4L7 8.5l6-6 1.5 1z" />
            </svg>
          </button>
        </li>
      </ul>
      </Transition>
      </div>

      <footer class="sidebar-footer">
        <button type="button" class="add-contact-btn" @click="openAddModal">
          <span class="add-icon">+</span>
          <span>添加好友 / 群聊</span>
        </button>
      </footer>
    </aside>

    <main class="chat-main">
      <header class="chat-header">
        <button
          type="button"
          class="btn btn-ghost btn-sm mobile-menu-btn"
          aria-label="打开会话列表"
          @click="showMobileSidebar"
        >
          ☰
        </button>
        <button
          v-if="chat.activeConvId"
          type="button"
          class="chat-title-btn"
          @click="openChatHeader"
        >
          <h2 class="chat-title">{{ activeTitle }}</h2>
        </button>
        <h2 v-else class="chat-title">{{ activeTitle }}</h2>
        <div class="chat-header-right">
          <button
            v-if="chat.activeConvId"
            type="button"
            class="btn btn-ghost btn-sm"
            aria-label="清空本地消息"
            title="清空本地消息缓存"
            @click="clearLocalChat"
          >
            清空
          </button>
          <button
            v-if="chat.activeConvId"
            type="button"
            class="btn btn-ghost btn-sm"
            :aria-pressed="selectMode"
            aria-label="多选消息"
            @click="toggleSelectMode"
          >
            {{ selectMode ? '取消多选' : '多选' }}
          </button>
          <button
            v-if="chat.activeConvId"
            type="button"
            class="btn btn-ghost btn-sm"
            :aria-pressed="chat.isActiveMuted"
            :aria-label="chat.isActiveMuted ? '取消免打扰' : '免打扰'"
            @click="chat.toggleActiveMute()"
          >
            {{ chat.isActiveMuted ? '已静音' : '免打扰' }}
          </button>
          <button
            v-if="chat.activeConvId"
            type="button"
            class="btn btn-ghost btn-sm"
            :aria-pressed="showSearch"
            aria-label="搜索消息"
            @click="toggleSearch"
          >
            搜索
          </button>
          <span v-if="chat.activeConvId" class="chat-status" :class="wsStatusClass">{{ wsStatusLabel }}</span>
        </div>
      </header>

      <Transition name="panel-slide">
        <div v-if="chat.activeConvId && showSearch" class="msg-search-panel">
          <div class="msg-search-bar">
            <input
              ref="searchInputEl"
              v-model="searchInput"
              type="search"
              class="msg-search-input"
              placeholder="搜索本会话消息…"
              aria-label="搜索本会话消息"
              @input="onSearchInput"
            />
            <button type="button" class="btn btn-ghost btn-sm" @click="toggleSearch">关闭</button>
          </div>
          <div v-if="chat.searchLoading" class="msg-search-hint">搜索中…</div>
          <div v-else-if="searchInput.trim() && !chat.searchResults.length" class="msg-search-hint">无匹配消息</div>
          <ul v-else-if="chat.searchResults.length" class="msg-search-list">
            <li v-for="m in chat.searchResults" :key="m.client_msg_id">
              <button type="button" class="msg-search-item" @click="jumpToSearchResult(m)">
                <span class="msg-search-meta">{{ senderLabel(m) }} · {{ formatMessageTime(m.created_at) }}</span>
                <span class="msg-search-snippet">{{ messageBody(m) || m.content }}</span>
              </button>
            </li>
          </ul>
        </div>
      </Transition>

      <Transition name="panel-slide">
        <div v-if="showReconnectBanner" class="reconnect-banner" role="status">
          <span class="reconnect-banner-main">
            <svg class="reconnect-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
              <path d="M4 12a8 8 0 0114.9-4M20 12a8 8 0 01-14.9 4" stroke-linecap="round" />
              <path d="M19 4v4h-4M5 20v-4h4" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
            <span>{{ reconnectBannerText }}</span>
          </span>
          <button
            v-if="chat.wsStatus !== 'open'"
            type="button"
            class="btn btn-secondary btn-sm"
            @click="chat.forceReconnect()"
          >
            立即重连
          </button>
        </div>
      </Transition>

      <Transition name="panel-slide">
        <div
          v-if="chat.activeGroupId && chat.activeGroupNotice"
          class="group-notice-bar"
          role="status"
        >
          <span class="group-notice-label">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
              <path d="M3 11v2a1 1 0 001 1h2l5 4V6L6 10H4a1 1 0 00-1 1z" stroke-linejoin="round" />
              <path d="M16 8.5a4.5 4.5 0 010 7M18.5 6a8 8 0 010 12" stroke-linecap="round" />
            </svg>
            公告
          </span>
          <span class="group-notice-text">{{ chat.activeGroupNotice }}</span>
        </div>
      </Transition>

      <div v-if="!chat.activeConvId" class="empty-chat">
        <div class="empty-icon" aria-hidden="true">
          <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path
              d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
        </div>
        <p class="empty-title">
          {{
            sidebarLoading
              ? '正在加载会话…'
              : settings.layoutMode === 'detached'
                ? '点击左侧会话将打开独立聊天窗口'
                : '选择好友开始聊天'
          }}
        </p>
        <p v-if="sidebarLoading" class="empty-desc">请稍候</p>
        <p v-else-if="!chat.friends.length" class="empty-desc">
          当前没有好友，
          <button type="button" class="link-btn" @click="openAddModal">添加好友</button>
        </p>
        <p v-else class="empty-desc">
          {{
            settings.layoutMode === 'detached'
              ? '也可在设置中改回经典单窗口；右键会话可「独立窗口打开」'
              : '从左侧选择一位好友或群聊'
          }}
        </p>
      </div>

      <template v-else>
        <div class="message-pane">
        <div
          ref="messageListEl"
          class="message-list"
          role="log"
          aria-live="polite"
          @scroll.passive="onMessageListScroll"
        >
          <button
            v-if="chat.historyHasMore[chat.activeConvId]"
            type="button"
            class="load-more-btn"
            :disabled="historyLoading"
            @click="loadOlder"
          >
            {{ historyLoading ? '加载中…' : '加载更早消息' }}
          </button>
          <div v-if="historyLoading && !(chat.messages[chat.activeConvId] || []).length" class="msg-skeleton" aria-busy="true">
            <div v-for="n in 4" :key="n" class="msg-skeleton-row" :class="{ me: n % 2 === 0 }">
              <div class="msg-skeleton-bubble" />
            </div>
          </div>
          <div
            v-else-if="!(chat.messages[chat.activeConvId] || []).length"
            class="local-empty"
          >
            <p>本地暂无消息</p>
            <button type="button" class="btn btn-secondary btn-sm" @click="reloadHistory">重新加载历史</button>
          </div>
          <template v-for="item in displayItems" :key="item.key">
            <div
              v-if="item.kind === 'divider'"
              class="date-divider"
              :class="{ unread: item.variant === 'unread' }"
            >
              <span>{{ item.label }}</span>
            </div>
            <div
              v-else
              class="message-row"
              :data-client-msg-id="item.message.client_msg_id"
              :class="{
                me: sameId(item.message.from_user_id, auth.user?.id),
                highlight: chat.highlightClientMsgId === item.message.client_msg_id,
                selected: selectMode && isMsgSelected(item.message.client_msg_id),
                'select-mode': selectMode,
                sending: item.message.status === 'sending' || item.message.status === 'uploading',
                failed: item.message.status === 'failed',
                [`cluster-${item.cluster}`]: true,
                'is-swiping': swipeMsgId === item.message.client_msg_id && swipeOffset > 0,
              }"
              @click="selectMode && toggleSelectMsg(item.message.client_msg_id)"
            >
              <div
                v-if="swipeMsgId === item.message.client_msg_id && swipeOffset > 0"
                class="swipe-reply-hint"
                :style="{ opacity: Math.min(1, swipeOffset / SWIPE_REPLY_THRESHOLD) }"
                aria-hidden="true"
              >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M9 14L4 9l5-5" stroke-linecap="round" stroke-linejoin="round" />
                  <path d="M20 20v-7a4 4 0 00-4-4H4" stroke-linecap="round" />
                </svg>
              </div>
              <label v-if="selectMode" class="msg-check" @click.stop>
                <input
                  type="checkbox"
                  :checked="isMsgSelected(item.message.client_msg_id)"
                  @change="toggleSelectMsg(item.message.client_msg_id)"
                />
              </label>
              <div
                class="bubble"
                :style="swipeMsgId === item.message.client_msg_id && swipeOffset > 0
                  ? { transform: `translateX(${swipeOffset}px)` }
                  : undefined"
                @dblclick="!selectMode && onBubbleDblClick(item.message)"
                @contextmenu="onBubbleContextMenu(item.message, $event)"
                @touchstart.passive="onBubbleTouchStart(item.message, $event)"
                @touchmove.passive="onBubbleTouchMove($event)"
                @touchend.passive="onBubbleTouchEnd"
                @touchcancel.passive="onBubbleTouchEnd"
              >
                <div
                  v-if="item.showSender"
                  class="msg-sender"
                >
                  {{ senderLabel(item.message) }}
                </div>
                <div
                  v-if="!selectMode && (isTextMessage(item.message) || canRecall(item.message) || canReply(item.message))"
                  class="msg-actions msg-actions-desktop"
                >
                  <button
                    v-if="canReply(item.message)"
                    type="button"
                    class="msg-action-btn"
                    @click="startReply(item.message)"
                  >回复</button>
                  <button
                    v-if="isTextMessage(item.message)"
                    type="button"
                    class="msg-action-btn"
                    @click="copyText(messageBody(item.message))"
                  >复制</button>
                  <button
                    v-if="canRecall(item.message)"
                    type="button"
                    class="msg-action-btn"
                    @click="recallMsg(item.message)"
                  >撤回</button>
                </div>
                <template v-if="parseFileContent(item.message.content) || item.message.localPreview">
                  <div
                    v-if="item.message.msg_type === 2 || item.message.localPreview || parseFileContent(item.message.content)?.content_type?.startsWith('image/')"
                    class="msg-image-wrap"
                  >
                    <img
                      :src="item.message.localPreview || fileUrl(parseFileContent(item.message.content)!.url)"
                      class="msg-image clickable"
                      alt="图片"
                      loading="lazy"
                      decoding="async"
                      @load="($event.target as HTMLImageElement).classList.add('loaded')"
                      @click="openImagePreview(item.message.localPreview || parseFileContent(item.message.content)!.url)"
                    />
                    <div
                      v-if="item.message.status === 'uploading'"
                      class="upload-overlay"
                    >
                      <div class="upload-bar">
                        <div class="upload-bar-fill" :style="{ width: `${item.message.uploadProgress || 0}%` }" />
                      </div>
                      <span class="upload-pct">{{ item.message.uploadProgress || 0 }}%</span>
                    </div>
                  </div>
                  <a
                    v-else
                    :href="fileUrl(parseFileContent(item.message.content)!.url)"
                    target="_blank"
                    rel="noopener"
                    class="file-link"
                  >
                    <svg class="file-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
                      <path d="M14 2H7a2 2 0 00-2 2v16a2 2 0 002 2h10a2 2 0 002-2V8l-5-6z" stroke-linejoin="round" />
                      <path d="M14 2v6h6" stroke-linejoin="round" />
                    </svg>
                    <span class="file-name">{{ parseFileContent(item.message.content)!.filename }}</span>
                    <span
                      v-if="parseFileContent(item.message.content)!.size"
                      class="file-size"
                    >{{ formatFileSize(parseFileContent(item.message.content)!.size) }}</span>
                    <span v-if="item.message.status === 'uploading'" class="upload-file-pct">
                      上传中 {{ item.message.uploadProgress || 0 }}%
                    </span>
                  </a>
                </template>
                <template v-else>
                  <span
                    v-if="item.message.msg_type === 4"
                    class="recalled"
                  >{{ item.message.content }}</span>
                  <template v-else>
                    <div
                      v-if="messageReply(item.message)"
                      class="msg-quote"
                      role="button"
                      tabindex="0"
                      title="查看原消息"
                      @click.stop="jumpToQuoted(item.message)"
                      @keydown.enter.prevent="jumpToQuoted(item.message)"
                    >
                      <div class="msg-quote-name">{{ messageReply(item.message)!.n }}</div>
                      <div class="msg-quote-preview">{{ messageReply(item.message)!.p }}</div>
                    </div>
                    <span
                      class="msg-text"
                      v-html="renderMessageHtml(item.message.content)"
                    />
                  </template>
                </template>
                <time
                  v-if="item.showTime && item.message.created_at"
                  class="msg-time"
                  :title="preciseTimeIds.has(item.message.client_msg_id) ? '点击隐藏秒' : '点击显示秒'"
                  @click.stop="togglePreciseTime(item.message.client_msg_id)"
                >{{ formatMessageTime(item.message.created_at, preciseTimeIds.has(item.message.client_msg_id)) }}</time>
                <span
                  v-if="sameId(item.message.from_user_id, auth.user?.id) && (item.message.status === 'sending' || item.message.status === 'uploading')"
                  class="read-tag pending"
                  title="发送中"
                  aria-label="发送中"
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                    <circle cx="12" cy="12" r="9" />
                    <path d="M12 7v5l3 2" stroke-linecap="round" stroke-linejoin="round" />
                  </svg>
                </span>
                <span
                  v-else-if="item.showTime && sameId(item.message.from_user_id, auth.user?.id) && item.message.seq && item.message.seq <= chat.activePeerReadSeq"
                  class="read-tag"
                  title="已读"
                  aria-label="已读"
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true">
                    <path d="M2.5 12.5l4 4L14 8" stroke-linecap="round" stroke-linejoin="round" />
                    <path d="M8.5 12.5l4 4L20.5 7.5" stroke-linecap="round" stroke-linejoin="round" />
                  </svg>
                </span>
                <span
                  v-else-if="item.showTime && sameId(item.message.from_user_id, auth.user?.id) && item.message.status !== 'failed'"
                  class="read-tag sent"
                  title="已发送"
                  aria-label="已发送"
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true">
                    <path d="M4 12.5l5 5L20 7" stroke-linecap="round" stroke-linejoin="round" />
                  </svg>
                </span>
                <button
                  v-if="sameId(item.message.from_user_id, auth.user?.id) && item.message.status === 'failed'"
                  type="button"
                  class="retry-btn"
                  title="发送失败，点击重试"
                  aria-label="发送失败，点击重试"
                  @click="chat.retryMessage(item.message.client_msg_id)"
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                    <circle cx="12" cy="12" r="9" />
                    <path d="M12 8v5M12 16h.01" stroke-linecap="round" />
                  </svg>
                  <span>重试</span>
                </button>
              </div>
            </div>
          </template>
        </div>
        <Transition name="fab">
          <button
            v-if="!nearBottom"
            type="button"
            class="jump-latest-btn"
            @click="jumpToLatest"
          >
            {{ pendingNewCount > 0 ? `${pendingNewCount > 99 ? '99+' : pendingNewCount} 条新消息` : '回到底部' }}
          </button>
        </Transition>
        </div>
        <div class="composer-stack">
          <Transition name="stack">
            <div v-if="selectMode" class="select-bar" role="toolbar" aria-label="多选操作">
              <span>已选 {{ selectedCount }} 条</span>
              <div class="select-bar-actions">
                <button
                  type="button"
                  class="btn btn-primary btn-sm"
                  :disabled="!selectedCount"
                  @click="copySelectedMessages"
                >
                  复制转发文本
                </button>
                <button type="button" class="btn btn-secondary btn-sm" @click="toggleSelectMode">完成</button>
              </div>
            </div>
          </Transition>
          <Transition name="stack">
            <div v-if="chat.activeTypingLabel" class="typing-indicator" aria-live="polite">
              <span class="typing-label">{{ chat.activeTypingLabel }}</span>
              <span class="typing-dots" aria-hidden="true">
                <i /><i /><i />
              </span>
            </div>
          </Transition>
          <Transition name="stack">
            <div v-if="mentionOpen && mentionCandidates.length" class="mention-panel" role="listbox">
              <button
                v-for="m in mentionCandidates"
                :key="m.id"
                type="button"
                class="mention-item"
                role="option"
                @mousedown.prevent="insertMention(m)"
              >
                <UserAvatar :src="avatarUrl(m.avatar)" :name="chat.mentionName(m)" :size="28" />
                <span class="mention-name">{{ chat.mentionName(m) }}</span>
                <span class="mention-user">@{{ m.username }}</span>
              </button>
            </div>
          </Transition>
          <Transition name="stack">
            <div v-if="chat.uploading && !hasVisibleUploadBubble" class="upload-banner" role="status">
              <svg class="upload-banner-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                <path d="M12 16V4M8 8l4-4 4 4" stroke-linecap="round" stroke-linejoin="round" />
                <path d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2" stroke-linecap="round" />
              </svg>
              <div class="upload-banner-copy">
                <span>正在上传… {{ chat.uploadPercent }}%</span>
                <div class="upload-banner-bar">
                  <div class="upload-banner-fill" :style="{ width: `${chat.uploadPercent}%` }" />
                </div>
              </div>
            </div>
          </Transition>
          <Transition name="stack">
            <div v-if="showEmoji" class="emoji-panel" role="dialog" aria-label="表情">
              <button
                v-for="e in EMOJIS"
                :key="e"
                type="button"
                class="emoji-btn"
                @click="insertEmoji(e)"
              >{{ e }}</button>
            </div>
          </Transition>
          <Transition name="stack">
            <div v-if="replyTarget" class="reply-bar" role="status">
              <span class="reply-bar-accent" aria-hidden="true" />
              <svg class="reply-bar-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                <path d="M9 14L4 9l5-5" stroke-linecap="round" stroke-linejoin="round" />
                <path d="M20 20v-7a4 4 0 00-4-4H4" stroke-linecap="round" />
              </svg>
              <div class="reply-bar-body">
                <span class="reply-bar-label">回复 {{ replyTarget.n }}</span>
                <span class="reply-bar-preview">{{ replyTarget.p }}</span>
              </div>
              <button type="button" class="reply-bar-cancel" aria-label="取消回复" @click="cancelReply">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" aria-hidden="true">
                  <path d="M6 6l12 12M18 6L6 18" stroke-linecap="round" />
                </svg>
              </button>
            </div>
          </Transition>
        </div>
        <footer class="composer">
          <input ref="fileInput" type="file" class="hidden-file" @change="onFileChange" />
          <button
            type="button"
            class="btn btn-secondary btn-sm composer-icon-btn"
            aria-label="表情"
            :aria-pressed="showEmoji"
            @click="toggleEmoji"
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
              <circle cx="12" cy="12" r="9" />
              <path d="M8.5 10.5h.01M15.5 10.5h.01" stroke-linecap="round" />
              <path d="M8.2 14.2c1.1 1.2 2.4 1.8 3.8 1.8s2.7-.6 3.8-1.8" stroke-linecap="round" />
            </svg>
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm composer-icon-btn"
            aria-label="发送文件"
            :disabled="chat.uploading"
            @click="pickFile"
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
              <path d="M21.4 11.6l-8.5 8.5a5 5 0 01-7.1-7.1l8.5-8.5a3.2 3.2 0 014.5 4.5L10.2 17.6a1.4 1.4 0 01-2-2l7.4-7.4" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
          </button>
          <textarea
            ref="composerInputEl"
            v-model="input"
            class="input composer-input"
            rows="1"
            :placeholder="chat.activeGroupId ? '发消息… Enter 发送，Shift+Enter 换行' : '发消息… Enter 发送，Shift+Enter 换行'"
            aria-label="消息输入框"
            @input="onComposerInput"
            @keyup="updateMentionState"
            @click="updateMentionState"
            @keydown="onComposerKeydown"
            @paste="onComposerPaste"
          />
          <button
            type="button"
            class="btn btn-primary send-btn"
            :class="{ sending: sendingLock }"
            :disabled="!input.trim() || chat.uploading || sendingLock"
            :aria-busy="sendingLock"
            @click="send"
          >
            <svg
              v-if="sendingLock"
              class="send-spin"
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2.4"
              aria-hidden="true"
            >
              <circle cx="12" cy="12" r="9" opacity="0.25" />
              <path d="M21 12a9 9 0 00-9-9" stroke-linecap="round" />
            </svg>
            <span>{{ sendingLock ? '发送中' : '发送' }}</span>
          </button>
        </footer>
      </template>
    </main>

    <Transition name="fade">
      <AddContactModal v-if="showAddModal" @close="showAddModal = false" />
    </Transition>
    <Transition name="fade">
      <ImageLightbox v-if="previewImage" :src="previewImage" @close="previewImage = ''" />
    </Transition>

    <Transition name="fade-scale">
      <div
        v-if="msgMenu"
        class="ctx-menu msg-ctx-menu"
        :style="{ left: `${msgMenu.x}px`, top: `${msgMenu.y}px` }"
        role="menu"
        @click.stop
        @contextmenu.prevent
      >
        <button
          v-if="canReply(msgMenu.message)"
          type="button"
          class="ctx-item"
          role="menuitem"
          @click="msgMenuReply()"
        >回复</button>
        <button
          v-if="isTextMessage(msgMenu.message)"
          type="button"
          class="ctx-item"
          role="menuitem"
          @click="msgMenuCopy()"
        >复制</button>
        <button
          v-if="canRecall(msgMenu.message)"
          type="button"
          class="ctx-item"
          role="menuitem"
          @click="msgMenuRecall()"
        >撤回</button>
        <button type="button" class="ctx-item" role="menuitem" @click="msgMenuSelect()">多选</button>
      </div>
    </Transition>

    <Transition name="fade-scale">
      <div
        v-if="ctxMenu"
        class="ctx-menu"
        :style="{ left: `${ctxMenu.x}px`, top: `${ctxMenu.y}px` }"
        role="menu"
        @click.stop
        @contextmenu.prevent
      >
      <button type="button" class="ctx-item" role="menuitem" @click="ctxOpenChat">打开会话</button>
      <button type="button" class="ctx-item" role="menuitem" @click="ctxOpenDetached">独立窗口打开</button>
      <button type="button" class="ctx-item" role="menuitem" @click="ctxTogglePin">
        <template v-if="ctxMenu.kind === 'friend'">
          {{ chat.isPinnedFriend(ctxMenu.id) ? '取消置顶' : '置顶' }}
        </template>
        <template v-else>
          {{ chat.isPinnedGroup(ctxMenu.id) ? '取消置顶' : '置顶' }}
        </template>
      </button>
      <button type="button" class="ctx-item" role="menuitem" @click="ctxToggleMute">
        <template v-if="ctxMenu.kind === 'friend'">
          {{ chat.isMuted(directConvId(auth.user?.id || '', ctxMenu.id)) ? '取消免打扰' : '免打扰' }}
        </template>
        <template v-else>
          {{
            chat.isMuted(ctxGroup()?.conversation_id || '')
              ? '取消免打扰'
              : '免打扰'
          }}
        </template>
      </button>
      <button type="button" class="ctx-item" role="menuitem" @click="ctxOpenProfile">
        {{ ctxMenu.kind === 'friend' ? '查看资料' : '群聊信息' }}
      </button>
      </div>
    </Transition>

    <Transition name="fade">
      <div
        v-if="confirmDialog"
        class="confirm-backdrop"
        role="presentation"
        @click.self="closeConfirm"
      >
        <div class="confirm-card" role="dialog" aria-modal="true" :aria-label="confirmDialog.title">
          <h3 class="confirm-title">{{ confirmDialog.title }}</h3>
          <p class="confirm-body">{{ confirmDialog.body }}</p>
          <div class="confirm-actions">
            <button type="button" class="btn btn-secondary btn-sm" @click="closeConfirm">取消</button>
            <button
              type="button"
              class="btn btn-sm"
              :class="confirmDialog.danger ? 'btn-danger' : 'btn-primary'"
              @click="runConfirm"
            >
              {{ confirmDialog.confirmLabel || '确定' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.app-shell {
  display: grid;
  grid-template-columns: var(--sidebar-width) 1fr;
  height: 100%;
  max-height: 100%;
  overflow: hidden;
  background: var(--color-bg-app);
  padding-top: env(safe-area-inset-top, 0px);
  padding-left: env(safe-area-inset-left, 0px);
  padding-right: env(safe-area-inset-right, 0px);
}

.sidebar {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  overflow: hidden;
  background: var(--color-bg-surface);
  border-right: 1px solid var(--color-border);
  min-width: 0;
}

.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  min-height: var(--header-height);
  flex-shrink: 0;
  padding: 0 var(--space-4);
  border-bottom: 1px solid var(--color-border);
  background: var(--color-bg-sidebar);
}

.user-info {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  min-width: 0;
}

.user-info-btn {
  flex: 1;
  padding: var(--space-1);
  text-align: left;
  border-radius: var(--radius-sm);
}

.user-info-btn:hover {
  background: var(--color-primary-muted);
}

.user-name {
  font-size: var(--text-sm);
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.alert-success {
  color: var(--color-success);
  background: #ecfdf5;
  border: 1px solid #a7f3d0;
}

.sidebar-alert {
  margin: var(--space-3) var(--space-4) 0;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-2);
}

.alert-dismiss {
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  padding: 0;
  font-size: 1.25rem;
  line-height: 1;
  color: inherit;
  opacity: 0.7;
  border-radius: var(--radius-sm);
}

.pending-box {
  margin: var(--space-3) var(--space-4);
  padding: var(--space-3);
  background: var(--color-warning-bg);
  border: 1px solid #fde68a;
  border-radius: var(--radius-md);
}

.section-label {
  margin: 0 0 var(--space-2);
  font-size: var(--text-xs);
  font-weight: 600;
  color: var(--color-text-muted);
}

.pending-item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) 0;
  font-size: var(--text-sm);
}

.pending-actions {
  margin-left: auto;
  display: flex;
  gap: var(--space-1);
}

.pending-meta {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.pending-sub {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}

.sidebar-search {
  flex-shrink: 0;
  padding: var(--space-3) var(--space-4) 0;
}

.sidebar-search .input {
  width: 100%;
}

.date-divider {
  display: flex;
  justify-content: center;
  margin: var(--space-2) 0;
}

/* WhatsApp / 微信：滚动时日期条粘在消息区顶部 */
.date-divider:not(.unread) {
  position: sticky;
  top: 6px;
  z-index: 6;
  pointer-events: none;
}

.date-divider span {
  padding: 3px 12px;
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  background: rgba(255, 255, 255, 0.88);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  border-radius: var(--radius-full);
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.06);
  animation: empty-in var(--transition-base) ease;
}

.date-divider.unread {
  position: relative;
  z-index: 1;
  pointer-events: none;
}

.date-divider.unread span {
  color: var(--color-primary);
  background: var(--color-primary-muted);
  font-weight: 600;
  box-shadow: none;
  backdrop-filter: none;
}

.sidebar-tabs {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  gap: 2px;
  margin: var(--space-3) var(--space-4) 0;
  padding: var(--space-1);
  background: var(--color-bg-sidebar);
  border-radius: var(--radius-md);
}

.mark-all-btn {
  flex-shrink: 0;
  font-size: var(--text-xs);
  color: var(--color-primary);
  white-space: nowrap;
}

.sidebar-tab {
  flex: 1;
  min-height: 36px;
  border-radius: var(--radius-sm);
  font-weight: 600;
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  transition:
    background var(--transition-fast),
    color var(--transition-fast);
}

.tab-badge {
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  font-size: 10px;
  font-weight: 700;
  line-height: 18px;
  color: #fff;
  background: var(--color-danger);
  border-radius: var(--radius-full);
}

.sidebar-tab.active {
  background: var(--color-bg-surface);
  color: var(--color-primary);
  box-shadow: var(--shadow-sm);
}

.sidebar-list-pane {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.friend-list {
  list-style: none;
  margin: 0;
  padding: var(--space-2) 0;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  overscroll-behavior: contain;
  -webkit-overflow-scrolling: touch;
}

.empty-hint {
  padding: var(--space-6) var(--space-4);
  text-align: center;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.skeleton-list {
  list-style: none;
  padding: var(--space-2) var(--space-4);
}

.skeleton-row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) 0;
}

.skeleton-avatar {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-full);
  background: linear-gradient(90deg, #e2e8f0 25%, #f1f5f9 50%, #e2e8f0 75%);
  background-size: 200% 100%;
  animation: skeleton-shimmer 1.2s ease-in-out infinite;
}

.skeleton-lines {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.skeleton-line {
  height: 10px;
  border-radius: var(--radius-sm);
  background: linear-gradient(90deg, #e2e8f0 25%, #f1f5f9 50%, #e2e8f0 75%);
  background-size: 200% 100%;
  animation: skeleton-shimmer 1.2s ease-in-out infinite;
}

.skeleton-line.w60 {
  width: 60%;
}

.skeleton-line.w40 {
  width: 40%;
}

@keyframes skeleton-shimmer {
  0% {
    background-position: 100% 0;
  }
  100% {
    background-position: -100% 0;
  }
}

.msg-skeleton {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-4) 0;
}

.msg-skeleton-row {
  display: flex;
}

.msg-skeleton-row.me {
  justify-content: flex-end;
}

.msg-skeleton-bubble {
  width: 42%;
  height: 36px;
  border-radius: var(--radius-md);
  background: linear-gradient(90deg, #e2e8f0 25%, #f1f5f9 50%, #e2e8f0 75%);
  background-size: 200% 100%;
  animation: skeleton-shimmer 1.2s ease-in-out infinite;
}

.ctx-menu {
  position: fixed;
  z-index: 1000;
  min-width: 140px;
  padding: 4px 0;
  background: var(--color-bg-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.12);
}

.ctx-item {
  display: block;
  width: 100%;
  padding: 8px 14px;
  text-align: left;
  font-size: var(--text-sm);
  border: 0;
  background: transparent;
  cursor: pointer;
  color: var(--color-text);
}

.ctx-item:hover {
  background: var(--color-primary-muted);
}

.link-btn {
  color: var(--color-primary);
  font-weight: 600;
  text-decoration: underline;
  padding: 0;
}

.friend-item {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  cursor: pointer;
  transition:
    background var(--transition-fast),
    border-color var(--transition-fast);
  border-left: 3px solid transparent;
}

.friend-item:hover {
  background: var(--color-primary-muted);
}

.friend-item:active {
  background: var(--color-primary-soft);
}

.friend-item.active {
  background: var(--color-primary-soft);
  border-left-color: var(--color-primary);
}

.friend-item.has-unread {
  background: rgba(13, 148, 136, 0.06);
}

.friend-item.has-unread.active {
  background: var(--color-primary-soft);
}

.friend-item.is-agent {
  background: linear-gradient(90deg, rgba(13, 148, 136, 0.05), transparent);
}

.agent-badge {
  flex-shrink: 0;
  margin-left: 6px;
  padding: 1px 6px;
  font-size: 10px;
  font-weight: 700;
  line-height: 1.4;
  color: var(--color-primary);
  background: var(--color-primary-muted);
  border-radius: var(--radius-full);
}

.friend-item.has-unread .friend-name {
  font-weight: 700;
  color: var(--color-text-primary);
}

.friend-item.has-unread .friend-preview:not(.draft) {
  color: var(--color-text-secondary);
  font-weight: 500;
}

.friend-item.pinned {
  background: rgba(255, 193, 7, 0.08);
}

.friend-item.pinned.active {
  background: var(--color-primary-soft);
}

.avatar-btn {
  padding: 0;
  border-radius: 50%;
}

.friend-row-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  min-width: 0;
}

.list-time {
  flex-shrink: 0;
  font-size: 10px;
  color: var(--color-text-muted);
}

.read-tag {
  display: inline-flex;
  align-items: center;
  margin-top: 2px;
  margin-left: 2px;
  color: var(--color-primary);
  vertical-align: middle;
  transition: opacity var(--transition-base);
}

.read-tag.sent {
  color: var(--color-text-muted);
}

.read-tag.pending {
  color: var(--color-text-muted);
  opacity: 0.75;
}

.message-row.me .read-tag {
  color: var(--color-primary);
}

.message-row.me .read-tag.sent,
.message-row.me .read-tag.pending {
  color: rgba(0, 0, 0, 0.35);
}

.friend-name {
  font-size: var(--text-sm);
  font-weight: 500;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.friend-meta {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.friend-preview {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.friend-preview.draft {
  color: var(--color-text-muted);
}

.draft-tag {
  margin-right: 4px;
  font-weight: 600;
  color: #b45309;
}

.chat-title-btn {
  padding: 0;
  text-align: left;
  border-radius: var(--radius-sm);
}

.chat-title-btn:hover .chat-title {
  color: var(--color-primary);
}

.msg-time {
  display: block;
  margin-top: var(--space-1);
  min-width: 3.5em;
  font-size: 10px;
  color: var(--color-text-muted);
  text-align: right;
  cursor: pointer;
  user-select: none;
  transition: color var(--transition-fast);
}

.msg-time:hover {
  text-decoration: underline;
}

.message-row.me .msg-time {
  color: rgba(0, 0, 0, 0.45);
}

.badge {
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  font-size: 11px;
  font-weight: 700;
  line-height: 20px;
  text-align: center;
  color: #fff;
  background: var(--color-danger);
  border-radius: var(--radius-full);
  animation: badge-pop var(--transition-base) ease;
}

@keyframes badge-pop {
  from {
    opacity: 0;
    transform: scale(0.7);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

.mute-dot {
  display: inline-flex;
  align-items: center;
  color: var(--color-text-muted);
  opacity: 0.85;
}

.del-btn {
  opacity: 0;
  transition: opacity var(--transition-fast);
}

.pin-btn {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  color: var(--color-text-muted);
  opacity: 0.35;
  border-radius: var(--radius-sm);
  transition: opacity var(--transition-fast), color var(--transition-fast);
}

.pin-btn.pinned {
  color: #d97706;
  opacity: 1;
}

.pin-btn.pinned,
.friend-item:hover .pin-btn {
  opacity: 1;
}

.pin-btn.pinned {
  filter: saturate(1.4);
}

.friend-item:hover .del-btn {
  opacity: 1;
}

.sidebar-footer {
  flex-shrink: 0;
  padding: var(--space-3) var(--space-4);
  border-top: 1px solid var(--color-border);
}

.add-contact-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
  width: 100%;
  min-height: 44px;
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  background: var(--color-primary-muted);
  color: var(--color-primary);
  font-weight: 600;
  font-size: var(--text-sm);
}

.add-contact-btn:hover {
  background: var(--color-primary-soft);
}

.add-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--color-primary);
  color: #fff;
  font-size: 1.1rem;
  line-height: 1;
}

.chat-main {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  height: 100%;
  overflow: hidden;
  background: var(--color-bg-chat);
}

.chat-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  min-height: var(--header-height);
  flex-shrink: 0;
  padding: 0 var(--space-6);
  background: var(--color-bg-surface);
  border-bottom: 1px solid var(--color-border);
}

.chat-header-right {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex-shrink: 0;
}

.msg-search-panel {
  border-bottom: 1px solid var(--color-border);
  background: var(--color-bg-surface);
}

.msg-search-bar {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-6);
}

.msg-search-input {
  flex: 1;
  min-width: 0;
  height: 36px;
  padding: 0 var(--space-3);
  font-size: var(--text-sm);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-bg-chat);
}

.msg-search-hint {
  padding: 0 var(--space-6) var(--space-3);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.msg-search-list {
  list-style: none;
  margin: 0;
  padding: 0 0 var(--space-2);
  max-height: 220px;
  overflow-y: auto;
}

.msg-search-item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  width: 100%;
  padding: var(--space-2) var(--space-6);
  text-align: left;
  border: 0;
  background: transparent;
  cursor: pointer;
}

.msg-search-item:hover {
  background: var(--color-primary-muted);
}

.msg-search-meta {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}

.msg-search-snippet {
  font-size: var(--text-sm);
  color: var(--color-text);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.message-row.highlight .bubble {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
  animation: msg-highlight-pulse 2.4s ease-in-out forwards;
}

@keyframes msg-highlight-pulse {
  0%,
  35% {
    outline-color: var(--color-primary);
    box-shadow: 0 0 0 3px rgba(13, 148, 136, 0.2);
  }
  55% {
    outline-color: var(--color-primary);
    box-shadow: 0 0 0 0 transparent;
  }
  100% {
    outline-color: transparent;
    box-shadow: none;
  }
}

.chat-title {
  margin: 0;
  font-size: var(--text-lg);
  font-weight: 600;
}

.chat-status {
  font-size: var(--text-xs);
  font-weight: 500;
}

.chat-status.online {
  color: var(--color-success);
}

.chat-status.connecting {
  color: #d97706;
}

.chat-status.offline {
  color: var(--color-danger);
}

.chat-status.online::before,
.chat-status.connecting::before,
.chat-status.offline::before {
  content: '';
  display: inline-block;
  width: 6px;
  height: 6px;
  margin-right: 6px;
  border-radius: var(--radius-full);
  vertical-align: middle;
}

.chat-status.online::before {
  background: var(--color-success);
}

.chat-status.connecting::before {
  background: #d97706;
}

.chat-status.offline::before {
  background: var(--color-danger);
}

.reconnect-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-6);
  font-size: var(--text-sm);
  color: #92400e;
  background: #fffbeb;
  border-bottom: 1px solid #fde68a;
}

.reconnect-banner-main {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.reconnect-icon {
  flex-shrink: 0;
  opacity: 0.9;
}

.group-notice-bar {
  display: flex;
  align-items: flex-start;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-6);
  font-size: var(--text-sm);
  color: #0f766e;
  background: #f0fdfa;
  border-bottom: 1px solid #99f6e4;
}

.group-notice-label {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-weight: 700;
  font-size: var(--text-xs);
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  background: var(--color-primary-muted);
}

.group-notice-text {
  flex: 1;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.load-more-btn {
  align-self: center;
  margin-bottom: var(--space-2);
  padding: var(--space-1) var(--space-3);
  font-size: var(--text-xs);
  color: var(--color-primary);
  background: var(--color-primary-muted);
  border-radius: var(--radius-full);
  transition:
    opacity var(--transition-fast),
    transform var(--transition-fast),
    background var(--transition-fast);
}

.load-more-btn:hover:not(:disabled) {
  background: var(--color-primary-soft, var(--color-primary-muted));
}

.load-more-btn:active:not(:disabled) {
  transform: scale(0.97);
}

.load-more-btn:disabled {
  opacity: 0.55;
  cursor: wait;
}

.empty-chat {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--space-8);
  text-align: center;
  animation: empty-in var(--transition-base) ease;
}

@keyframes empty-in {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.empty-icon {
  color: var(--color-text-muted);
  opacity: 0.5;
  margin-bottom: var(--space-4);
}

.empty-title {
  margin: 0 0 var(--space-2);
  font-size: var(--text-lg);
  font-weight: 600;
}

.empty-desc {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.message-pane {
  position: relative;
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.message-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: var(--space-6);
  display: flex;
  flex-direction: column;
  gap: 2px;
  scroll-behavior: auto;
  overscroll-behavior: contain;
  -webkit-overflow-scrolling: touch;
  background-color: var(--color-bg-chat);
  background-image:
    radial-gradient(rgba(15, 23, 42, 0.035) 1px, transparent 1px);
  background-size: 14px 14px;
}

.message-row.cluster-solo,
.message-row.cluster-first {
  margin-top: 10px;
}

.message-row.cluster-middle,
.message-row.cluster-last {
  margin-top: 2px;
}

.message-row.cluster-middle .bubble,
.message-row.cluster-last .bubble {
  box-shadow: none;
}

.message-row.cluster-first:not(.me) .bubble {
  border-bottom-left-radius: var(--space-1);
}

.message-row.cluster-middle:not(.me) .bubble,
.message-row.cluster-last:not(.me) .bubble {
  border-top-left-radius: var(--radius-sm);
  border-bottom-left-radius: var(--space-1);
}

.message-row.cluster-first.me .bubble {
  border-bottom-right-radius: var(--space-1);
}

.message-row.cluster-middle.me .bubble,
.message-row.cluster-last.me .bubble {
  border-top-right-radius: var(--radius-sm);
  border-bottom-right-radius: var(--space-1);
}

.jump-latest-btn {
  position: absolute;
  right: var(--space-6);
  bottom: var(--space-4);
  z-index: 5;
  padding: 8px 14px;
  font-size: var(--text-xs);
  font-weight: 600;
  color: var(--color-text-inverse);
  background: var(--color-primary);
  border: 0;
  border-radius: var(--radius-full);
  box-shadow: 0 4px 12px rgba(13, 148, 136, 0.35);
  cursor: pointer;
}

.jump-latest-btn:hover {
  background: var(--color-primary-hover);
}

.composer-stack {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}

.message-row {
  display: flex;
  align-items: flex-end;
  gap: var(--space-2);
  position: relative;
}

.message-row.is-swiping .bubble {
  transition: none;
}

.swipe-reply-hint {
  position: absolute;
  left: 4px;
  top: 50%;
  transform: translateY(-50%);
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  color: var(--color-primary);
  pointer-events: none;
}

.message-row.select-mode {
  cursor: pointer;
}

.message-row.selected .bubble {
  outline: 2px solid var(--color-primary);
  outline-offset: 1px;
}

.msg-check {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  flex-shrink: 0;
  padding-bottom: 4px;
  cursor: pointer;
}

.msg-check input {
  width: 16px;
  height: 16px;
  accent-color: var(--color-primary);
}

.select-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-6);
  font-size: var(--text-sm);
  color: var(--color-text);
  background: var(--color-bg-surface);
  border-top: 1px solid var(--color-border);
}

.select-bar-actions {
  display: flex;
  gap: var(--space-2);
}

.local-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-8) var(--space-4);
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}

.local-empty p {
  margin: 0;
}

.message-row.me {
  justify-content: flex-end;
}

.bubble {
  position: relative;
  max-width: min(70%, 480px);
  padding: var(--space-3) var(--space-4);
  font-size: var(--text-sm);
  line-height: 1.55;
  border-radius: var(--radius-lg);
  border-bottom-left-radius: var(--space-1);
  background: var(--color-bubble-other);
  box-shadow: var(--shadow-sm);
  word-break: break-word;
  touch-action: pan-y;
  transition: opacity var(--transition-fast), box-shadow var(--transition-fast), transform 0.15s ease;
}

.message-row.sending .bubble {
  opacity: 0.72;
}

.message-row.failed .bubble {
  opacity: 0.92;
  box-shadow: 0 0 0 1px rgba(220, 38, 38, 0.28);
}

.msg-sender {
  margin-bottom: 4px;
  font-size: var(--text-xs);
  font-weight: 600;
  color: var(--color-primary);
}

.msg-quote {
  margin-bottom: 6px;
  padding: 6px 8px;
  border-left: 3px solid var(--color-primary);
  border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
  background: rgba(0, 0, 0, 0.06);
  font-size: var(--text-xs);
  line-height: 1.4;
  cursor: pointer;
  transition: background var(--transition-fast);
}

.msg-quote:hover {
  background: rgba(0, 0, 0, 0.1);
}

.message-row.me .msg-quote {
  background: rgba(255, 255, 255, 0.35);
}

.msg-quote-name {
  font-weight: 600;
  color: var(--color-primary);
}

.msg-quote-preview {
  color: var(--color-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 240px;
}

.reply-bar {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: 8px 12px;
  border-top: 1px solid var(--color-border);
  background: var(--color-bg-muted, rgba(0, 0, 0, 0.03));
}

.reply-bar-accent {
  flex-shrink: 0;
  width: 3px;
  align-self: stretch;
  min-height: 28px;
  border-radius: 2px;
  background: var(--color-primary);
}

.reply-bar-icon {
  flex-shrink: 0;
  color: var(--color-primary);
  opacity: 0.9;
}

.reply-bar-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.reply-bar-label {
  font-size: var(--text-xs);
  font-weight: 600;
  color: var(--color-primary);
}

.reply-bar-preview {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.reply-bar-cancel {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
}

.reply-bar-cancel:hover {
  background: rgba(0, 0, 0, 0.06);
  color: var(--color-text-primary);
}

.msg-actions {
  display: flex;
  gap: var(--space-2);
  margin-bottom: var(--space-1);
  opacity: 0;
  transition: opacity var(--transition-fast);
}

.bubble:hover .msg-actions-desktop,
.bubble:focus-within .msg-actions-desktop {
  opacity: 1;
}

@media (hover: none) {
  .msg-actions-desktop {
    display: none;
  }
}

.msg-action-btn {
  padding: 2px 8px;
  font-size: 10px;
  color: var(--color-text-muted);
  background: rgba(255, 255, 255, 0.85);
  border-radius: var(--radius-sm);
}

.recalled {
  color: var(--color-text-muted);
  font-style: italic;
}

.message-row.me .bubble {
  background: var(--color-bubble-me);
  border-bottom-left-radius: var(--radius-lg);
  border-bottom-right-radius: var(--space-1);
}

.typing-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: var(--space-1) var(--space-6);
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  background: var(--color-bg-surface);
  border-top: 1px solid var(--color-border);
}

.typing-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.typing-dots {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  flex-shrink: 0;
}

.typing-dots i {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--color-text-muted);
  opacity: 0.35;
  animation: typing-bounce 1.2s ease-in-out infinite;
}

.typing-dots i:nth-child(2) {
  animation-delay: 0.15s;
}

.typing-dots i:nth-child(3) {
  animation-delay: 0.3s;
}

@keyframes typing-bounce {
  0%,
  60%,
  100% {
    opacity: 0.35;
    transform: translateY(0);
  }
  30% {
    opacity: 1;
    transform: translateY(-2px);
  }
}

.mention-panel {
  display: flex;
  flex-direction: column;
  max-height: 220px;
  overflow-y: auto;
  background: var(--color-bg-surface);
  border-top: 1px solid var(--color-border);
}

.mention-item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  width: 100%;
  padding: var(--space-2) var(--space-6);
  text-align: left;
  border: 0;
  background: transparent;
  cursor: pointer;
}

.mention-item:hover {
  background: var(--color-primary-muted);
}

.mention-name {
  font-size: var(--text-sm);
  font-weight: 500;
}

.mention-user {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}

:deep(.msg-text .mention) {
  color: var(--color-primary);
  font-weight: 600;
}

:deep(.msg-text .msg-link) {
  color: var(--color-primary);
  text-decoration: underline;
  word-break: break-all;
}

:deep(.msg-text .msg-link:hover) {
  opacity: 0.85;
}

.composer {
  display: flex;
  align-items: flex-end;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-5);
  padding-bottom: calc(var(--space-3) + env(safe-area-inset-bottom, 0px));
  background: var(--color-bg-surface);
  border-top: 1px solid var(--color-border);
}

.composer-icon-btn {
  flex-shrink: 0;
  width: 40px;
  min-width: 40px;
  padding: 0;
  color: var(--color-text-secondary);
}

.composer-icon-btn[aria-pressed='true'] {
  color: var(--color-primary);
  background: var(--color-primary-muted);
}

.send-btn {
  flex-shrink: 0;
  min-width: 72px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.send-btn.sending {
  opacity: 0.9;
}

.send-spin {
  animation: send-spin 0.7s linear infinite;
}

@keyframes send-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .send-spin {
    animation: none;
  }
}

.composer-input {
  flex: 1;
  min-width: 0;
  min-height: 40px;
  max-height: 120px;
  padding-top: 10px;
  padding-bottom: 10px;
  line-height: 1.4;
  resize: none;
  overflow-y: auto;
  transition: height var(--transition-fast);
}

.hidden-file {
  display: none;
}

.msg-image-wrap {
  position: relative;
  display: inline-block;
  max-width: 240px;
}

.msg-image {
  display: block;
  max-width: min(70vw, 240px);
  max-height: 200px;
  width: auto;
  height: auto;
  object-fit: cover;
  object-position: center;
  border-radius: var(--radius-sm);
  background: var(--color-border);
  opacity: 0;
  transition: opacity var(--transition-base);
  /* WeChat-like: extreme aspect ratios clipped toward ~3:1 feel via max box */
  aspect-ratio: auto;
}

.msg-image.loaded,
.msg-image[src^='blob:'] {
  opacity: 1;
}

.msg-image.clickable {
  cursor: zoom-in;
}

.upload-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  background: rgba(15, 23, 42, 0.45);
  border-radius: var(--radius-sm);
}

.upload-bar {
  width: 70%;
  height: 4px;
  background: rgba(255, 255, 255, 0.35);
  border-radius: var(--radius-full);
  overflow: hidden;
}

.upload-bar-fill {
  height: 100%;
  background: #fff;
  transition: width 0.15s ease;
}

.upload-pct {
  font-size: var(--text-xs);
  color: #fff;
  font-weight: 600;
}

.upload-file-pct {
  display: block;
  margin-top: 2px;
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}

.upload-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: var(--space-2) var(--space-6);
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  background: var(--color-bg-surface);
  border-top: 1px solid var(--color-border);
}

.upload-banner-icon {
  flex-shrink: 0;
  color: var(--color-primary);
}

.upload-banner-copy {
  flex: 1;
  min-width: 0;
}

.upload-banner-bar {
  margin-top: 4px;
  height: 3px;
  background: var(--color-border);
  border-radius: var(--radius-full);
  overflow: hidden;
}

.upload-banner-fill {
  height: 100%;
  background: var(--color-primary);
  transition: width 0.15s ease;
}

.emoji-panel {
  display: grid;
  grid-template-columns: repeat(8, 1fr);
  gap: 4px;
  padding: var(--space-3) var(--space-6);
  background: var(--color-bg-surface);
  border-top: 1px solid var(--color-border);
}

.emoji-btn {
  width: 100%;
  aspect-ratio: 1;
  font-size: 1.25rem;
  line-height: 1;
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  cursor: pointer;
}

.emoji-btn:hover {
  background: var(--color-primary-muted);
}

.retry-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-top: var(--space-1);
  padding: 2px 0;
  font-size: var(--text-xs);
  font-weight: 500;
  color: var(--color-danger);
  background: transparent;
  border: 0;
  cursor: pointer;
  opacity: 0.9;
  transition: opacity var(--transition-fast);
}

.retry-btn:hover {
  opacity: 1;
  text-decoration: underline;
}

.file-link {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  max-width: 100%;
  color: var(--color-primary);
  text-decoration: none;
}

.file-link:hover .file-name {
  text-decoration: underline;
}

.file-icon {
  flex-shrink: 0;
  opacity: 0.85;
}

.file-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-size {
  flex-shrink: 0;
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}

.mobile-menu-btn {
  display: none;
  flex-shrink: 0;
}

.sidebar-backdrop {
  display: none;
}

@media (max-width: 768px) {
  .app-shell {
    grid-template-columns: 1fr;
    position: relative;
  }

  .mobile-menu-btn {
    display: inline-flex;
  }

  .sidebar-backdrop {
    display: block;
    position: fixed;
    inset: 0;
    z-index: 40;
    background: rgba(15, 23, 42, 0.35);
  }

  .sidebar {
    position: fixed;
    top: 0;
    left: 0;
    bottom: 0;
    z-index: 50;
    width: min(86vw, 320px);
    max-height: none;
    border-right: 1px solid var(--color-border);
    border-bottom: none;
    transform: translateX(-105%);
    transition: transform 0.2s ease;
    box-shadow: 8px 0 24px rgba(15, 23, 42, 0.12);
    padding-bottom: env(safe-area-inset-bottom, 0px);
  }

  .sidebar.open {
    transform: translateX(0);
  }

  .app-shell:not(.chat-open) .sidebar {
    transform: translateX(0);
    box-shadow: none;
  }

  .app-shell:not(.chat-open) .sidebar-backdrop {
    display: none;
  }

  .app-shell:not(.chat-open) .chat-main {
    display: none;
  }

  .chat-header {
    gap: var(--space-2);
    padding-left: var(--space-3);
  }

  .emoji-panel {
    grid-template-columns: repeat(6, 1fr);
  }
}

/* Motion: panel / stack / fab / menu */
.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--transition-base);
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.fade-scale-enter-active,
.fade-scale-leave-active {
  transition:
    opacity var(--transition-fast),
    transform var(--transition-fast);
}
.fade-scale-enter-from,
.fade-scale-leave-to {
  opacity: 0;
  transform: scale(0.96);
}

.panel-slide-enter-active,
.panel-slide-leave-active {
  transition:
    opacity var(--transition-base),
    transform var(--transition-base),
    max-height var(--transition-base);
  overflow: hidden;
}
.panel-slide-enter-from,
.panel-slide-leave-to {
  opacity: 0;
  max-height: 0;
  transform: translateY(-6px);
}
.panel-slide-enter-to,
.panel-slide-leave-from {
  max-height: 280px;
}

.stack-enter-active,
.stack-leave-active {
  transition:
    opacity var(--transition-base),
    transform var(--transition-base),
    max-height var(--transition-base);
  overflow: hidden;
}
.stack-enter-from,
.stack-leave-to {
  opacity: 0;
  max-height: 0;
  transform: translateY(8px);
}
.stack-enter-to,
.stack-leave-from {
  max-height: 260px;
}

.fab-enter-active,
.fab-leave-active {
  transition:
    opacity var(--transition-base),
    transform var(--transition-base);
}
.fab-enter-from,
.fab-leave-to {
  opacity: 0;
  transform: translateY(10px) scale(0.96);
}

.confirm-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1200;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-4);
  background: rgba(15, 23, 42, 0.45);
}

.confirm-card {
  width: 100%;
  max-width: 360px;
  padding: var(--space-5);
  background: var(--color-bg-surface);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg, 0 16px 40px rgba(15, 23, 42, 0.18));
  animation: confirm-in var(--transition-base) ease;
}

@keyframes confirm-in {
  from {
    opacity: 0;
    transform: translateY(8px) scale(0.98);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.confirm-title {
  margin: 0 0 var(--space-2);
  font-size: var(--text-base);
  font-weight: 600;
}

.confirm-body {
  margin: 0 0 var(--space-5);
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  line-height: 1.5;
}

.confirm-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
}

.tab-fade-enter-active,
.tab-fade-leave-active {
  transition: opacity var(--transition-fast);
}
.tab-fade-enter-from,
.tab-fade-leave-to {
  opacity: 0;
}

.pending-item-enter-active,
.pending-item-leave-active {
  transition:
    opacity var(--transition-base),
    transform var(--transition-base),
    max-height var(--transition-base);
  overflow: hidden;
}
.pending-item-enter-from,
.pending-item-leave-to {
  opacity: 0;
  transform: translateX(-8px);
  max-height: 0;
}
.pending-item-enter-to,
.pending-item-leave-from {
  max-height: 72px;
}
</style>
