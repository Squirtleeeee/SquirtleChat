<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { parseError } from '../api/errors'
import AddContactModal from '../components/AddContactModal.vue'
import ImageLightbox from '../components/ImageLightbox.vue'
import UserAvatar from '../components/UserAvatar.vue'
import LinkPreviewCard from '../components/LinkPreviewCard.vue'
import { useAuthStore } from '../stores/auth'
import {
  friendDisplayName,
  parseFileContent,
  useChatStore,
  REACTION_EMOJIS,
  type ChatMessage,
  type FriendWithConv,
  type GroupWithConv,
} from '../stores/chat'
import type { PublicProfile } from '../stores/auth'
import { dayKey, formatDateDivider, formatListTime, formatMessageTime, previewMessage } from '../utils/format'
import { ensureNotifyPermission } from '../utils/notify'
import { directConvId, idStr, sameId } from '../utils/id'
import { mediaUrl } from '../utils/media'
import { isAgentProfile, AGENT_AVATAR } from '../constants/agent'
import { buildReplyContent, parseReplyContent, type ReplyMeta } from '../utils/reply'
import { firstHttpUrl } from '../utils/link'
import { useSettingsStore } from '../stores/settings'
import { openDetachedChat } from '../utils/desktop'

defineOptions({ name: 'ChatView' })

const auth = useAuthStore()
const chat = useChatStore()
const settings = useSettingsStore()
const router = useRouter()
const input = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const recording = ref(false)
const recordSecs = ref(0)
let mediaRecorder: MediaRecorder | null = null
let recordChunks: BlobPart[] = []
let recordTimer = 0
let recordStartedAt = 0
const messageListEl = ref<HTMLElement | null>(null)
const showAddModal = ref(false)
const filterQuery = ref('')
const previewImage = ref('')
const showSearch = ref(false)
const searchHitIndex = ref(-1)
const showPins = ref(false)
const showBookmarks = ref(false)
const bookmarkTitle = ref('')
const bookmarkUrl = ref('')
const editingMsgId = ref('')
const editDraft = ref('')
const showStars = ref(false)
const showSchedule = ref(false)
const showReminders = ref(false)
const showMedia = ref(false)
const showMentions = ref(false)
const showPollComposer = ref(false)
const pollQuestion = ref('')
const pollOptions = ref(['', ''])
const scheduleAtLocal = ref('')
const scheduledItems = ref<{ id: string; conversation_id: string; content: string; send_at: string }[]>([])
const searchInput = ref('')
const searchInputEl = ref<HTMLInputElement | null>(null)
const skipAutoScroll = ref(false)
let searchDebounce = 0
const DRAFT_KEY = 'squirtlechat_drafts'
const NOTICE_DISMISS_KEY = 'squirtlechat_notice_dismissed'
const draftsMap = ref<Record<string, string>>(loadDrafts())
let draftSyncTimer = 0
const pendingDraftSync = new Set<string>()
const noticeDismissed = ref<Record<string, string>>(loadNoticeDismissed())
const mentionOpen = ref(false)

function loadNoticeDismissed(): Record<string, string> {
  try {
    return JSON.parse(localStorage.getItem(NOTICE_DISMISS_KEY) || '{}') as Record<string, string>
  } catch {
    return {}
  }
}

function dismissActiveNotice() {
  const gid = chat.activeGroupId
  const notice = chat.activeGroupNotice
  if (!gid || !notice) return
  noticeDismissed.value = { ...noticeDismissed.value, [gid]: notice }
  localStorage.setItem(NOTICE_DISMISS_KEY, JSON.stringify(noticeDismissed.value))
}

const showGroupNoticeBar = computed(() => {
  const gid = chat.activeGroupId
  const notice = chat.activeGroupNotice
  if (!gid || !notice) return false
  return noticeDismissed.value[gid] !== notice
})
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
const forwardOpen = ref(false)
const forwardQuery = ref('')
const forwarding = ref(false)
const reactPickerFor = ref('')
const groupReadPopupFor = ref('')

function toggleReactPicker(clientMsgId: string) {
  reactPickerFor.value = reactPickerFor.value === clientMsgId ? '' : clientMsgId
}

function toggleGroupReadPopup(clientMsgId: string) {
  const next = groupReadPopupFor.value === clientMsgId ? '' : clientMsgId
  groupReadPopupFor.value = next
  if (next && chat.activeConvId) {
    void chat.loadReadState(chat.activeConvId)
  }
}

function groupReadProfile(userId: string): PublicProfile | undefined {
  return (
    chat.activeGroupMembers.find((x) => sameId(x.id, userId)) ||
    chat.friends.find((x) => sameId(x.id, userId))
  )
}

function groupReadName(userId: string) {
  const m = groupReadProfile(userId)
  if (!m) return userId
  const f = chat.friends.find((x) => sameId(x.id, userId))
  if (f) return friendDisplayName(f)
  return chat.mentionName(m)
}

function groupReadAvatar(userId: string) {
  return avatarUrl(groupReadProfile(userId)?.avatar)
}

function openGroupReadProfile(userId: string) {
  groupReadPopupFor.value = ''
  router.push(`/profile/${userId}`)
}

async function onPickReaction(m: ChatMessage, emoji: string) {
  reactPickerFor.value = ''
  await chat.toggleReaction(m, emoji)
}

async function runGlobalSearch() {
  await chat.searchGlobal(filterQuery.value)
}

function convTitleHint(convId: string) {
  if (convId.startsWith('g_')) {
    const g = chat.groups.find((x) => x.conversation_id === convId)
    return g?.name || '群聊'
  }
  const parts = convId.split('_')
  const other = parts.find((p) => !sameId(p, auth.user?.id))
  const f = chat.friends.find((x) => sameId(x.id, other))
  return f ? friendDisplayName(f) : '私聊'
}

async function openGlobalHit(m: ChatMessage) {
  await chat.jumpToMessage(m)
  mobileSidebarOpen.value = false
}
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
const draftConflict = ref<{
  convId: string
  local: string
  remote: string
  rest: { convId: string; local: string; remote: string }[]
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

const mentionShowAll = computed(() => {
  if (!chat.activeGroupId || !mentionOpen.value) return false
  const q = mentionQuery.value.trim().toLowerCase()
  if (!q) return true
  return '所有人'.includes(q) || 'all'.includes(q) || q.includes('全')
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

function queueDraftSync(convId: string) {
  pendingDraftSync.add(convId)
  window.clearTimeout(draftSyncTimer)
  draftSyncTimer = window.setTimeout(() => {
    void flushDraftSync()
  }, 800)
}

async function flushDraftSync() {
  const ids = [...pendingDraftSync]
  pendingDraftSync.clear()
  for (const convId of ids) {
    const content = draftsMap.value[convId] || ''
    try {
      await chat.persistDraft(convId, content)
    } catch {
      pendingDraftSync.add(convId)
    }
  }
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
  queueDraftSync(convId)
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

async function syncDraftsFromCloud() {
  try {
    const remote = await chat.loadDrafts()
    if (!remote) return
    const merged = { ...draftsMap.value }
    const localOnly: string[] = []
    const conflicts: { convId: string; local: string; remote: string }[] = []

    for (const [k, v] of Object.entries(remote)) {
      if (typeof v !== 'string' || !v) continue
      const local = (draftsMap.value[k] || '').trim()
      const cloud = v.trim()
      if (local && cloud && local !== cloud) {
        conflicts.push({ convId: k, local, remote: cloud })
        continue
      }
      if (!local && cloud) merged[k] = v
    }
    for (const [k, v] of Object.entries(draftsMap.value)) {
      if (v && !(k in remote)) localOnly.push(k)
    }

    // non-conflicting merges first
    for (const c of conflicts) {
      // keep previous local until resolved
      merged[c.convId] = c.local
    }
    draftsMap.value = merged
    persistDrafts()
    for (const k of localOnly) {
      queueDraftSync(k)
    }
    if (conflicts.length) {
      const [first, ...rest] = conflicts
      draftConflict.value = { ...first, rest }
      chat.setTransientNotice(`发现 ${conflicts.length} 处草稿冲突，请选择保留版本`)
    } else if (chat.activeConvId) {
      input.value = readDraft(chat.activeConvId)
    }
  } catch {
    /* keep local */
  }
}

function applyDraftConflict(keep: 'local' | 'cloud') {
  const cur = draftConflict.value
  if (!cur) return
  const next = { ...draftsMap.value }
  if (keep === 'cloud') {
    next[cur.convId] = cur.remote
  } else {
    next[cur.convId] = cur.local
    queueDraftSync(cur.convId)
  }
  draftsMap.value = next
  persistDrafts()
  if (chat.activeConvId === cur.convId) {
    input.value = next[cur.convId] || ''
  }
  if (cur.rest.length) {
    const [first, ...rest] = cur.rest
    draftConflict.value = { ...first, rest }
  } else {
    draftConflict.value = null
    chat.setTransientNotice(keep === 'local' ? '已保留本地草稿' : '已采用云端草稿')
  }
}

function draftConflictTitle(convId: string) {
  const g = chat.groups.find((x) => x.conversation_id === convId)
  if (g) return g.name
  const myId = idStr(auth.user?.id)
  const f = chat.friends.find((x) => directConvId(myId, x.id) === convId)
  return f ? friendDisplayName(f) : '会话'
}

function previewDraftSnippet(text: string) {
  const t = text.trim().replace(/\s+/g, ' ')
  return t.length > 80 ? `${t.slice(0, 80)}…` : t
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
  let list = chat.sortedFriends.filter((f) => chat.friendInActiveFolder(f.id))
  const q = filterText.value
  if (!q) return list
  return list.filter((f) => {
    const name = friendDisplayName(f).toLowerCase()
    const user = f.username.toLowerCase()
    return name.includes(q) || user.includes(q) || (f.remark || '').toLowerCase().includes(q)
  })
})
const filteredGroups = computed(() => {
  let list = chat.sortedGroups.filter((g) => chat.groupInActiveFolder(g))
  const q = filterText.value
  if (!q) return list
  return list.filter((g) => g.name.toLowerCase().includes(q))
})
const newFolderName = ref('')
const showFolderManage = ref(false)

function ctxConvId(): string {
  const menu = ctxMenu.value
  if (!menu) return ''
  if (menu.kind === 'friend') return directConvId(auth.user?.id || '', menu.id)
  return chat.groups.find((x) => x.id === menu.id)?.conversation_id || ''
}

function ctxAssignFolder(folderId: string) {
  const convId = ctxConvId()
  closeCtxMenu()
  if (!convId) return
  chat.assignConvToFolder(folderId, convId)
}

function ctxRemoveFromFolders() {
  const convId = ctxConvId()
  closeCtxMenu()
  if (!convId) return
  chat.removeConvFromFolders(convId)
}

function submitNewFolder() {
  chat.createFolder(newFolderName.value)
  newFolderName.value = ''
}
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
  if (showSearch.value && chat.searchResults.length) {
    if (e.key === 'F3' || ((e.ctrlKey || e.metaKey) && (e.key === 'g' || e.key === 'G'))) {
      e.preventDefault()
      void jumpSearchHit(e.shiftKey ? -1 : 1)
      return
    }
    if (document.activeElement === searchInputEl.value) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        void jumpSearchHit(1)
        return
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        void jumpSearchHit(-1)
        return
      }
    }
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
      chat.loadConversations(),
      chat.loadPending(),
      chat.loadGroupInvitations(),
      chat.loadChatPrefs(),
      chat.loadReminders(),
    ])
    await syncDraftsFromCloud()
    await chat.pullSync()
    setInterval(() => chat.pullSync(), 3000)
    setInterval(() => chat.loadPending(), 8000)
    setInterval(() => {
      void chat.refreshPresence(chat.friends.map((f) => f.id))
    }, 20000)
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
  window.clearTimeout(draftSyncTimer)
  void flushDraftSync()
})

function send() {
  if (mentionOpen.value && (mentionShowAll.value || mentionCandidates.value.length)) {
    if (mentionShowAll.value && !mentionCandidates.value.length) insertMentionAll()
    else if (mentionShowAll.value && !mentionQuery.value.trim()) insertMentionAll()
    else if (mentionCandidates.value.length) insertMention(mentionCandidates.value[0])
    else insertMentionAll()
    return
  }
  if (chat.activeGroupId && !chat.canPostInActiveGroup) {
    chat.setTransientNotice(chat.activeGroupPostBlockReason || '暂时无法发言')
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
  insertMentionToken(name)
}

function insertMentionAll() {
  insertMentionToken('所有人')
}

function insertMentionToken(name: string) {
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
    .replace(/(^|[\s([{（【「『])#([a-zA-Z0-9_\u4e00-\u9fff]{1,64})/g, (_m, pre, tag) => {
      return `${pre}<button type="button" class="msg-hashtag" data-tag="${tag}">#${tag}</button>`
    })
    .replace(/\n/g, '<br>')
}

function messageLink(m: ChatMessage) {
  if (m.msg_type !== 1) return null
  const text = parseReplyContent(m.content).text || messageBody(m)
  return firstHttpUrl(text)
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

async function startVoiceRecord() {
  if (recording.value || chat.uploading || !chat.activeConvId) return
  if (!navigator.mediaDevices?.getUserMedia) {
    chat.setTransientNotice('当前环境不支持录音')
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
        chat.setTransientNotice('录音太短')
        return
      }
      await chat.sendVoice(blob, duration)
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
    chat.setTransientNotice('无法访问麦克风')
  }
}

function stopVoiceRecord() {
  if (!recording.value || !mediaRecorder) return
  if (mediaRecorder.state !== 'inactive') mediaRecorder.stop()
}

function cancelVoiceRecord() {
  if (!mediaRecorder) {
    recording.value = false
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

function voiceMeta(m: ChatMessage) {
  const f = parseFileContent(m.content)
  return {
    url: m.localPreview || (f?.url ? mediaUrl(f.url) : ''),
    duration: f?.duration || 1,
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

function runMsgMenu(action: 'reply' | 'copy' | 'translate' | 'recall' | 'select') {
  const menu = msgMenu.value
  msgMenu.value = null
  if (!menu) return
  const m = menu.message
  if (action === 'reply') startReply(m)
  else if (action === 'copy') void copyText(messageBody(m))
  else if (action === 'translate') void chat.translateMessage(m)
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
function msgMenuTranslate() {
  runMsgMenu('translate')
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

function selectAllVisibleMessages() {
  const list = chat.messages[chat.activeConvId || ''] || []
  selectedMsgIds.value = new Set(
    list.filter((m) => m.msg_type !== 4 && m.content !== '[已撤回]').map((m) => m.client_msg_id),
  )
}

function clearSelectedMessages() {
  selectedMsgIds.value = new Set()
}

const selectedStarrableCount = computed(
  () =>
    selectedMessagesInOrder().filter((m) => m.msg_id && m.msg_type !== 4 && !chat.isStarred(m.msg_id))
      .length,
)

async function starSelectedMessages() {
  const msgs = selectedMessagesInOrder()
  if (!msgs.length) return
  await chat.batchStar(msgs)
  selectMode.value = false
  selectedMsgIds.value = new Set()
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

const forwardFriends = computed(() => {
  const q = forwardQuery.value.trim().toLowerCase()
  return chat.sortedFriends.filter((f) => {
    if (!q) return true
    return friendDisplayName(f).toLowerCase().includes(q) || f.username.toLowerCase().includes(q)
  })
})

const forwardGroups = computed(() => {
  const q = forwardQuery.value.trim().toLowerCase()
  return chat.sortedGroups.filter((g) => {
    if (!q) return true
    return g.name.toLowerCase().includes(q)
  })
})

function openForwardPicker() {
  if (!selectedCount.value) return
  const eligible = selectedMessagesInOrder().filter(
    (m) => m.msg_type !== 4 && m.content !== '[已撤回]',
  )
  if (!eligible.length) {
    chat.setTransientNotice('没有可转发的消息')
    return
  }
  if (eligible.length > 30) {
    chat.setError('一次最多转发 30 条，请减少选择')
    return
  }
  forwardQuery.value = ''
  forwardOpen.value = true
}

function closeForwardPicker() {
  forwardOpen.value = false
  forwarding.value = false
}

async function forwardToFriend(f: FriendWithConv) {
  const msgs = selectedMessagesInOrder()
  if (!msgs.length || forwarding.value) return
  forwarding.value = true
  try {
    const convId = directConvId(auth.user?.id || '', f.id)
    await chat.forwardMessages(
      { conversationId: convId, conversationType: 1, toUserId: String(f.id) },
      msgs,
    )
    selectMode.value = false
    selectedMsgIds.value = new Set()
    closeForwardPicker()
  } finally {
    forwarding.value = false
  }
}

async function forwardToGroup(g: GroupWithConv) {
  const msgs = selectedMessagesInOrder()
  if (!msgs.length || forwarding.value) return
  forwarding.value = true
  try {
    await chat.forwardMessages(
      {
        conversationId: g.conversation_id,
        conversationType: 2,
        groupId: String(g.id),
      },
      msgs,
    )
    selectMode.value = false
    selectedMsgIds.value = new Set()
    closeForwardPicker()
  } finally {
    forwarding.value = false
  }
}

function canRecall(m: ChatMessage) {
  if (!sameId(m.from_user_id, auth.user?.id)) return false
  if (m.msg_type === 4 || m.content === '[已撤回]') return false
  if (!m.msg_id) return false
  if (!m.created_at) return true
  return Date.now() - new Date(m.created_at).getTime() < 2 * 60 * 1000
}

function canEdit(m: ChatMessage) {
  if (!sameId(m.from_user_id, auth.user?.id)) return false
  if (m.msg_type !== 1) return false
  if (m.content === '[已撤回]') return false
  if (!m.msg_id) return false
  if (parseFileContent(m.content)) return false
  if (!m.created_at) return true
  return Date.now() - new Date(m.created_at).getTime() < 15 * 60 * 1000
}

function startEdit(m: ChatMessage) {
  editingMsgId.value = m.client_msg_id
  editDraft.value = messageBody(m)
}

function cancelEdit() {
  editingMsgId.value = ''
  editDraft.value = ''
}

async function saveEdit(m: ChatMessage) {
  const reply = messageReply(m)
  const body = editDraft.value.trim()
  if (!body) return
  const content = reply ? buildReplyContent(reply, body) : body
  await chat.editMessage(m, content)
  cancelEdit()
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

/** 打开会话或历史加载后：有未读则定位到未读分隔线，否则滚到底 */
async function scrollToLatestOnOpen() {
  nearBottom.value = true
  pendingNewCount.value = 0
  skipAutoScroll.value = true
  try {
    await nextTick()
    if (unreadDividerAfterSeq.value > 0) {
      for (let i = 0; i < 4; i++) {
        const hit = await jumpToFirstUnread(false)
        if (hit) break
        await new Promise<void>((r) => requestAnimationFrame(() => r()))
      }
      return
    }
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

async function jumpToFirstUnread(smooth = true): Promise<boolean> {
  await nextTick()
  const el = messageListEl.value
  if (!el) return false
  const sep = el.querySelector('#unread-sep, [data-unread-sep="1"]') as HTMLElement | null
  if (!sep) return false
  sep.scrollIntoView({ behavior: smooth ? 'smooth' : 'auto', block: 'center' })
  nearBottom.value = isNearBottom(el)
  return true
}

const hasUnreadMarker = computed(() => unreadDividerAfterSeq.value > 0)

function toggleSearch() {
  showSearch.value = !showSearch.value
  if (!showSearch.value) {
    searchInput.value = ''
    searchHitIndex.value = -1
    chat.clearSearch()
  } else {
    showPins.value = false
    showBookmarks.value = false
    showStars.value = false
    showSchedule.value = false
    showReminders.value = false
    void chat.loadHashtags()
    void nextTick(() => searchInputEl.value?.focus())
  }
}

function onMsgTextClick(e: MouseEvent) {
  const el = (e.target as HTMLElement | null)?.closest?.('.msg-hashtag') as HTMLElement | null
  if (!el) return
  e.preventDefault()
  const tag = el.getAttribute('data-tag') || ''
  if (!tag) return
  showSearch.value = true
  searchInput.value = `#${tag}`
  void chat.searchByHashtag(tag)
}

function selectHashtag(tag: string) {
  searchHitIndex.value = -1
  searchInput.value = `#${tag}`
  void chat.searchByHashtag(tag)
}

function togglePinsPanel() {
  showPins.value = !showPins.value
  if (showPins.value) {
    showSearch.value = false
    showBookmarks.value = false
    showStars.value = false
    void chat.loadPins()
  }
}

function toggleBookmarksPanel() {
  showBookmarks.value = !showBookmarks.value
  if (showBookmarks.value) {
    showSearch.value = false
    showPins.value = false
    showStars.value = false
    bookmarkTitle.value = ''
    bookmarkUrl.value = ''
    void chat.loadBookmarks()
  }
}

async function submitBookmark() {
  const title = bookmarkTitle.value.trim()
  const url = bookmarkUrl.value.trim()
  if (!title || !url) return
  await chat.addBookmark(title, url)
  bookmarkTitle.value = ''
  bookmarkUrl.value = ''
}

function toggleStarsPanel() {
  showStars.value = !showStars.value
  if (showStars.value) {
    showSearch.value = false
    showPins.value = false
    showBookmarks.value = false
    showSchedule.value = false
    showReminders.value = false
    void chat.loadStarred()
  }
}

async function toggleSchedulePanel() {
  showSchedule.value = !showSchedule.value
  if (showSchedule.value) {
    showSearch.value = false
    showPins.value = false
    showBookmarks.value = false
    showStars.value = false
    showReminders.value = false
    const min = new Date(Date.now() + 60_000)
    scheduleAtLocal.value = toLocalInputValue(min)
    scheduledItems.value = await chat.loadScheduled()
  }
}

async function toggleRemindersPanel() {
  showReminders.value = !showReminders.value
  if (showReminders.value) {
    showSearch.value = false
    showPins.value = false
    showBookmarks.value = false
    showStars.value = false
    showSchedule.value = false
    showMentions.value = false
    showMedia.value = false
    await chat.loadReminders()
  }
}

async function toggleMediaPanel() {
  showMedia.value = !showMedia.value
  if (showMedia.value) {
    showSearch.value = false
    showPins.value = false
    showBookmarks.value = false
    showStars.value = false
    showSchedule.value = false
    showReminders.value = false
    showMentions.value = false
    await chat.loadMedia(chat.mediaKind || 'all')
  }
}

function remindIn(msg: { msg_id?: string | number; conversation_id?: string; msg_type?: number; content?: string }, minutes: number) {
  const at = new Date(Date.now() + minutes * 60_000)
  void chat.remindMessage(msg as any, at.toISOString())
}

function remindTomorrowMorning(msg: { msg_id?: string | number; conversation_id?: string; msg_type?: number; content?: string }) {
  const at = new Date()
  at.setDate(at.getDate() + 1)
  at.setHours(9, 0, 0, 0)
  if (at.getTime() - Date.now() < 30_000) {
    at.setDate(at.getDate() + 1)
  }
  void chat.remindMessage(msg as any, at.toISOString())
}

async function jumpToReminder(item: {
  conversation_id: string
  msg_id: string
  preview: string
}) {
  showReminders.value = false
  const group = chat.groups.find((g) => g.conversation_id === item.conversation_id)
  if (group) {
    await chat.openGroup(group)
    return
  }
  // direct conv id is often "d_{min}_{max}" — match friend via openDirect from sidebar list
  const myId = idStr(auth.user?.id)
  const friend = chat.friends.find((f) => directConvId(myId, f.id) === item.conversation_id)
  if (friend) {
    await chat.openDirect(friend)
    return
  }
  chat.activeConvId = item.conversation_id
  chat.activeGroupId = ''
  await chat.loadHistory(item.conversation_id)
}

function toLocalInputValue(d: Date) {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

async function submitSchedule() {
  const text = input.value.trim()
  if (!text || !scheduleAtLocal.value || !chat.activeConvId) return
  const sendAt = new Date(scheduleAtLocal.value)
  if (Number.isNaN(sendAt.getTime())) {
    chat.setError('时间无效')
    return
  }
  await chat.scheduleMessage(text, sendAt.toISOString())
  input.value = ''
  saveDraft(chat.activeConvId, '')
  scheduledItems.value = await chat.loadScheduled()
}

async function cancelScheduledItem(id: string) {
  await chat.cancelScheduled(id)
  scheduledItems.value = await chat.loadScheduled()
}

async function toggleMentionsPanel() {
  showMentions.value = !showMentions.value
  if (showMentions.value) {
    showSearch.value = false
    showPins.value = false
    showBookmarks.value = false
    showStars.value = false
    showSchedule.value = false
    await chat.loadMentions()
  }
}

function togglePollComposer() {
  showPollComposer.value = !showPollComposer.value
  if (showPollComposer.value) {
    pollQuestion.value = ''
    pollOptions.value = ['', '']
  }
}

function addPollOption() {
  if (pollOptions.value.length >= 8) return
  pollOptions.value = [...pollOptions.value, '']
}

async function submitPoll() {
  await chat.sendPoll(pollQuestion.value, pollOptions.value)
  showPollComposer.value = false
  pollQuestion.value = ''
  pollOptions.value = ['', '']
}

type PollContent = { question: string; options: { id: string; text: string }[] }

function parsePoll(content: string): PollContent | null {
  try {
    const o = JSON.parse(content) as PollContent
    if (o?.question && Array.isArray(o.options)) return o
  } catch {
    /* ignore */
  }
  return null
}

function pollCount(msgId: string | number | undefined, optionId: string) {
  const p = chat.pollFor(msgId)
  return p?.counts.find((c) => c.option_id === optionId)?.count || 0
}

function pollPct(msgId: string | number | undefined, optionId: string) {
  const p = chat.pollFor(msgId)
  if (!p || !p.total) return 0
  return Math.round((pollCount(msgId, optionId) / p.total) * 100)
}

function onSearchInput() {
  window.clearTimeout(searchDebounce)
  searchHitIndex.value = -1
  searchDebounce = window.setTimeout(() => {
    void chat.searchMessages(searchInput.value)
  }, 300)
}

async function jumpToSearchResult(m: ChatMessage) {
  const idx = chat.searchResults.findIndex((x) => x.client_msg_id === m.client_msg_id)
  if (idx >= 0) searchHitIndex.value = idx
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

async function jumpSearchHit(delta: number) {
  const list = chat.searchResults
  if (!list.length) return
  let next = searchHitIndex.value
  if (next < 0) next = delta > 0 ? 0 : list.length - 1
  else next = (next + delta + list.length) % list.length
  searchHitIndex.value = next
  await jumpToSearchResult(list[next])
}

const searchHitLabel = computed(() => {
  const n = chat.searchResults.length
  if (!n) return ''
  const i = searchHitIndex.value
  if (i < 0) return `${n} 条结果`
  return `${i + 1} / ${n}`
})

async function jumpToPinned(m: ChatMessage) {
  showPins.value = false
  await jumpToSearchResult(m)
}

function senderLabel(m: ChatMessage) {
  if (sameId(m.from_user_id, auth.user?.id)) return '我'
  if (chat.activeGroupId) {
    return chat.groupMemberDisplayName(
      m.from_user_id,
      chat.activeGroupMembers.find((x) => sameId(x.id, m.from_user_id)),
    )
  }
  const friend = chat.friends.find((f) => sameId(f.id, m.from_user_id))
  if (friend) return friendDisplayName(friend)
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
          placeholder="搜索好友 / 群 / 聊天记录"
          aria-label="搜索会话"
          @keydown.enter.prevent="runGlobalSearch"
        />
        <button
          type="button"
          class="btn btn-ghost btn-sm sidebar-search-btn"
          :disabled="!filterQuery.trim() || chat.globalSearchLoading"
          @click="runGlobalSearch"
        >
          {{ chat.globalSearchLoading ? '…' : '搜记录' }}
        </button>
      </div>
      <div v-if="chat.globalSearchResults.length" class="global-search-panel">
        <div class="global-search-head">
          <span>聊天记录</span>
          <button type="button" class="btn btn-ghost btn-sm" @click="chat.globalSearchResults = []">清除</button>
        </div>
        <button
          v-for="m in chat.globalSearchResults"
          :key="m.client_msg_id"
          type="button"
          class="global-search-item"
          @click="openGlobalHit(m)"
        >
          <span class="gs-conv">{{ convTitleHint(m.conversation_id) }}</span>
          <span class="gs-body">{{ previewMessage(m.content, m.msg_type) }}</span>
        </button>
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

      <div class="folder-bar" role="toolbar" aria-label="会话文件夹">
        <button
          type="button"
          class="folder-chip"
          :class="{ active: !chat.activeFolderId }"
          @click="chat.activeFolderId = ''"
        >
          全部
        </button>
        <button
          v-for="f in chat.folders"
          :key="f.id"
          type="button"
          class="folder-chip"
          :class="{ active: chat.activeFolderId === f.id }"
          :title="`${f.name}（${f.conversation_ids.length}）`"
          @click="chat.setActiveFolder(f.id)"
        >
          {{ f.name }}
        </button>
        <button
          type="button"
          class="folder-chip folder-chip-manage"
          :aria-pressed="showFolderManage"
          @click="showFolderManage = !showFolderManage"
        >
          管理
        </button>
      </div>
      <div v-if="showFolderManage" class="folder-manage">
        <div class="folder-manage-row">
          <input
            v-model="newFolderName"
            class="input folder-manage-input"
            maxlength="24"
            placeholder="新文件夹名称"
            @keydown.enter.prevent="submitNewFolder"
          />
          <button type="button" class="btn btn-primary btn-sm" :disabled="!newFolderName.trim()" @click="submitNewFolder">
            新建
          </button>
        </div>
        <ul v-if="chat.folders.length" class="folder-manage-list">
          <li v-for="f in chat.folders" :key="f.id" class="folder-manage-item">
            <span>{{ f.name }} · {{ f.conversation_ids.length }}</span>
            <button type="button" class="btn btn-ghost btn-sm" @click="chat.deleteFolder(f.id)">删除</button>
          </li>
        </ul>
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
            <span
              class="online-dot"
              :class="{ on: chat.isOnline(f.id) }"
              :title="chat.isOnline(f.id) ? '在线' : '离线'"
            />
          </button>
          <div class="friend-meta">
            <div class="friend-row-top">
              <span class="friend-name">{{ friendDisplayName(f) }}</span>
              <span v-if="isAgentProfile(f)" class="agent-badge" title="AI 助手">龟</span>
              <span v-else-if="f.status_emoji || f.status_text" class="friend-status" :title="f.status_text || ''">
                {{ f.status_emoji }}{{ f.status_text ? ` ${f.status_text}` : '' }}
              </span>
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
            aria-label="导出会话"
            title="导出聊天记录为文本"
            @click="chat.exportTranscript()"
          >
            导出
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
            type="button"
            class="btn btn-ghost btn-sm"
            :aria-pressed="showMentions"
            aria-label="未读提及"
            @click="toggleMentionsPanel"
          >
            @提及{{ chat.mentionInbox.length ? ` ${chat.mentionInbox.length}` : '' }}
          </button>
          <button
            type="button"
            class="btn btn-ghost btn-sm"
            :aria-pressed="showStars"
            aria-label="我的收藏"
            @click="toggleStarsPanel"
          >
            收藏{{ chat.starredList.length ? ` ${chat.starredList.length}` : '' }}
          </button>
          <button
            type="button"
            class="btn btn-ghost btn-sm"
            :aria-pressed="showSchedule"
            aria-label="定时消息"
            @click="toggleSchedulePanel"
          >
            定时
          </button>
          <button
            type="button"
            class="btn btn-ghost btn-sm"
            :aria-pressed="showReminders"
            aria-label="消息提醒"
            @click="toggleRemindersPanel"
          >
            提醒{{ chat.reminderList.length ? ` ${chat.reminderList.length}` : '' }}
          </button>
          <button
            v-if="chat.activeConvId"
            type="button"
            class="btn btn-ghost btn-sm"
            :aria-pressed="showMedia"
            aria-label="媒体库"
            @click="toggleMediaPanel"
          >
            媒体
          </button>
          <button
            v-if="chat.activeConvId"
            type="button"
            class="btn btn-ghost btn-sm"
            :aria-pressed="showBookmarks"
            aria-label="会话书签"
            @click="toggleBookmarksPanel"
          >
            书签{{ chat.activeBookmarks().length ? ` ${chat.activeBookmarks().length}` : '' }}
          </button>
          <button
            v-if="chat.activeConvId"
            type="button"
            class="btn btn-ghost btn-sm"
            :aria-pressed="showPins"
            aria-label="置顶消息"
            @click="togglePinsPanel"
          >
            置顶{{ chat.activePins().length ? ` ${chat.activePins().length}` : '' }}
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
          <button
            v-if="chat.activeConvId && hasUnreadMarker"
            type="button"
            class="btn btn-ghost btn-sm"
            aria-label="跳到未读"
            @click="jumpToFirstUnread(true)"
          >
            未读
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
              placeholder="搜索本会话消息，或点话题…"
              aria-label="搜索本会话消息"
              @input="onSearchInput"
            />
            <button type="button" class="btn btn-ghost btn-sm" @click="toggleSearch">关闭</button>
          </div>
          <div v-if="chat.hashtagList.length" class="hashtag-chip-row">
            <button
              v-for="h in chat.hashtagList"
              :key="h.tag"
              type="button"
              class="hashtag-chip"
              :class="{ active: chat.activeHashtag === h.tag }"
              @click="selectHashtag(h.tag)"
            >
              #{{ h.tag }} · {{ h.count }}
            </button>
          </div>
          <div v-if="chat.searchLoading" class="msg-search-hint">搜索中…</div>
          <div v-else-if="searchInput.trim() && !chat.searchResults.length" class="msg-search-hint">无匹配消息</div>
          <template v-else-if="chat.searchResults.length">
            <div class="search-nav-bar" role="toolbar" aria-label="搜索结果导航">
              <span class="search-nav-label">{{ searchHitLabel }}</span>
              <div class="search-nav-actions">
                <button
                  type="button"
                  class="btn btn-ghost btn-sm"
                  :disabled="!chat.searchResults.length"
                  title="上一条 (Shift+F3)"
                  @click="jumpSearchHit(-1)"
                >
                  上一条
                </button>
                <button
                  type="button"
                  class="btn btn-ghost btn-sm"
                  :disabled="!chat.searchResults.length"
                  title="下一条 (F3)"
                  @click="jumpSearchHit(1)"
                >
                  下一条
                </button>
              </div>
            </div>
            <ul class="msg-search-list">
              <li v-for="(m, i) in chat.searchResults" :key="m.client_msg_id">
                <button
                  type="button"
                  class="msg-search-item"
                  :class="{ active: searchHitIndex === i }"
                  @click="jumpToSearchResult(m)"
                >
                  <span class="msg-search-meta">{{ senderLabel(m) }} · {{ formatMessageTime(m.created_at) }}</span>
                  <span class="msg-search-snippet">{{ messageBody(m) || m.content }}</span>
                </button>
              </li>
            </ul>
          </template>
        </div>
      </Transition>

      <Transition name="panel-slide">
        <div v-if="chat.activeConvId && showPins" class="msg-search-panel pins-panel">
          <div class="msg-search-bar">
            <span class="pins-panel-title">置顶消息（{{ chat.activePins().length }}）</span>
            <button type="button" class="btn btn-ghost btn-sm" @click="togglePinsPanel">关闭</button>
          </div>
          <div v-if="!chat.activePins().length" class="msg-search-hint">暂无置顶，悬停消息点「置顶」</div>
          <ul v-else class="msg-search-list">
            <li v-for="p in chat.activePins()" :key="p.message.client_msg_id || String(p.message.msg_id)">
              <div class="pin-row">
                <button type="button" class="msg-search-item" @click="jumpToPinned(p.message)">
                  <span class="msg-search-meta">
                    {{ senderLabel(p.message) }} · {{ formatMessageTime(p.message.created_at) }}
                  </span>
                  <span class="msg-search-snippet">{{ previewMessage(p.message.content, p.message.msg_type) }}</span>
                </button>
                <button
                  type="button"
                  class="btn btn-ghost btn-sm pin-unpin"
                  title="取消置顶"
                  @click="chat.togglePin(p.message)"
                >
                  取消
                </button>
              </div>
            </li>
          </ul>
        </div>
      </Transition>

      <Transition name="panel-slide">
        <div v-if="chat.activeConvId && showBookmarks" class="msg-search-panel pins-panel">
          <div class="msg-search-bar">
            <span class="pins-panel-title">会话书签（{{ chat.activeBookmarks().length }}）</span>
            <button type="button" class="btn btn-ghost btn-sm" @click="toggleBookmarksPanel">关闭</button>
          </div>
          <div class="bookmark-form">
            <input v-model="bookmarkTitle" class="msg-search-input" placeholder="标题" maxlength="64" />
            <input
              v-model="bookmarkUrl"
              class="msg-search-input"
              placeholder="https://…"
              maxlength="1024"
              @keydown.enter.prevent="submitBookmark"
            />
            <button
              type="button"
              class="btn btn-primary btn-sm"
              :disabled="!bookmarkTitle.trim() || !bookmarkUrl.trim()"
              @click="submitBookmark"
            >
              添加
            </button>
          </div>
          <div v-if="!chat.activeBookmarks().length" class="msg-search-hint">添加常用文档、看板链接</div>
          <ul v-else class="msg-search-list">
            <li v-for="b in chat.activeBookmarks()" :key="b.id">
              <div class="pin-row">
                <a class="msg-search-item bookmark-link" :href="b.url" target="_blank" rel="noopener">
                  <span class="msg-search-meta">{{ b.title }}</span>
                  <span class="msg-search-snippet">{{ b.url }}</span>
                </a>
                <button
                  type="button"
                  class="btn btn-ghost btn-sm pin-unpin"
                  title="删除书签"
                  @click="chat.deleteBookmark(b.id)"
                >
                  删除
                </button>
              </div>
            </li>
          </ul>
        </div>
      </Transition>

      <Transition name="panel-slide">
        <div v-if="showMentions" class="msg-search-panel pins-panel">
          <div class="msg-search-bar">
            <span class="pins-panel-title">未读 @提及（{{ chat.mentionInbox.length }}）</span>
            <button type="button" class="btn btn-ghost btn-sm" @click="toggleMentionsPanel">关闭</button>
          </div>
          <div v-if="!chat.mentionInbox.length" class="msg-search-hint">暂无未读提及</div>
          <ul v-else class="msg-search-list">
            <li v-for="m in chat.mentionInbox" :key="m.client_msg_id">
              <button
                type="button"
                class="msg-search-item"
                @click="showMentions = false; jumpToSearchResult(m)"
              >
                <span class="msg-search-meta">{{ senderLabel(m) }} · {{ formatMessageTime(m.created_at) }}</span>
                <span class="msg-search-snippet">{{ previewMessage(m.content, m.msg_type) }}</span>
              </button>
            </li>
          </ul>
        </div>
      </Transition>

      <Transition name="panel-slide">
        <div v-if="showStars" class="msg-search-panel pins-panel">
          <div class="msg-search-bar">
            <span class="pins-panel-title">我的收藏（{{ chat.starredList.length }}）</span>
            <button type="button" class="btn btn-ghost btn-sm" @click="toggleStarsPanel">关闭</button>
          </div>
          <div v-if="!chat.starredList.length" class="msg-search-hint">悬停消息点「收藏」保存稍后查看</div>
          <ul v-else class="msg-search-list">
            <li v-for="s in chat.starredList" :key="String(s.message.msg_id || s.message.client_msg_id)">
              <div class="pin-row">
                <button type="button" class="msg-search-item" @click="jumpToSearchResult(s.message)">
                  <span class="msg-search-meta">
                    {{ senderLabel(s.message) }} · {{ formatMessageTime(s.message.created_at) }}
                  </span>
                  <span class="msg-search-snippet">{{ previewMessage(s.message.content, s.message.msg_type) }}</span>
                </button>
                <button
                  type="button"
                  class="btn btn-ghost btn-sm pin-unpin"
                  @click="chat.toggleStar(s.message)"
                >
                  取消
                </button>
              </div>
            </li>
          </ul>
        </div>
      </Transition>

      <Transition name="panel-slide">
        <div v-if="chat.activeConvId && showSchedule" class="msg-search-panel pins-panel">
          <div class="msg-search-bar">
            <span class="pins-panel-title">定时发送</span>
            <button type="button" class="btn btn-ghost btn-sm" @click="toggleSchedulePanel">关闭</button>
          </div>
          <div class="bookmark-form">
            <input v-model="scheduleAtLocal" class="msg-search-input" type="datetime-local" />
            <button
              type="button"
              class="btn btn-primary btn-sm"
              :disabled="!input.trim() || !scheduleAtLocal"
              @click="submitSchedule"
            >
              定时发送当前输入
            </button>
          </div>
          <div v-if="!scheduledItems.length" class="msg-search-hint">输入内容后选择时间即可定时发出</div>
          <ul v-else class="msg-search-list">
            <li v-for="item in scheduledItems" :key="item.id">
              <div class="pin-row">
                <div class="msg-search-item">
                  <span class="msg-search-meta">{{ formatMessageTime(item.send_at) }}</span>
                  <span class="msg-search-snippet">{{ item.content }}</span>
                </div>
                <button type="button" class="btn btn-ghost btn-sm pin-unpin" @click="cancelScheduledItem(item.id)">
                  取消
                </button>
              </div>
            </li>
          </ul>
        </div>
      </Transition>

      <Transition name="panel-slide">
        <div v-if="showReminders" class="msg-search-panel pins-panel">
          <div class="msg-search-bar">
            <span class="pins-panel-title">稍后提醒（{{ chat.reminderList.length }}）</span>
            <button type="button" class="btn btn-ghost btn-sm" @click="toggleRemindersPanel">关闭</button>
          </div>
          <div v-if="!chat.reminderList.length" class="msg-search-hint">悬停消息点「提醒」设置稍后处理</div>
          <ul v-else class="msg-search-list">
            <li v-for="item in chat.reminderList" :key="item.id">
              <div class="pin-row">
                <button type="button" class="msg-search-item" @click="jumpToReminder(item)">
                  <span class="msg-search-meta">{{ formatMessageTime(item.remind_at) }}</span>
                  <span class="msg-search-snippet">{{ item.preview || '消息提醒' }}</span>
                </button>
                <button
                  type="button"
                  class="btn btn-ghost btn-sm pin-unpin"
                  @click="chat.cancelReminder(item.id)"
                >
                  取消
                </button>
              </div>
            </li>
          </ul>
        </div>
      </Transition>

      <Transition name="panel-slide">
        <div v-if="chat.activeConvId && showMedia" class="msg-search-panel pins-panel">
          <div class="msg-search-bar">
            <span class="pins-panel-title">媒体库</span>
            <button type="button" class="btn btn-ghost btn-sm" @click="toggleMediaPanel">关闭</button>
          </div>
          <div class="hashtag-chip-row">
            <button
              v-for="k in [
                { id: 'all', label: '全部' },
                { id: 'image', label: '图片' },
                { id: 'file', label: '文件' },
                { id: 'voice', label: '语音' },
              ]"
              :key="k.id"
              type="button"
              class="hashtag-chip"
              :class="{ active: chat.mediaKind === k.id }"
              @click="chat.loadMedia(k.id)"
            >
              {{ k.label }}
            </button>
          </div>
          <div v-if="chat.mediaLoading" class="msg-search-hint">加载中…</div>
          <div v-else-if="!chat.mediaList.length" class="msg-search-hint">暂无媒体消息</div>
          <div v-else class="media-gallery">
            <template v-for="m in chat.mediaList" :key="m.client_msg_id || String(m.msg_id)">
              <button
                v-if="m.msg_type === 2 || parseFileContent(m.content)?.content_type?.startsWith('image/')"
                type="button"
                class="media-thumb"
                @click="openImagePreview(parseFileContent(m.content)?.url || '')"
              >
                <img
                  :src="fileUrl(parseFileContent(m.content)!.url)"
                  alt=""
                  loading="lazy"
                />
              </button>
              <a
                v-else-if="m.msg_type === 3 || parseFileContent(m.content)"
                class="media-file-row"
                :href="fileUrl(parseFileContent(m.content)!.url)"
                target="_blank"
                rel="noopener noreferrer"
              >
                <span class="media-file-name">{{ parseFileContent(m.content)?.filename || '文件' }}</span>
                <span class="msg-search-meta">{{ formatMessageTime(m.created_at) }}</span>
              </a>
              <div v-else-if="m.msg_type === 5" class="media-file-row">
                <span class="media-file-name">语音消息</span>
                <span class="msg-search-meta">{{ formatMessageTime(m.created_at) }}</span>
              </div>
            </template>
          </div>
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
          v-if="showGroupNoticeBar"
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
          <button type="button" class="group-notice-dismiss" aria-label="关闭公告" @click="dismissActiveNotice">
            ×
          </button>
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
              :id="item.variant === 'unread' ? 'unread-sep' : undefined"
              :data-unread-sep="item.variant === 'unread' ? '1' : undefined"
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
                  v-if="!selectMode && (isTextMessage(item.message) || canRecall(item.message) || canEdit(item.message) || canReply(item.message) || item.message.msg_id)"
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
                    v-if="isTextMessage(item.message) && item.message.msg_id"
                    type="button"
                    class="msg-action-btn"
                    :disabled="chat.translatingMsgId === idStr(item.message.msg_id)"
                    @click="chat.translateMessage(item.message)"
                  >{{ chat.translatingMsgId === idStr(item.message.msg_id) ? '翻译中…' : '翻译' }}</button>
                  <button
                    v-if="canRecall(item.message)"
                    type="button"
                    class="msg-action-btn"
                    @click="recallMsg(item.message)"
                  >撤回</button>
                  <button
                    v-if="canEdit(item.message)"
                    type="button"
                    class="msg-action-btn"
                    @click="startEdit(item.message)"
                  >编辑</button>
                  <button
                    v-if="item.message.msg_id && item.message.msg_type !== 4"
                    type="button"
                    class="msg-action-btn"
                    :class="{ active: chat.isStarred(item.message.msg_id) }"
                    @click="chat.toggleStar(item.message)"
                  >{{ chat.isStarred(item.message.msg_id) ? '已藏' : '收藏' }}</button>
                  <button
                    v-if="item.message.msg_id && item.message.msg_type !== 4"
                    type="button"
                    class="msg-action-btn"
                    title="1 小时后提醒"
                    @click="remindIn(item.message, 60)"
                  >提醒</button>
                  <button
                    v-if="item.message.msg_id && item.message.msg_type !== 4"
                    type="button"
                    class="msg-action-btn"
                    title="明天 9:00 提醒"
                    @click="remindTomorrowMorning(item.message)"
                  >明早</button>
                  <button
                    v-if="item.message.msg_id && item.message.msg_type !== 4"
                    type="button"
                    class="msg-action-btn"
                    title="表情回应"
                    @click="toggleReactPicker(item.message.client_msg_id)"
                  >表情</button>
                  <button
                    v-if="item.message.msg_id && item.message.msg_type !== 4"
                    type="button"
                    class="msg-action-btn"
                    :class="{ active: chat.isMsgPinned(item.message.msg_id) }"
                    :title="chat.isMsgPinned(item.message.msg_id) ? '取消置顶' : '置顶'"
                    @click="chat.togglePin(item.message)"
                  >{{ chat.isMsgPinned(item.message.msg_id) ? '取消置顶' : '置顶' }}</button>
                </div>
                <template v-if="parseFileContent(item.message.content) || item.message.localPreview">
                  <div v-if="item.message.msg_type === 5" class="voice-msg">
                    <audio
                      class="voice-audio"
                      controls
                      preload="metadata"
                      :src="voiceMeta(item.message).url"
                    />
                    <span class="voice-dur">{{ voiceMeta(item.message).duration }}″</span>
                  </div>
                  <div
                    v-else-if="item.message.msg_type === 2 || (item.message.localPreview && item.message.msg_type !== 5) || parseFileContent(item.message.content)?.content_type?.startsWith('image/')"
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
                  <div v-else-if="item.message.msg_type === 6 && parsePoll(item.message.content)" class="poll-card">
                    <div class="poll-q">{{ parsePoll(item.message.content)!.question }}</div>
                    <button
                      v-for="opt in parsePoll(item.message.content)!.options"
                      :key="opt.id"
                      type="button"
                      class="poll-opt"
                      :class="{ mine: chat.pollFor(item.message.msg_id)?.my_option_id === opt.id }"
                      :disabled="!item.message.msg_id"
                      @click="chat.votePoll(item.message, opt.id)"
                    >
                      <span class="poll-opt-text">{{ opt.text }}</span>
                      <span class="poll-opt-meta">{{ pollCount(item.message.msg_id, opt.id) }} · {{ pollPct(item.message.msg_id, opt.id) }}%</span>
                      <span
                        class="poll-opt-bar"
                        :style="{ width: `${pollPct(item.message.msg_id, opt.id)}%` }"
                      />
                    </button>
                    <div class="poll-total">共 {{ chat.pollFor(item.message.msg_id)?.total || 0 }} 票</div>
                  </div>
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
                    <div v-if="editingMsgId === item.message.client_msg_id" class="msg-edit-box">
                      <textarea
                        v-model="editDraft"
                        class="msg-edit-input"
                        rows="3"
                        @keydown.esc.prevent="cancelEdit"
                        @keydown.enter.exact.prevent="saveEdit(item.message)"
                      />
                      <div class="msg-edit-actions">
                        <button type="button" class="btn btn-ghost btn-sm" @click="cancelEdit">取消</button>
                        <button
                          type="button"
                          class="btn btn-primary btn-sm"
                          :disabled="!editDraft.trim()"
                          @click="saveEdit(item.message)"
                        >保存</button>
                      </div>
                    </div>
                    <template v-else>
                      <span
                        class="msg-text"
                        v-html="renderMessageHtml(item.message.content)"
                        @click="onMsgTextClick"
                      />
                      <span v-if="item.message.edited_at" class="msg-edited" title="已编辑">已编辑</span>
                      <div
                        v-if="item.message.msg_id && chat.translationFor(item.message.msg_id)"
                        class="msg-translation"
                      >
                        <div class="msg-translation-label">
                          翻译
                          <button
                            type="button"
                            class="msg-translation-clear"
                            @click="chat.clearTranslation(item.message.msg_id)"
                          >
                            关闭
                          </button>
                        </div>
                        <div class="msg-translation-text">
                          {{ chat.translationFor(item.message.msg_id)!.text }}
                        </div>
                      </div>
                      <LinkPreviewCard
                        v-if="messageLink(item.message)"
                        :url="messageLink(item.message)!"
                      />
                    </template>
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
                <button
                  v-else-if="
                    chat.activeGroupId &&
                    item.showTime &&
                    sameId(item.message.from_user_id, auth.user?.id) &&
                    item.message.seq &&
                    chat.groupPeerCount(chat.activeConvId) > 0
                  "
                  type="button"
                  class="group-read-btn"
                  :title="`已读 ${chat.groupReadCount(chat.activeConvId, item.message.seq)}/${chat.groupPeerCount(chat.activeConvId)}`"
                  @click.stop="toggleGroupReadPopup(item.message.client_msg_id)"
                >
                  已读 {{ chat.groupReadCount(chat.activeConvId, item.message.seq) }}
                </button>
                <span
                  v-else-if="item.showTime && sameId(item.message.from_user_id, auth.user?.id) && item.message.status !== 'failed' && !chat.activeGroupId"
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
                <div
                  v-if="groupReadPopupFor === item.message.client_msg_id"
                  class="group-read-popup"
                  @click.stop
                >
                  <p class="group-read-title">
                    已读 {{ chat.groupReadCount(chat.activeConvId, item.message.seq) }}/{{ chat.groupPeerCount(chat.activeConvId) }}
                  </p>
                  <div
                    v-if="chat.groupReadMembers(chat.activeConvId, item.message.seq).length"
                    class="group-read-grid"
                  >
                    <button
                      v-for="m in chat.groupReadMembers(chat.activeConvId, item.message.seq)"
                      :key="m.user_id"
                      type="button"
                      class="group-read-person"
                      :title="groupReadName(m.user_id)"
                      @click="openGroupReadProfile(m.user_id)"
                    >
                      <UserAvatar
                        :src="groupReadAvatar(m.user_id)"
                        :name="groupReadName(m.user_id)"
                        :size="28"
                      />
                      <span class="group-read-person-name">{{ groupReadName(m.user_id) }}</span>
                    </button>
                  </div>
                  <p v-else class="group-read-empty">暂无人已读</p>
                  <template v-if="chat.groupUnreadMembers(chat.activeConvId, item.message.seq).length">
                    <p class="group-read-title group-read-title-unread">未读</p>
                    <div class="group-read-grid">
                      <button
                        v-for="m in chat.groupUnreadMembers(chat.activeConvId, item.message.seq)"
                        :key="`u-${m.user_id}`"
                        type="button"
                        class="group-read-person dim"
                        :title="groupReadName(m.user_id)"
                        @click="openGroupReadProfile(m.user_id)"
                      >
                        <UserAvatar
                          :src="groupReadAvatar(m.user_id)"
                          :name="groupReadName(m.user_id)"
                          :size="28"
                        />
                        <span class="group-read-person-name">{{ groupReadName(m.user_id) }}</span>
                      </button>
                    </div>
                  </template>
                </div>
                <div
                  v-if="reactPickerFor === item.message.client_msg_id"
                  class="react-picker"
                  @click.stop
                >
                  <button
                    v-for="em in REACTION_EMOJIS"
                    :key="em"
                    type="button"
                    class="react-pick-btn"
                    @click="onPickReaction(item.message, em)"
                  >{{ em }}</button>
                </div>
                <div
                  v-if="chat.reactionsFor(item.message.msg_id).length"
                  class="react-strip"
                  @click.stop
                >
                  <button
                    v-for="r in chat.reactionsFor(item.message.msg_id)"
                    :key="r.emoji"
                    type="button"
                    class="react-chip"
                    :class="{ mine: r.mine }"
                    @click="chat.toggleReaction(item.message, r.emoji)"
                  >
                    <span>{{ r.emoji }}</span>
                    <span class="react-count">{{ r.count }}</span>
                  </button>
                </div>
              </div>
            </div>
          </template>
        </div>
        <Transition name="fab">
          <div v-if="!nearBottom || hasUnreadMarker" class="jump-fabs">
            <button
              v-if="hasUnreadMarker"
              type="button"
              class="jump-latest-btn jump-unread-btn"
              @click="jumpToFirstUnread(true)"
            >
              跳到未读
            </button>
            <button
              v-if="!nearBottom"
              type="button"
              class="jump-latest-btn"
              @click="jumpToLatest"
            >
              {{ pendingNewCount > 0 ? `${pendingNewCount > 99 ? '99+' : pendingNewCount} 条新消息` : '回到底部' }}
            </button>
          </div>
        </Transition>
        </div>
        <div class="composer-stack">
          <Transition name="stack">
            <div v-if="selectMode" class="select-bar" role="toolbar" aria-label="多选操作">
              <span>已选 {{ selectedCount }} 条</span>
              <div class="select-bar-actions">
                <button type="button" class="btn btn-ghost btn-sm" @click="selectAllVisibleMessages">
                  全选
                </button>
                <button
                  type="button"
                  class="btn btn-ghost btn-sm"
                  :disabled="!selectedCount"
                  @click="clearSelectedMessages"
                >
                  清空
                </button>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="!selectedStarrableCount"
                  @click="starSelectedMessages"
                >
                  收藏
                </button>
                <button
                  type="button"
                  class="btn btn-primary btn-sm"
                  :disabled="!selectedCount"
                  @click="openForwardPicker"
                >
                  转发
                </button>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="!selectedCount"
                  @click="copySelectedMessages"
                >
                  复制
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
            <div
              v-if="mentionOpen && (mentionShowAll || mentionCandidates.length)"
              class="mention-panel"
              role="listbox"
            >
              <button
                v-if="mentionShowAll"
                type="button"
                class="mention-item mention-all"
                role="option"
                @mousedown.prevent="insertMentionAll"
              >
                <span class="mention-all-icon" aria-hidden="true">＠</span>
                <span class="mention-name">所有人</span>
                <span class="mention-user">通知群内全部成员</span>
              </button>
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
            <div v-if="showPollComposer" class="poll-composer" role="dialog" aria-label="发起投票">
              <input v-model="pollQuestion" class="input" type="text" maxlength="120" placeholder="投票问题" />
              <input
                v-for="(_, i) in pollOptions"
                :key="i"
                v-model="pollOptions[i]"
                class="input"
                type="text"
                maxlength="64"
                :placeholder="`选项 ${i + 1}`"
              />
              <div class="poll-composer-actions">
                <button type="button" class="btn btn-ghost btn-sm" :disabled="pollOptions.length >= 8" @click="addPollOption">
                  加选项
                </button>
                <button type="button" class="btn btn-ghost btn-sm" @click="showPollComposer = false">取消</button>
                <button
                  type="button"
                  class="btn btn-primary btn-sm"
                  :disabled="!pollQuestion.trim() || pollOptions.filter((t) => t.trim()).length < 2"
                  @click="submitPoll"
                >
                  发送投票
                </button>
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
            :disabled="chat.uploading || recording"
            @click="pickFile"
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
              <path d="M21.4 11.6l-8.5 8.5a5 5 0 01-7.1-7.1l8.5-8.5a3.2 3.2 0 014.5 4.5L10.2 17.6a1.4 1.4 0 01-2-2l7.4-7.4" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
          </button>
          <button
            v-if="!recording"
            type="button"
            class="btn btn-secondary btn-sm composer-icon-btn"
            aria-label="按住说话"
            title="点击开始录音，最长 60 秒"
            :disabled="chat.uploading || !chat.activeConvId"
            @click="startVoiceRecord"
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
              <rect x="9" y="2" width="6" height="12" rx="3" />
              <path d="M5 11a7 7 0 0014 0M12 18v3" stroke-linecap="round" />
            </svg>
          </button>
          <button
            v-if="!recording"
            type="button"
            class="btn btn-secondary btn-sm composer-icon-btn"
            aria-label="发起投票"
            title="发起投票"
            :aria-pressed="showPollComposer"
            :disabled="chat.uploading || !chat.activeConvId"
            @click="togglePollComposer"
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
              <path d="M4 19V5M4 19h16M8 15v-4M12 15V8M16 15v-6" stroke-linecap="round" />
            </svg>
          </button>
          <div v-else class="voice-rec-bar">
            <span class="voice-rec-dot" aria-hidden="true" />
            <span>录音中 {{ recordSecs }}s</span>
            <button type="button" class="btn btn-ghost btn-sm" @click="cancelVoiceRecord">取消</button>
            <button type="button" class="btn btn-primary btn-sm" @click="stopVoiceRecord">发送</button>
          </div>
          <textarea
            v-show="!recording"
            ref="composerInputEl"
            v-model="input"
            class="input composer-input"
            rows="1"
            :placeholder="
              chat.activeGroupId && !chat.canPostInActiveGroup
                ? chat.activeGroupPostBlockReason
                : chat.activeGroupId
                  ? '发消息… Enter 发送，Shift+Enter 换行'
                  : '发消息… Enter 发送，Shift+Enter 换行'
            "
            :disabled="!!chat.activeGroupId && !chat.canPostInActiveGroup"
            aria-label="消息输入框"
            @input="onComposerInput"
            @keyup="updateMentionState"
            @click="updateMentionState"
            @keydown="onComposerKeydown"
            @paste="onComposerPaste"
          />
          <button
            v-show="!recording"
            type="button"
            class="btn btn-primary send-btn"
            :class="{ sending: sendingLock }"
            :disabled="!input.trim() || chat.uploading || sendingLock || (!!chat.activeGroupId && !chat.canPostInActiveGroup)"
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

    <Teleport to="body">
      <Transition name="fade">
        <div v-if="forwardOpen" class="forward-overlay" @click.self="closeForwardPicker">
          <div class="forward-card" role="dialog" aria-modal="true" aria-label="选择转发对象">
            <header class="forward-head">
              <h3>转发给</h3>
              <button type="button" class="btn btn-ghost btn-sm" @click="closeForwardPicker">关闭</button>
            </header>
            <input
              v-model="forwardQuery"
              class="input forward-search"
              type="search"
              placeholder="搜索好友或群聊"
            />
            <div class="forward-list">
              <p class="forward-label">好友</p>
              <button
                v-for="f in forwardFriends"
                :key="'f-' + f.id"
                type="button"
                class="forward-item"
                :disabled="forwarding"
                @click="forwardToFriend(f)"
              >
                <UserAvatar :src="friendAvatarUrl(f)" :name="friendDisplayName(f)" :size="36" />
                <span>{{ friendDisplayName(f) }}</span>
              </button>
              <p v-if="!forwardFriends.length" class="forward-empty">无匹配好友</p>
              <p class="forward-label">群聊</p>
              <button
                v-for="g in forwardGroups"
                :key="'g-' + g.id"
                type="button"
                class="forward-item"
                :disabled="forwarding"
                @click="forwardToGroup(g)"
              >
                <UserAvatar :name="g.name" :size="36" />
                <span>{{ g.name }}</span>
              </button>
              <p v-if="!forwardGroups.length" class="forward-empty">无匹配群聊</p>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

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
          v-if="isTextMessage(msgMenu.message) && msgMenu.message.msg_id"
          type="button"
          class="ctx-item"
          role="menuitem"
          @click="msgMenuTranslate()"
        >翻译</button>
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
      <template v-if="chat.folders.length">
        <div class="ctx-sep" role="separator" />
        <button
          v-for="f in chat.folders"
          :key="f.id"
          type="button"
          class="ctx-item"
          role="menuitem"
          @click="ctxAssignFolder(f.id)"
        >
          移入「{{ f.name }}」
        </button>
        <button type="button" class="ctx-item" role="menuitem" @click="ctxRemoveFromFolders">移出文件夹</button>
      </template>
      <button type="button" class="ctx-item" role="menuitem" @click="ctxOpenProfile">
        {{ ctxMenu.kind === 'friend' ? '查看资料' : '群聊信息' }}
      </button>
      </div>
    </Transition>

    <Transition name="fade">
      <div
        v-if="draftConflict"
        class="confirm-backdrop"
        role="presentation"
      >
        <div class="confirm-card draft-conflict-card" role="dialog" aria-modal="true" aria-label="草稿冲突">
          <h3 class="confirm-title">草稿冲突</h3>
          <p class="confirm-body">
            「{{ draftConflictTitle(draftConflict.convId) }}」的本地与云端草稿不一致，请选择保留哪一版。
            <span v-if="draftConflict.rest.length" class="draft-conflict-rest">
              （还有 {{ draftConflict.rest.length }} 处待处理）
            </span>
          </p>
          <div class="draft-conflict-cols">
            <div class="draft-conflict-col">
              <div class="draft-conflict-label">本地</div>
              <p class="draft-conflict-snippet">{{ previewDraftSnippet(draftConflict.local) }}</p>
            </div>
            <div class="draft-conflict-col">
              <div class="draft-conflict-label">云端</div>
              <p class="draft-conflict-snippet">{{ previewDraftSnippet(draftConflict.remote) }}</p>
            </div>
          </div>
          <div class="confirm-actions">
            <button type="button" class="btn btn-secondary btn-sm" @click="applyDraftConflict('cloud')">
              使用云端
            </button>
            <button type="button" class="btn btn-primary btn-sm" @click="applyDraftConflict('local')">
              保留本地
            </button>
          </div>
        </div>
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
  display: flex;
  gap: 6px;
  align-items: center;
}

.sidebar-search .input {
  flex: 1;
  min-width: 0;
}

.sidebar-search-btn {
  flex-shrink: 0;
}

.global-search-panel {
  max-height: 180px;
  overflow-y: auto;
  margin: var(--space-2) var(--space-4) 0;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-bg-surface);
  flex-shrink: 0;
}

.global-search-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 8px;
  font-size: 12px;
  color: var(--color-text-muted);
  border-bottom: 1px solid var(--color-border);
}

.global-search-item {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 2px;
  text-align: left;
  border: none;
  background: transparent;
  padding: 8px;
  cursor: pointer;
  border-bottom: 1px solid var(--color-border);
}

.global-search-item:hover {
  background: var(--color-bg-sidebar);
}

.gs-conv {
  font-size: 12px;
  font-weight: 600;
}

.gs-body {
  font-size: 12px;
  color: var(--color-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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

.folder-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: var(--space-2) var(--space-4) 0;
  flex-shrink: 0;
}

.folder-chip {
  border: 1px solid var(--color-border);
  background: transparent;
  color: var(--color-text-secondary);
  font-size: 12px;
  padding: 4px 10px;
  border-radius: var(--radius-sm, 6px);
  cursor: pointer;
  max-width: 7.5rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.folder-chip.active {
  border-color: var(--color-primary);
  color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 10%, transparent);
}

.folder-chip-manage {
  margin-left: auto;
}

.folder-manage {
  margin: var(--space-2) var(--space-4) 0;
  padding: var(--space-2);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm, 6px);
  flex-shrink: 0;
}

.folder-manage-row {
  display: flex;
  gap: 6px;
}

.folder-manage-input {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  padding: 6px 8px;
}

.folder-manage-list {
  list-style: none;
  margin: 8px 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.folder-manage-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 12px;
  color: var(--color-text-secondary);
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

.ctx-sep {
  height: 1px;
  margin: 4px 0;
  background: var(--color-border);
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
  position: relative;
  border: none;
  background: transparent;
  padding: 0;
  cursor: pointer;
  flex-shrink: 0;
}

.online-dot {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #94a3b8;
  border: 2px solid var(--color-bg-surface);
  box-sizing: border-box;
}

.online-dot.on {
  background: #22c55e;
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

.group-read-btn {
  border: none;
  background: transparent;
  color: var(--color-text-muted);
  font-size: 11px;
  cursor: pointer;
  padding: 0 2px;
  margin-left: 4px;
}

.group-read-btn:hover {
  color: var(--color-primary);
}

.group-read-popup {
  position: absolute;
  right: 8px;
  bottom: calc(100% - 4px);
  z-index: 6;
  min-width: 168px;
  max-width: 240px;
  max-height: 240px;
  overflow-y: auto;
  padding: 10px;
  background: var(--color-bg-surface);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  box-shadow: 0 8px 20px rgba(15, 23, 42, 0.12);
}

.group-read-title {
  margin: 0 0 8px;
  font-size: 12px;
  font-weight: 600;
}

.group-read-title-unread {
  margin-top: 10px;
  color: var(--color-text-muted);
  font-weight: 500;
}

.group-read-grid {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.group-read-person {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 4px 6px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  cursor: pointer;
  text-align: left;
  color: inherit;
}

.group-read-person:hover {
  background: var(--color-primary-muted, color-mix(in srgb, var(--color-primary) 12%, transparent));
}

.group-read-person.dim {
  opacity: 0.72;
}

.group-read-person-name {
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.group-read-empty {
  margin: 0;
  font-size: 12px;
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

.friend-status {
  flex: 1;
  min-width: 0;
  font-size: 11px;
  color: var(--color-text-muted);
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

.pins-panel-title {
  flex: 1;
  font-size: var(--text-sm);
  font-weight: 600;
  color: var(--color-text);
}

.pin-row {
  display: flex;
  align-items: stretch;
  gap: 4px;
}

.pin-row .msg-search-item {
  flex: 1;
  min-width: 0;
}

.pin-unpin {
  flex-shrink: 0;
  align-self: center;
  margin-right: var(--space-4);
}

.bookmark-form {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  padding: 0 var(--space-6) var(--space-3);
}

.bookmark-form .msg-search-input {
  flex: 1 1 140px;
}

.bookmark-link {
  text-decoration: none;
  color: inherit;
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

.msg-search-item.active {
  background: color-mix(in srgb, var(--color-primary) 14%, transparent);
}

.search-nav-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0 var(--space-6) var(--space-2);
}

.search-nav-label {
  font-size: 12px;
  color: var(--color-text-muted);
  font-weight: 600;
}

.search-nav-actions {
  display: flex;
  gap: 4px;
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

.group-notice-dismiss {
  flex-shrink: 0;
  border: none;
  background: transparent;
  color: #0f766e;
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
  padding: 0 4px;
  opacity: 0.7;
}

.group-notice-dismiss:hover {
  opacity: 1;
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

.jump-fabs {
  position: absolute;
  right: var(--space-6);
  bottom: var(--space-4);
  z-index: 5;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
}

.jump-latest-btn {
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

.jump-unread-btn {
  background: var(--color-bg-surface);
  color: var(--color-primary);
  border: 1px solid var(--color-primary);
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.1);
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
  flex-wrap: wrap;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-6);
  font-size: var(--text-sm);
  color: var(--color-text);
  background: var(--color-bg-surface);
  border-top: 1px solid var(--color-border);
}

.select-bar-actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  justify-content: flex-end;
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

.msg-action-btn.active {
  color: var(--color-primary);
  font-weight: 600;
}

.msg-edited {
  margin-left: 6px;
  font-size: 10px;
  color: var(--color-text-muted);
}

.msg-translation {
  margin-top: 6px;
  padding-top: 6px;
  border-top: 1px dashed var(--color-border);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.msg-translation-label {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 2px;
  font-size: 10px;
  letter-spacing: 0.04em;
}

.msg-translation-clear {
  border: none;
  background: transparent;
  color: var(--color-primary);
  font: inherit;
  font-size: 10px;
  cursor: pointer;
  padding: 0;
}

.msg-translation-text {
  color: var(--color-text);
  white-space: pre-wrap;
  word-break: break-word;
}

.msg-edit-box {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 200px;
}

.msg-edit-input {
  width: 100%;
  min-height: 64px;
  resize: vertical;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  padding: 6px 8px;
  font: inherit;
  background: var(--color-bg-chat);
}

.msg-edit-actions {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
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

.mention-all-icon {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: rgba(13, 148, 136, 0.15);
  color: #0f766e;
  font-size: 14px;
  font-weight: 700;
  flex-shrink: 0;
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

:deep(.msg-text .msg-hashtag) {
  display: inline;
  padding: 0;
  margin: 0;
  border: 0;
  background: transparent;
  color: var(--color-primary);
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}

:deep(.msg-text .msg-hashtag:hover) {
  text-decoration: underline;
}

:deep(.msg-text .msg-link) {
  color: var(--color-primary);
  text-decoration: underline;
  word-break: break-all;
}

:deep(.msg-text .msg-link:hover) {
  opacity: 0.85;
}

.hashtag-chip-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 0 12px 8px;
}

.hashtag-chip {
  border: 1px solid var(--color-border);
  background: transparent;
  color: var(--color-text-secondary);
  font-size: 12px;
  padding: 3px 8px;
  border-radius: var(--radius-sm, 6px);
  cursor: pointer;
}

.hashtag-chip.active {
  border-color: var(--color-primary);
  color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 10%, transparent);
}

.media-gallery {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(88px, 1fr));
  gap: 8px;
  padding: 0 12px 12px;
  max-height: 280px;
  overflow-y: auto;
}

.media-thumb {
  aspect-ratio: 1;
  padding: 0;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm, 6px);
  overflow: hidden;
  cursor: pointer;
  background: var(--color-bg-sidebar);
}

.media-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.media-file-row {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm, 6px);
  text-decoration: none;
  color: inherit;
  font-size: 13px;
}

.media-file-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
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

.voice-rec-bar {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  font-size: 13px;
  color: #b91c1c;
}

.voice-rec-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #ef4444;
  animation: voice-pulse 1s ease infinite;
}

@keyframes voice-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.35;
  }
}

.voice-msg {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 160px;
}

.voice-audio {
  width: 180px;
  max-width: 100%;
  height: 32px;
}

.voice-dur {
  font-size: 12px;
  color: var(--color-text-muted);
  flex-shrink: 0;
}

.poll-card {
  min-width: 220px;
  max-width: 320px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.poll-q {
  font-weight: 600;
  font-size: 14px;
  line-height: 1.35;
}

.poll-opt {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  width: 100%;
  text-align: left;
  padding: 8px 10px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm, 6px);
  background: var(--color-surface, transparent);
  cursor: pointer;
  font-size: 13px;
  overflow: hidden;
}

.poll-opt:hover:not(:disabled) {
  border-color: var(--color-primary, #3b82f6);
}

.poll-opt.mine {
  border-color: var(--color-primary, #3b82f6);
  background: color-mix(in srgb, var(--color-primary, #3b82f6) 12%, transparent);
}

.poll-opt-text {
  position: relative;
  z-index: 1;
  flex: 1;
  min-width: 0;
}

.poll-opt-meta {
  position: relative;
  z-index: 1;
  flex-shrink: 0;
  color: var(--color-text-muted);
  font-size: 12px;
}

.poll-opt-bar {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  background: color-mix(in srgb, var(--color-primary, #3b82f6) 18%, transparent);
  pointer-events: none;
  z-index: 0;
}

.poll-total {
  font-size: 12px;
  color: var(--color-text-muted);
}

.poll-composer {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  border-top: 1px solid var(--color-border);
  background: var(--color-bg-elevated, var(--color-surface));
}

.poll-composer-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
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

.react-picker {
  display: flex;
  gap: 2px;
  margin-top: 4px;
  padding: 4px;
  background: var(--color-bg-surface);
  border: 1px solid var(--color-border);
  border-radius: 999px;
  width: fit-content;
}

.react-pick-btn {
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 16px;
  line-height: 1;
  padding: 2px 4px;
  border-radius: 6px;
}

.react-pick-btn:hover {
  background: var(--color-bg-sidebar);
}

.react-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 4px;
}

.react-chip {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  border: 1px solid var(--color-border);
  background: var(--color-bg-surface);
  border-radius: 999px;
  padding: 1px 7px;
  font-size: 12px;
  cursor: pointer;
}

.react-chip.mine {
  border-color: #0d9488;
  background: rgba(13, 148, 136, 0.1);
}

.react-count {
  color: var(--color-text-muted);
  font-size: 11px;
}

.forward-overlay {
  position: fixed;
  inset: 0;
  z-index: 1250;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-4);
  background: rgba(15, 23, 42, 0.45);
}

.forward-card {
  width: 100%;
  max-width: 400px;
  max-height: min(72vh, 560px);
  display: flex;
  flex-direction: column;
  background: var(--color-bg-surface);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg, 0 16px 40px rgba(15, 23, 42, 0.18));
  overflow: hidden;
}

.forward-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  border-bottom: 1px solid var(--color-border);
}

.forward-head h3 {
  margin: 0;
  font-size: var(--text-base);
}

.forward-search {
  margin: 10px 14px 0;
  width: auto;
}

.forward-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px 8px 14px;
}

.forward-label {
  margin: 10px 8px 4px;
  font-size: 12px;
  color: var(--color-text-muted);
}

.forward-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  border: none;
  background: transparent;
  padding: 8px;
  border-radius: 8px;
  cursor: pointer;
  text-align: left;
  font-size: var(--text-sm);
  color: var(--color-text-primary);
}

.forward-item:hover:not(:disabled) {
  background: var(--color-bg-sidebar);
}

.forward-item:disabled {
  opacity: 0.6;
  cursor: wait;
}

.forward-empty {
  margin: 4px 8px 8px;
  font-size: 12px;
  color: var(--color-text-muted);
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

.draft-conflict-card {
  max-width: 420px;
}

.draft-conflict-rest {
  display: block;
  margin-top: 4px;
  color: var(--color-text-muted);
  font-size: 12px;
}

.draft-conflict-cols {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin: 0 0 var(--space-4);
}

.draft-conflict-col {
  padding: 8px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-bg-sidebar, rgba(15, 23, 42, 0.03));
  min-width: 0;
}

.draft-conflict-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-muted);
  margin-bottom: 4px;
}

.draft-conflict-snippet {
  margin: 0;
  font-size: 12px;
  line-height: 1.4;
  word-break: break-word;
  color: var(--color-text);
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
