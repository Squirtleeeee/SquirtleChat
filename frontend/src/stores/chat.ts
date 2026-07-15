import { defineStore } from 'pinia'
import http, { unwrapApiData } from '../api/http'
import { ApiError } from '../api/errors'
import type { AckPayload, WSFrame, WSStatus } from '../api/ws'
import { useAuthStore, type PublicProfile } from './auth'
import { useSettingsStore } from './settings'
import { directConvId, idStr, sameId, type UserId } from '../utils/id'
import { isAgentProfile, AGENT_NICKNAME } from '../constants/agent'
import { previewMessage } from '../utils/format'
import { buildReplyContent, parseReplyContent, type ReplyMeta } from '../utils/reply'
import { showMessageNotification } from '../utils/notify'

function escapeRegExp(s: string) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

export type { PublicProfile }

export type ChatMessage = {
  msg_id?: number
  client_msg_id: string
  conversation_id: string
  from_user_id: UserId
  seq?: number
  msg_type: number
  content: string
  created_at?: string
  edited_at?: string
  status?: 'sending' | 'sent' | 'failed' | 'uploading'
  /** 0–100 while uploading */
  uploadProgress?: number
  /** blob: URL for local preview before upload finishes */
  localPreview?: string
}

export type ReactionSummary = {
  emoji: string
  count: number
  mine?: boolean
}

export type PinnedItem = {
  message: ChatMessage
  pinned_by: string
  pinned_at?: string
}

export type ConversationBookmark = {
  id: string
  conversation_id: string
  title: string
  url: string
  created_by: string
  created_at?: string
}

export const REACTION_EMOJIS = ['👍', '❤️', '😂', '😮', '😢', '🎉'] as const

export type ConversationItem = {
  conversation_id: string
  type: number
  title?: string
  last_seq: number
  last_read_seq: number
  unread_count: number
  last_content: string
  last_msg_type?: number
  updated_at?: string
}

export type FriendWithConv = PublicProfile & {
  lastPreview?: string
  updatedAt?: string
}

export type GroupWithConv = GroupItem & {
  lastPreview?: string
  updatedAt?: string
}

export type GroupItem = {
  id: string
  name: string
  group_no?: string
  owner_id: string
  conversation_id: string
  notice?: string
  welcome_text?: string
  admin_only?: boolean
  slow_mode_secs?: number
}

export type GroupPublicItem = {
  id: string
  name: string
  group_no: string
  member_count: number
  is_member?: boolean
}

export type GroupInvitationItem = {
  id: string
  group_id: string
  group_name: string
  group_no: string
  from_user_id: string
  from_name: string
  from_avatar?: string
  message: string
  invite_type: number
}

export type FilePayload = {
  url: string
  filename: string
  content_type: string
  size?: number
  /** seconds, for voice messages */
  duration?: number
}

export function friendDisplayName(f: PublicProfile & { remark?: string }) {
  if (isAgentProfile(f)) return f.remark?.trim() || AGENT_NICKNAME
  return f.remark?.trim() || f.nickname || f.username
}

export function parseFileContent(content: string): FilePayload | null {
  try {
    const o = JSON.parse(content) as FilePayload
    if (o?.url) return o
  } catch {
    /* plain text */
  }
  return null
}

function isNotFoundApi(e: unknown) {
  return e instanceof ApiError && e.message.includes('接口不存在')
}

const PINNED_KEY = 'squirtlechat_pinned'
const MUTED_KEY = 'squirtlechat_muted'

type PinnedState = { friends: string[]; groups: string[] }

function loadPinned(): PinnedState {
  try {
    const raw = localStorage.getItem(PINNED_KEY)
    if (!raw) return { friends: [], groups: [] }
    const parsed = JSON.parse(raw) as PinnedState | string[]
    if (Array.isArray(parsed)) return { friends: parsed, groups: [] }
    return { friends: parsed.friends || [], groups: parsed.groups || [] }
  } catch {
    return { friends: [], groups: [] }
  }
}

function savePinned(friends: string[], groups: string[]) {
  localStorage.setItem(PINNED_KEY, JSON.stringify({ friends, groups }))
}

function loadMuted(): string[] {
  try {
    const raw = localStorage.getItem(MUTED_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as string[]
    return Array.isArray(parsed) ? parsed.map(idStr) : []
  } catch {
    return []
  }
}

function saveMuted(ids: string[]) {
  localStorage.setItem(MUTED_KEY, JSON.stringify(ids))
}

const initialPinned = loadPinned()
const initialMuted = loadMuted()

export const useChatStore = defineStore('chat', {
  state: () => ({
    agentUserId: '' as UserId,
    agentLLMEnabled: false,
    friends: [] as PublicProfile[],
    groups: [] as GroupItem[],
    conversations: [] as ConversationItem[],
    activeConvId: '',
    activeToUser: '' as UserId,
    activeGroupId: '',
    activeTitle: '',
    messages: {} as Record<string, ChatMessage[]>,
    /** msg_id string -> reaction summaries */
    reactions: {} as Record<string, ReactionSummary[]>,
    /** msg_id -> translation cache */
    translations: {} as Record<string, { text: string; target_lang: string }>,
    translatingMsgId: '' as string,
    /** conversation_id -> pinned messages */
    pinsByConv: {} as Record<string, PinnedItem[]>,
    /** conversation_id -> bookmarks */
    bookmarksByConv: {} as Record<string, ConversationBookmark[]>,
    /** msg_id string -> starred */
    starredMsgIds: {} as Record<string, boolean>,
    starredList: [] as { message: ChatMessage; starred_at?: string }[],
    mentionInbox: [] as ChatMessage[],
    reminderList: [] as {
      id: string
      conversation_id: string
      msg_id: string
      preview: string
      remind_at: string
    }[],
    /** msg_id -> poll results */
    polls: {} as Record<
      string,
      { msg_id: string; total: number; counts: { option_id: string; count: number }[]; my_option_id?: string }
    >,
    /** user_id -> online */
    onlineMap: {} as Record<string, boolean>,
    pending: [] as {
      id: string
      from_user_id: UserId
      display_name: string
      avatar?: string
      message: string
    }[],
    groupInvitations: [] as GroupInvitationItem[],
    sinceSeq: 0,
    error: '',
    notice: '',
    sidebarTab: 'friends' as 'friends' | 'groups',
    wsBound: false,
    wsStatus: 'closed' as WSStatus,
    /** True after a disconnect until sync after reconnect finishes. */
    wasDisconnected: false,
    syncingAfterReconnect: false,
    reconnectAttempt: 0,
    noticeTimer: 0 as number,
    searchQuery: '',
    searchResults: [] as ChatMessage[],
    searchLoading: false,
    hashtagList: [] as { tag: string; count: number }[],
    activeHashtag: '',
    mediaList: [] as ChatMessage[],
    mediaKind: 'all' as string,
    mediaLoading: false,
    draftItems: [] as { conversation_id: string; content: string; updated_at?: string }[],
    globalSearchResults: [] as ChatMessage[],
    globalSearchLoading: false,
    highlightClientMsgId: '',
    highlightTimer: 0 as number,
    /** conversation_id -> peer user ids currently typing */
    typingUsers: {} as Record<string, string[]>,
    typingClearTimers: {} as Record<string, number>,
    lastTypingSentAt: 0,
    /** group_id -> members (for @mention) */
    groupMembers: {} as Record<string, PublicProfile[]>,
    groupMemberRoles: {} as Record<string, Record<string, number>>,
    groupMemberMuted: {} as Record<string, Record<string, boolean>>,
    groupMemberNicknames: {} as Record<string, Record<string, string>>,
    groupMemberRemarks: {} as Record<string, Record<string, string>>,
    historyHasMore: {} as Record<string, boolean>,
    peerReadSeq: {} as Record<string, number>,
    /** conversation_id -> per-member read seq (for group receipts) */
    memberReadState: {} as Record<string, { user_id: string; read_seq: number }[]>,
    pinnedFriendIds: initialPinned.friends,
    pinnedGroupIds: initialPinned.groups,
    mutedConvIds: initialMuted,
    folders: [] as { id: string; name: string; conversation_ids: string[] }[],
    activeFolderId: '' as string,
    uploading: false,
    uploadPercent: 0,
  }),
  getters: {
    activeGroupMembers(): PublicProfile[] {
      if (!this.activeGroupId) return []
      return this.groupMembers[this.activeGroupId] || []
    },
    isActiveMuted(): boolean {
      return !!this.activeConvId && this.mutedConvIds.includes(this.activeConvId)
    },
    activeGroupNotice(): string {
      if (!this.activeGroupId) return ''
      const g = this.groups.find((x) => x.id === this.activeGroupId)
      return (g?.notice || '').trim()
    },
    activeGroupAdminOnly(): boolean {
      if (!this.activeGroupId) return false
      const g = this.groups.find((x) => x.id === this.activeGroupId)
      return !!g?.admin_only
    },
    canPostInActiveGroup(): boolean {
      if (!this.activeGroupId) return true
      const auth = useAuthStore()
      const uid = idStr(auth.user?.id)
      if (this.groupMemberMuted[this.activeGroupId]?.[uid]) return false
      const role = this.groupMemberRoles[this.activeGroupId]?.[uid] ?? 0
      const g = this.groups.find((x) => x.id === this.activeGroupId)
      const isManager = !!(g && (sameId(g.owner_id, uid) || role >= 1))
      if (this.activeGroupAdminOnly && !isManager) return false
      const secs = g?.slow_mode_secs || 0
      if (secs > 0 && !isManager && this.activeConvId) {
        const list = this.messages[this.activeConvId] || []
        for (let i = list.length - 1; i >= 0; i--) {
          const m = list[i]
          if (!sameId(m.from_user_id, uid)) continue
          const t = m.created_at ? Date.parse(m.created_at) : 0
          if (t && Date.now() - t < secs * 1000) return false
          break
        }
      }
      return true
    },
    activeGroupPostBlockReason(): string {
      if (!this.activeGroupId || this.canPostInActiveGroup) return ''
      const auth = useAuthStore()
      const uid = idStr(auth.user?.id)
      if (this.groupMemberMuted[this.activeGroupId]?.[uid]) return '你已被禁言'
      if (this.activeGroupAdminOnly) return '全员禁言中，仅管理员可发言'
      const g = this.groups.find((x) => x.id === this.activeGroupId)
      const secs = g?.slow_mode_secs || 0
      if (secs > 0 && this.activeConvId) {
        const list = this.messages[this.activeConvId] || []
        for (let i = list.length - 1; i >= 0; i--) {
          const m = list[i]
          if (!sameId(m.from_user_id, uid)) continue
          const t = m.created_at ? Date.parse(m.created_at) : 0
          if (t) {
            const remain = Math.ceil((secs * 1000 - (Date.now() - t)) / 1000)
            if (remain > 0) return `慢速模式：请 ${remain} 秒后再发送`
          }
          break
        }
      }
      return '暂时无法发言'
    },
    sortedFriends(): FriendWithConv[] {
      const auth = useAuthStore()
      const myId = auth.user?.id || ''
      const list = [...this.friends]
        .map((f) => {
          const conv = this.conversations.find(
            (c) => c.conversation_id === directConvId(myId, f.id),
          )
          return {
            ...f,
            lastPreview: conv ? previewMessage(conv.last_content, conv.last_msg_type) : '',
            updatedAt: conv?.updated_at || '',
            unread: conv?.unread_count || 0,
          }
        })
        .sort((a, b) => {
          const aAgent = isAgentProfile(a) ? 2 : 0
          const bAgent = isAgentProfile(b) ? 2 : 0
          if (aAgent !== bAgent) return bAgent - aAgent
          const ap = this.pinnedFriendIds.includes(a.id) ? 1 : 0
          const bp = this.pinnedFriendIds.includes(b.id) ? 1 : 0
          if (ap !== bp) return bp - ap
          return (b.updatedAt || '').localeCompare(a.updatedAt || '')
        })
      return list
    },
    sortedGroups(): GroupWithConv[] {
      return [...this.groups]
        .map((g) => {
          const conv = this.conversations.find((c) => c.conversation_id === g.conversation_id)
          return {
            ...g,
            lastPreview: conv ? previewMessage(conv.last_content, conv.last_msg_type) : '',
            updatedAt: conv?.updated_at || '',
          }
        })
        .sort((a, b) => {
          const ap = this.pinnedGroupIds.includes(a.id) ? 1 : 0
          const bp = this.pinnedGroupIds.includes(b.id) ? 1 : 0
          if (ap !== bp) return bp - ap
          return (b.updatedAt || '').localeCompare(a.updatedAt || '')
        })
    },
    unreadForFriend: (s) => (fid: UserId) => {
      const auth = useAuthStore()
      const convId = directConvId(auth.user?.id || '', fid)
      if (convId === s.activeConvId) return 0
      const conv = s.conversations.find((c) => c.conversation_id === convId)
      return conv?.unread_count || 0
    },
    unreadForGroup: (s) => (convId: string) => {
      if (convId === s.activeConvId) return 0
      const conv = s.conversations.find((c) => c.conversation_id === convId)
      return conv?.unread_count || 0
    },
    activePeerReadSeq: (s) => {
      if (!s.activeConvId || s.activeGroupId) return 0
      return s.peerReadSeq[s.activeConvId] || 0
    },
    friendsUnread(): number {
      return this.conversations
        .filter(
          (c) =>
            c.type === 1 &&
            c.conversation_id !== this.activeConvId &&
            !this.mutedConvIds.includes(c.conversation_id),
        )
        .reduce((n, c) => n + (c.unread_count || 0), 0)
    },
    groupsUnread(): number {
      return this.conversations
        .filter(
          (c) =>
            c.type === 2 &&
            c.conversation_id !== this.activeConvId &&
            !this.mutedConvIds.includes(c.conversation_id),
        )
        .reduce((n, c) => n + (c.unread_count || 0), 0)
    },
    activeTypingLabel(): string {
      if (!this.activeConvId) return ''
      const ids = this.typingUsers[this.activeConvId] || []
      if (!ids.length) return ''
      if (this.activeGroupId) {
        const names = ids.map((uid) => {
          const f = this.friends.find((x) => sameId(x.id, uid))
          return f ? friendDisplayName(f) : '有人'
        })
        if (names.length === 1) return `${names[0]} 正在输入…`
        if (names.length === 2) return `${names[0]}、${names[1]} 正在输入…`
        return `${names.length} 人正在输入…`
      }
      return '对方正在输入…'
    },
  },
  actions: {
    setError(msg: string) {
      this.error = msg
      this.clearNotice()
    },
    setNotice(msg: string) {
      this.setTransientNotice(msg)
    },
    clearError() {
      this.error = ''
    },
    clearNotice() {
      if (this.noticeTimer) {
        window.clearTimeout(this.noticeTimer)
        this.noticeTimer = 0
      }
      this.notice = ''
    },
    bindWS() {
      const auth = useAuthStore()
      if (!auth.ws) return
      // Always re-attach: reconnectWS / connectWS may replace the client.
      auth.ws.on((frame: WSFrame) => this.onFrame(frame))
      auth.ws.onStatus((s) => {
        const prev = this.wsStatus
        this.wsStatus = s
        this.reconnectAttempt = auth.ws?.reconnectAttempt || 0
        if (s === 'closed' && prev === 'open') {
          this.wasDisconnected = true
        }
        if (s === 'open' && prev !== 'open' && this.wasDisconnected) {
          void this.onWSReconnected()
        }
      })
      if (!this.wsBound) {
        this.wsBound = true
        window.addEventListener('squirtle:token-refreshed', ((e: CustomEvent<string>) => {
          auth.accessToken = e.detail
          auth.ws?.refreshToken(e.detail)
        }) as EventListener)
      }
    },
    async onWSReconnected() {
      this.syncingAfterReconnect = true
      try {
        await this.pullSync()
        this.setTransientNotice('连接已恢复，消息已同步')
      } catch {
        this.setTransientNotice('连接已恢复，同步消息失败，将自动重试')
      } finally {
        this.syncingAfterReconnect = false
        this.wasDisconnected = false
      }
    },
    setTransientNotice(msg: string) {
      this.error = ''
      this.notice = msg
      if (this.noticeTimer) window.clearTimeout(this.noticeTimer)
      this.noticeTimer = window.setTimeout(() => {
        if (this.notice === msg) this.notice = ''
        this.noticeTimer = 0
      }, 3000)
    },
    forceReconnect() {
      const auth = useAuthStore()
      this.wasDisconnected = true
      if (auth.ws) {
        auth.reconnectWS()
        this.bindWS()
      } else if (auth.accessToken) {
        auth.connectWS()
        this.bindWS()
      }
    },
    onFrame(frame: WSFrame) {
      if (frame.type === 'kick') {
        const auth = useAuthStore()
        this.setTransientNotice('该设备已被强制下线')
        void auth.logout().then(() => {
          void import('../router').then((m) => m.default.replace('/login'))
        })
        return
      }
      if (frame.type === 'error') {
        const payload = frame.payload as { msg?: string }
        this.error = payload?.msg || '消息发送失败'
        return
      }
      if (frame.type === 'ack') {
        const ack = frame.payload as AckPayload
        if (!ack?.client_msg_id) return
        let ackConvId = ''
        for (const convId of Object.keys(this.messages)) {
          const list = this.messages[convId]
          const idx = list.findIndex((m) => m.client_msg_id === ack.client_msg_id)
          if (idx >= 0) {
            list[idx] = { ...list[idx], msg_id: ack.msg_id, seq: ack.seq, status: 'sent' }
            this.messages[convId] = [...list]
            ackConvId = convId
            break
          }
        }
        if (ackConvId && ackConvId === this.activeConvId && ack.seq) {
          this.bumpLocalReadSeq(ackConvId, ack.seq)
        }
        return
      }
      if (frame.type === 'read') {
        const p = frame.payload as { conversation_id?: string; user_id?: string; read_seq?: number }
        const auth = useAuthStore()
        if (p?.conversation_id && p.read_seq != null && !sameId(p.user_id, auth.user?.id)) {
          const prev = this.peerReadSeq[p.conversation_id] || 0
          this.peerReadSeq[p.conversation_id] = Math.max(prev, p.read_seq)
          const list = [...(this.memberReadState[p.conversation_id] || [])]
          const uid = idStr(p.user_id)
          const idx = list.findIndex((m) => sameId(m.user_id, uid))
          if (idx >= 0) list[idx] = { ...list[idx], read_seq: Math.max(list[idx].read_seq, p.read_seq) }
          else list.push({ user_id: uid, read_seq: p.read_seq })
          this.memberReadState[p.conversation_id] = list
        }
        return
      }
      if (frame.type === 'recall') {
        const p = frame.payload as ChatMessage
        if (!p?.conversation_id) return
        const list = this.messages[p.conversation_id] || []
        const idx = list.findIndex(
          (m) => m.client_msg_id === p.client_msg_id || (p.msg_id && m.msg_id === p.msg_id),
        )
        if (idx >= 0) {
          list[idx] = { ...list[idx], msg_type: 4, content: '[已撤回]' }
          this.messages[p.conversation_id] = [...list]
        }
        void this.loadConversations()
        return
      }
      if (frame.type === 'edit') {
        const p = frame.payload as ChatMessage
        if (!p?.conversation_id) return
        const list = this.messages[p.conversation_id] || []
        const idx = list.findIndex(
          (m) => m.client_msg_id === p.client_msg_id || (p.msg_id != null && idStr(m.msg_id) === idStr(p.msg_id)),
        )
        if (idx >= 0) {
          list[idx] = {
            ...list[idx],
            content: p.content,
            edited_at: p.edited_at || new Date().toISOString(),
            msg_type: p.msg_type ?? list[idx].msg_type,
          }
          this.messages[p.conversation_id] = [...list]
        }
        void this.loadConversations()
        return
      }
      if (frame.type === 'typing') {
        const p = frame.payload as { conversation_id?: string; user_id?: string; typing?: boolean }
        if (!p?.conversation_id || !p.user_id) return
        this.applyPeerTyping(p.conversation_id, idStr(p.user_id), p.typing !== false)
        return
      }
      if (frame.type === 'reaction') {
        const p = frame.payload as {
          msg_id?: string | number
          reactions?: ReactionSummary[]
        }
        if (p?.msg_id != null) {
          this.reactions[idStr(p.msg_id)] = p.reactions || []
        }
        return
      }
      if (frame.type === 'pin') {
        const p = frame.payload as {
          conversation_id?: string
          pins?: PinnedItem[]
        }
        if (p?.conversation_id) {
          this.pinsByConv[p.conversation_id] = this.normalizePins(p.pins || [])
        }
        return
      }
      if (frame.type === 'poll_vote') {
        const p = frame.payload as {
          msg_id?: string | number
          poll?: {
            msg_id: string
            total: number
            counts: { option_id: string; count: number }[]
            my_option_id?: string
          }
        }
        if (p?.msg_id != null && p.poll) {
          const id = idStr(p.msg_id)
          const prev = this.polls[id]
          this.polls[id] = {
            ...p.poll,
            msg_id: id,
            my_option_id: p.poll.my_option_id || prev?.my_option_id,
          }
        }
        return
      }
      if (frame.type === 'reminder') {
        const p = frame.payload as {
          id?: string
          conversation_id?: string
          msg_id?: string
          preview?: string
        }
        const preview = (p?.preview || '').trim() || '有一条消息待处理'
        this.setTransientNotice(`提醒：${preview}`)
        this.reminderList = this.reminderList.filter((r) => idStr(r.id) !== idStr(p?.id))
        return
      }
      if (frame.type !== 'message') return
      const msg = frame.payload as ChatMessage
      msg.from_user_id = idStr(msg.from_user_id)
      this.clearPeerTyping(msg.conversation_id, msg.from_user_id)
      this.mergeMessage(msg)
      const auth = useAuthStore()
      if (msg.conversation_id === this.activeConvId && !sameId(msg.from_user_id, auth.user?.id)) {
        void this.markRead(msg.conversation_id)
      }
      if (!sameId(msg.from_user_id, auth.user?.id)) {
        const settings = useSettingsStore()
        const mentioned = this.messageMentionsMe(msg)
        const muted = this.isMuted(msg.conversation_id)
        if (
          settings.shouldDesktopNotify() &&
          (!muted || mentioned) &&
          (document.hidden || msg.conversation_id !== this.activeConvId || mentioned)
        ) {
          const friend = this.friends.find((f) => sameId(f.id, msg.from_user_id))
          const title = mentioned
            ? `${friend ? friendDisplayName(friend) : '有人'} 提到了你`
            : friend
              ? friendDisplayName(friend)
              : '新消息'
          const body = previewMessage(msg.content, msg.msg_type)
          showMessageNotification(title, body)
        }
      }
      void this.loadConversations()
    },
    mergeMessage(msg: ChatMessage) {
      const list = this.messages[msg.conversation_id] || []
      const idx = list.findIndex((m) => m.client_msg_id === msg.client_msg_id)
      if (idx >= 0) {
        list[idx] = { ...list[idx], ...msg }
      } else {
        list.push(msg)
      }
      list.sort((a, b) => (a.seq || 0) - (b.seq || 0))
      this.messages[msg.conversation_id] = [...list]
    },
    async ensureAgent() {
      try {
        await http.post('/agent/ensure')
        const { data } = await http.get('/agent/info')
        const res = unwrapApiData<{ user_id: string; llm: boolean }>(data)
        this.agentUserId = idStr(res.user_id)
        this.agentLLMEnabled = !!res.llm
      } catch {
        /* agent optional */
      }
    },
    async refreshPresence(userIds: string[]) {
      const ids = [...new Set(userIds.map(idStr).filter(Boolean))].slice(0, 100)
      if (!ids.length) return
      try {
        const { data } = await http.post('/users/presence', { user_ids: ids })
        const res = unwrapApiData<{ online: Record<string, boolean> }>(data)
        this.onlineMap = { ...this.onlineMap, ...(res.online || {}) }
      } catch {
        /* ignore */
      }
    },
    isOnline(userId: string) {
      return !!this.onlineMap[idStr(userId)]
    },
    async loadFriends() {
      const { data } = await http.get('/friends')
      const res = unwrapApiData<{ friends: (PublicProfile & { remark?: string })[] }>(data)
      this.friends = (res.friends || []).map((f) => ({ ...f, id: idStr(f.id) }))
      void this.refreshPresence(this.friends.map((f) => f.id))
    },
    async setFriendRemark(friendId: UserId, remark: string) {
      const { data } = await http.put(`/friends/${idStr(friendId)}/remark`, { remark })
      unwrapApiData(data)
      const f = this.friends.find((x) => sameId(x.id, friendId))
      if (f) f.remark = remark
      if (sameId(this.activeToUser, friendId)) {
        const friend = this.friends.find((x) => sameId(x.id, friendId))
        if (friend) this.activeTitle = friendDisplayName(friend)
      }
      await this.loadFriends()
    },
    async loadGroups() {
      try {
        const { data } = await http.get('/groups')
        const res = unwrapApiData<{ groups: GroupItem[] }>(data)
        this.groups = (res.groups || []).map((g) => ({
          ...g,
          id: idStr(g.id),
          owner_id: idStr(g.owner_id),
        }))
      } catch (e) {
        if (isNotFoundApi(e)) {
          this.groups = []
          return
        }
        throw e
      }
    },
    async loadConversations() {
      try {
        const { data } = await http.get('/conversations')
        const res = unwrapApiData<{ conversations: ConversationItem[] }>(data)
        this.conversations = res.conversations || []
      } catch (e) {
        if (isNotFoundApi(e)) {
          this.conversations = []
          return
        }
        throw e
      }
    },
    async searchUsers(q: string) {
      const { data } = await http.get('/users/search', { params: { q, limit: 10 } })
      const res = unwrapApiData<{ users: PublicProfile[] }>(data)
      return (res.users || []).map((u) => ({ ...u, id: idStr(u.id) }))
    },
    async searchGroups(q: string) {
      const { data } = await http.get('/groups/discover', { params: { q, limit: 10 } })
      const res = unwrapApiData<{ groups: GroupPublicItem[] }>(data)
      return (res.groups || []).map((g) => ({
        ...g,
        id: idStr(g.id),
      }))
    },
    async lookupGroupByNo(groupNo: string) {
      const { data } = await http.get(`/groups/by-no/${encodeURIComponent(groupNo.trim())}`)
      const res = unwrapApiData<GroupPublicItem>(data)
      return { ...res, id: idStr(res.id) }
    },
    async requestFriend(toUserId: UserId, message = '') {
      this.clearError()
      const body: Record<string, string> = { to_user_id: idStr(toUserId) }
      if (message.trim()) body.message = message.trim()
      const { data } = await http.post('/friends/request', body)
      unwrapApiData(data)
      this.setNotice('好友申请已发送，请等待对方接受')
      await this.loadPending()
    },
    async loadPending() {
      await Promise.all([this.loadFriendPending(), this.loadGroupInvitations()])
    },
    async loadFriendPending() {
      const { data } = await http.get('/friends/requests')
      const res = unwrapApiData<{
        requests: {
          id: string
          from_user_id: UserId
          display_name: string
          avatar?: string
          message: string
        }[]
      }>(data)
      this.pending = (res.requests || []).map((r) => ({
        ...r,
        id: idStr(r.id),
        from_user_id: idStr(r.from_user_id),
      }))
    },
    async loadGroupInvitations() {
      const { data } = await http.get('/groups/invitations')
      const res = unwrapApiData<{ invitations: GroupInvitationItem[] }>(data)
      this.groupInvitations = (res.invitations || []).map((r) => ({
        ...r,
        id: idStr(r.id),
        group_id: idStr(r.group_id),
        from_user_id: idStr(r.from_user_id),
      }))
    },
    async acceptFriend(reqId: string) {
      this.clearError()
      const { data } = await http.post(`/friends/request/${reqId}/accept`)
      unwrapApiData(data)
      await this.loadFriends()
      await this.loadPending()
      this.setNotice('已添加好友')
    },
    async rejectFriend(reqId: string) {
      this.clearError()
      const { data } = await http.post(`/friends/request/${reqId}/reject`)
      unwrapApiData(data)
      await this.loadPending()
    },
    async deleteFriend(friendId: UserId) {
      this.clearError()
      const { data } = await http.delete(`/friends/${idStr(friendId)}`)
      unwrapApiData(data)
      if (sameId(this.activeToUser, friendId)) {
        this.activeConvId = ''
        this.activeToUser = ''
        this.activeTitle = ''
      }
      await this.loadFriends()
      await this.loadConversations()
    },
    async createGroup(name: string, inviteFriendIds: UserId[]) {
      this.clearError()
      const ids = inviteFriendIds.map((id) => idStr(id)).filter((s) => /^\d+$/.test(s))
      const { data } = await http.post('/groups', { name, invite_friend_ids: ids })
      const res = unwrapApiData<{
        conversation_id: string
        group_id: string
        group_no: string
        invites_sent: number
      }>(data)
      await this.loadGroups()
      await this.loadConversations()
      if (res.invites_sent > 0) {
        this.setNotice(`群聊已创建，已向 ${res.invites_sent} 位好友发送入群邀请`)
      } else {
        this.setNotice('群聊已创建')
      }
      return res
    },
    async startFaceToFace(code: string) {
      this.clearError()
      const { data } = await http.post('/groups/face-to-face/start', { code: code.trim() })
      const res = unwrapApiData<{
        conversation_id: string
        group_id: string
        group_no: string
        face_code: string
        expires_at: string
      }>(data)
      await this.loadGroups()
      await this.loadConversations()
      return res
    },
    async joinFaceToFace(code: string) {
      this.clearError()
      const { data } = await http.post('/groups/face-to-face/join', { code: code.trim() })
      const res = unwrapApiData<{ conversation_id?: string; group_id?: string }>(data)
      await this.loadGroups()
      await this.loadConversations()
      this.setNotice('已加入群聊')
      return res
    },
    async joinGroupByNo(groupNo: string) {
      this.clearError()
      const { data } = await http.post('/groups/join-by-no', { group_no: groupNo.trim() })
      unwrapApiData(data)
      this.setNotice('已发送加群申请，请在侧栏通知中接受')
      await this.loadGroupInvitations()
    },
    async acceptGroupInvitation(inviteId: string) {
      this.clearError()
      const { data } = await http.post(`/groups/invitations/${idStr(inviteId)}/accept`)
      unwrapApiData(data)
      await this.loadGroupInvitations()
      await this.loadGroups()
      await this.loadConversations()
      this.setNotice('已加入群聊')
    },
    async rejectGroupInvitation(inviteId: string) {
      this.clearError()
      const { data } = await http.post(`/groups/invitations/${idStr(inviteId)}/reject`)
      unwrapApiData(data)
      await this.loadGroupInvitations()
    },
    async inviteGroupMembers(groupId: string, userIds: UserId[]) {
      this.clearError()
      const ids = userIds.map((id) => Number(idStr(id))).filter((n) => n > 0)
      const { data } = await http.post(`/groups/${idStr(groupId)}/invites`, { user_ids: ids })
      const res = unwrapApiData<{ invites_sent: number }>(data)
      if (res.invites_sent > 0) {
        this.setNotice(`已向 ${res.invites_sent} 位好友发送入群邀请`)
      } else {
        this.setNotice('没有可邀请的好友')
      }
      return res.invites_sent
    },
    async setGroupAdmin(groupId: string, userId: string, admin: boolean) {
      this.clearError()
      const gid = idStr(groupId)
      const uid = idStr(userId)
      if (admin) {
        const { data } = await http.post(`/groups/${gid}/admins/${uid}`)
        unwrapApiData(data)
        this.setNotice('已设为管理员')
      } else {
        const { data } = await http.delete(`/groups/${gid}/admins/${uid}`)
        unwrapApiData(data)
        this.setNotice('已取消管理员')
      }
    },
    async refreshFaceToFace(groupId: string) {
      this.clearError()
      const { data } = await http.post(`/groups/${idStr(groupId)}/face-to-face/refresh`)
      return unwrapApiData<{ face_code: string; expires_at: string }>(data)
    },
    async getFaceToFaceSession(groupId: string) {
      const { data } = await http.get(`/groups/${idStr(groupId)}/face-to-face`)
      return unwrapApiData<{ face_code: string; expires_at: string }>(data)
    },
    async listGroupPendingInvites(groupId: string) {
      const { data } = await http.get(`/groups/${idStr(groupId)}/invitations`)
      const res = unwrapApiData<{
        invitations: {
          id: string
          to_user_id: string
          to_name: string
          to_avatar?: string
          message: string
          invite_type: number
          created_at: string
        }[]
      }>(data)
      return (res.invitations || []).map((r) => ({ ...r, id: idStr(r.id), to_user_id: idStr(r.to_user_id) }))
    },
    async cancelGroupInvite(groupId: string, inviteId: string) {
      this.clearError()
      const { data } = await http.delete(`/groups/${idStr(groupId)}/invitations/${idStr(inviteId)}`)
      unwrapApiData(data)
      this.setNotice('已撤销入群邀请')
    },
    async kickGroupMember(groupId: string, userId: string) {
      this.clearError()
      const { data } = await http.delete(`/groups/${idStr(groupId)}/members/${idStr(userId)}`)
      unwrapApiData(data)
      this.setNotice('已移出群成员')
    },
    async leaveGroup(groupId: string) {
      this.clearError()
      const gid = idStr(groupId)
      const me = useAuthStore().user?.id
      if (!me) return
      const { data } = await http.delete(`/groups/${gid}/members/${idStr(me)}`)
      unwrapApiData(data)
      if (this.activeGroupId === gid) {
        this.activeConvId = ''
        this.activeGroupId = ''
        this.activeTitle = ''
      }
      await this.loadGroups()
      await this.loadConversations()
      this.setNotice('已退出群聊')
    },
    async transferGroupOwner(groupId: string, newOwnerId: string) {
      this.clearError()
      const { data } = await http.post(`/groups/${idStr(groupId)}/transfer`, {
        new_owner_id: idStr(newOwnerId),
      })
      unwrapApiData(data)
      this.setNotice('群主已转让')
    },
    async fetchGroup(groupId: string) {
      const gid = idStr(groupId)
      const { data } = await http.get(`/groups/${gid}`)
      const res = unwrapApiData<{
        id: string
        name: string
        group_no?: string
        owner_id: string
        conversation_id: string
        notice?: string
        welcome_text?: string
        admin_only?: boolean
        slow_mode_secs?: number
        member_roles?: Record<string, number>
        member_muted?: Record<string, boolean>
        member_nicknames?: Record<string, string>
        member_remarks?: Record<string, string>
        members: PublicProfile[]
      }>(data)
      this.groupMembers[gid] = (res.members || []).map((m) => ({ ...m, id: idStr(m.id) }))
      if (res.member_roles) {
        this.groupMemberRoles[gid] = res.member_roles
      }
      this.groupMemberMuted[gid] = res.member_muted || {}
      this.groupMemberNicknames[gid] = res.member_nicknames || {}
      this.groupMemberRemarks[gid] = res.member_remarks || {}
      const idx = this.groups.findIndex((g) => g.id === gid)
      if (idx >= 0) {
        this.groups[idx] = {
          ...this.groups[idx],
          notice: res.notice || '',
          welcome_text: res.welcome_text || '',
          name: res.name,
          owner_id: idStr(res.owner_id),
          admin_only: !!res.admin_only,
          slow_mode_secs: res.slow_mode_secs || 0,
        }
      }
      return res
    },
    async setGroupNotice(groupId: string, notice: string) {
      const gid = idStr(groupId)
      const { data } = await http.put(`/groups/${gid}/notice`, { notice })
      unwrapApiData(data)
      const idx = this.groups.findIndex((g) => g.id === gid)
      if (idx >= 0) this.groups[idx] = { ...this.groups[idx], notice }
      this.setTransientNotice(notice.trim() ? '群公告已更新' : '群公告已清空')
    },
    async setGroupWelcome(groupId: string, welcomeText: string) {
      const gid = idStr(groupId)
      const { data } = await http.put(`/groups/${gid}/welcome`, { welcome_text: welcomeText })
      unwrapApiData(data)
      const idx = this.groups.findIndex((g) => g.id === gid)
      if (idx >= 0) this.groups[idx] = { ...this.groups[idx], welcome_text: welcomeText.trim() }
      this.setTransientNotice(welcomeText.trim() ? '群欢迎语已更新' : '群欢迎语已清空')
    },
    async setGroupAdminOnly(groupId: string, adminOnly: boolean) {
      const gid = idStr(groupId)
      const { data } = await http.put(`/groups/${gid}/admin-only`, { admin_only: adminOnly })
      unwrapApiData(data)
      const idx = this.groups.findIndex((g) => g.id === gid)
      if (idx >= 0) this.groups[idx] = { ...this.groups[idx], admin_only: adminOnly }
      this.setTransientNotice(adminOnly ? '已开启全员禁言' : '已关闭全员禁言')
    },
    async setGroupSlowMode(groupId: string, seconds: number) {
      const gid = idStr(groupId)
      const { data } = await http.put(`/groups/${gid}/slow-mode`, { seconds })
      unwrapApiData(data)
      const idx = this.groups.findIndex((g) => g.id === gid)
      if (idx >= 0) this.groups[idx] = { ...this.groups[idx], slow_mode_secs: seconds }
      this.setTransientNotice(seconds > 0 ? `已开启慢速模式（${seconds} 秒）` : '已关闭慢速模式')
    },
    async setGroupMemberMuted(groupId: string, userId: string, muted: boolean) {
      const gid = idStr(groupId)
      const uid = idStr(userId)
      if (muted) {
        await http.post(`/groups/${gid}/members/${uid}/mute`)
      } else {
        await http.delete(`/groups/${gid}/members/${uid}/mute`)
      }
      const map = { ...(this.groupMemberMuted[gid] || {}) }
      if (muted) map[uid] = true
      else delete map[uid]
      this.groupMemberMuted[gid] = map
      this.setTransientNotice(muted ? '已禁言该成员' : '已解除禁言')
    },
    async setMyGroupNickname(groupId: string, nickname: string) {
      const gid = idStr(groupId)
      const { data } = await http.put(`/groups/${gid}/my-nickname`, { nickname })
      const res = unwrapApiData<{ nickname?: string }>(data)
      const auth = useAuthStore()
      const uid = idStr(auth.user?.id)
      const map = { ...(this.groupMemberNicknames[gid] || {}) }
      const nick = (res.nickname ?? nickname).trim()
      if (nick) map[uid] = nick
      else delete map[uid]
      this.groupMemberNicknames[gid] = map
      this.setTransientNotice(nick ? '群名片已更新' : '已清除群名片')
    },
    async createGroupInviteLink(groupId: string, opts?: { max_uses?: number; expires_hours?: number }) {
      const gid = idStr(groupId)
      const { data } = await http.post(`/groups/${gid}/invite-links`, {
        max_uses: opts?.max_uses ?? 0,
        expires_hours: opts?.expires_hours ?? 0,
      })
      const res = unwrapApiData<{
        id: string
        code: string
        max_uses: number
        use_count: number
        expires_at?: string
      }>(data)
      this.setTransientNotice('邀请链接已创建')
      return res
    },
    async listGroupInviteLinks(groupId: string) {
      const gid = idStr(groupId)
      const { data } = await http.get(`/groups/${gid}/invite-links`)
      const res = unwrapApiData<{
        links: {
          id: string
          code: string
          max_uses: number
          use_count: number
          expires_at?: string
          expired?: boolean
          created_at?: string
        }[]
      }>(data)
      return res.links || []
    },
    async revokeGroupInviteLink(groupId: string, linkId: string) {
      const gid = idStr(groupId)
      await http.delete(`/groups/${gid}/invite-links/${idStr(linkId)}`)
      this.setTransientNotice('邀请链接已撤销')
    },
    async previewInviteLink(code: string) {
      const { data } = await http.get(`/invite-links/${encodeURIComponent(code.trim())}`)
      return unwrapApiData<{
        code: string
        group_id: string
        group_name: string
        group_no: string
        member_count: number
        is_member?: boolean
        usable?: boolean
        expires_at?: string
      }>(data)
    },
    async joinViaInviteLink(code: string) {
      const { data } = await http.post(`/invite-links/${encodeURIComponent(code.trim())}/join`)
      const res = unwrapApiData<{
        status: string
        group_id: string
        conversation_id: string
        name: string
      }>(data)
      await this.loadGroups()
      const g = this.groups.find((x) => sameId(x.id, res.group_id))
      if (g) await this.openGroup(g)
      this.setTransientNotice(res.status === 'already_member' ? '你已在该群中' : '已加入群聊')
      return res
    },
    groupMemberDisplayName(userId: string, fallback?: PublicProfile) {
      const uid = idStr(userId)
      if (this.activeGroupId) {
        const remark = this.groupMemberRemarks[this.activeGroupId]?.[uid]?.trim()
        if (remark) return remark
        const gn = this.groupMemberNicknames[this.activeGroupId]?.[uid]?.trim()
        if (gn) return gn
      }
      const friend = this.friends.find((f) => sameId(f.id, uid))
      if (friend) return friendDisplayName(friend)
      if (fallback) return fallback.nickname || fallback.username
      return '成员'
    },
    mentionName(user: PublicProfile) {
      if (this.activeGroupId) {
        const remark = this.groupMemberRemarks[this.activeGroupId]?.[idStr(user.id)]?.trim()
        if (remark) return remark
        const gn = this.groupMemberNicknames[this.activeGroupId]?.[idStr(user.id)]?.trim()
        if (gn) return gn
      }
      const friend = this.friends.find((f) => sameId(f.id, user.id))
      if (friend) return friendDisplayName(friend)
      return user.nickname || user.username
    },
    async setGroupMemberRemark(groupId: string, userId: string, remark: string) {
      const gid = idStr(groupId)
      const uid = idStr(userId)
      const { data } = await http.put(`/groups/${gid}/members/${uid}/remark`, { remark })
      const res = unwrapApiData<{ remark?: string }>(data)
      const map = { ...(this.groupMemberRemarks[gid] || {}) }
      const r = (res.remark ?? remark).trim()
      if (r) map[uid] = r
      else delete map[uid]
      this.groupMemberRemarks[gid] = map
      this.setTransientNotice(r ? '备注已更新' : '已清除备注')
    },
    /** Detect @me / @所有人 in text (strip reply envelope). */
    messageMentionsMe(msg: ChatMessage) {
      if (msg.msg_type !== 1) return false
      const auth = useAuthStore()
      const me = auth.user
      if (!me) return false
      const text = parseReplyContent(msg.content).text || msg.content
      if (/@所有人(?:\s|$|[，。！？,.!?])/.test(text) || text.includes('@所有人 ')) return true
      const names = [me.nickname, me.username].filter(Boolean) as string[]
      if (this.activeGroupId) {
        const gn = this.groupMemberNicknames[this.activeGroupId]?.[idStr(me.id)]?.trim()
        if (gn) names.unshift(gn)
      }
      for (const n of names) {
        if (!n) continue
        const re = new RegExp(`@${escapeRegExp(n)}(?:\\s|$|[，。！？,.!?])`)
        if (re.test(text)) return true
      }
      return false
    },
    isFriend(userId: string) {
      return this.friends.some((f) => sameId(f.id, userId))
    },
    togglePinFriend(friendId: string) {
      const id = idStr(friendId)
      const i = this.pinnedFriendIds.indexOf(id)
      if (i >= 0) this.pinnedFriendIds.splice(i, 1)
      else this.pinnedFriendIds.unshift(id)
      savePinned([...this.pinnedFriendIds], [...this.pinnedGroupIds])
      void this.persistChatPrefs()
    },
    togglePinGroup(groupId: string) {
      const id = idStr(groupId)
      const i = this.pinnedGroupIds.indexOf(id)
      if (i >= 0) this.pinnedGroupIds.splice(i, 1)
      else this.pinnedGroupIds.unshift(id)
      savePinned([...this.pinnedFriendIds], [...this.pinnedGroupIds])
      void this.persistChatPrefs()
    },
    isPinnedFriend(friendId: string) {
      return this.pinnedFriendIds.includes(idStr(friendId))
    },
    isPinnedGroup(groupId: string) {
      return this.pinnedGroupIds.includes(idStr(groupId))
    },
    isMuted(convId: string) {
      return this.mutedConvIds.includes(convId)
    },
    createFolder(name: string) {
      const n = name.trim().slice(0, 24)
      if (!n) return
      if (this.folders.length >= 20) {
        this.setError('最多 20 个文件夹')
        return
      }
      const id = `f_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 6)}`
      this.folders = [...this.folders, { id, name: n, conversation_ids: [] }]
      void this.persistChatPrefs()
      this.setTransientNotice('已创建文件夹')
    },
    renameFolder(folderId: string, name: string) {
      const n = name.trim().slice(0, 24)
      if (!n) return
      this.folders = this.folders.map((f) => (f.id === folderId ? { ...f, name: n } : f))
      void this.persistChatPrefs()
    },
    deleteFolder(folderId: string) {
      this.folders = this.folders.filter((f) => f.id !== folderId)
      if (this.activeFolderId === folderId) this.activeFolderId = ''
      void this.persistChatPrefs()
      this.setTransientNotice('已删除文件夹')
    },
    setActiveFolder(folderId: string) {
      this.activeFolderId = this.activeFolderId === folderId ? '' : folderId
    },
    convInFolder(folderId: string, convId: string) {
      const f = this.folders.find((x) => x.id === folderId)
      return !!f?.conversation_ids.includes(convId)
    },
    assignConvToFolder(folderId: string, convId: string) {
      if (!folderId || !convId) return
      this.folders = this.folders.map((f) => {
        const ids = new Set(f.conversation_ids)
        if (f.id === folderId) ids.add(convId)
        else ids.delete(convId)
        return { ...f, conversation_ids: [...ids] }
      })
      void this.persistChatPrefs()
      this.setTransientNotice('已移入文件夹')
    },
    removeConvFromFolders(convId: string) {
      this.folders = this.folders.map((f) => ({
        ...f,
        conversation_ids: f.conversation_ids.filter((id) => id !== convId),
      }))
      void this.persistChatPrefs()
      this.setTransientNotice('已移出文件夹')
    },
    friendInActiveFolder(friendId: string) {
      if (!this.activeFolderId) return true
      const auth = useAuthStore()
      const convId = directConvId(auth.user?.id || '', friendId)
      return this.convInFolder(this.activeFolderId, convId)
    },
    groupInActiveFolder(group: { conversation_id: string }) {
      if (!this.activeFolderId) return true
      return this.convInFolder(this.activeFolderId, group.conversation_id)
    },
    toggleMute(convId: string) {
      if (!convId) return
      const i = this.mutedConvIds.indexOf(convId)
      if (i >= 0) this.mutedConvIds.splice(i, 1)
      else this.mutedConvIds.push(convId)
      saveMuted([...this.mutedConvIds])
      void this.persistChatPrefs()
    },
    async persistChatPrefs() {
      try {
        const settings = useSettingsStore()
        await http.put('/users/me/chat-prefs', {
          muted: [...this.mutedConvIds],
          pinned_friends: [...this.pinnedFriendIds],
          pinned_groups: [...this.pinnedGroupIds],
          folders: this.folders.map((f) => ({
            id: f.id,
            name: f.name,
            conversation_ids: [...f.conversation_ids],
          })),
          notify: settings.cloudNotifyPayload(),
        })
      } catch {
        /* keep local; sync later */
      }
    },
    async loadChatPrefs() {
      try {
        const { data } = await http.get('/users/me/chat-prefs')
        const prefs = unwrapApiData<{
          muted?: string[]
          pinned_friends?: string[]
          pinned_groups?: string[]
          folders?: { id: string; name: string; conversation_ids?: string[] }[]
          notify?: {
            desktop_enabled?: boolean
            quiet_hours_enabled?: boolean
            quiet_start?: string
            quiet_end?: string
          }
        }>(data)
        const muted = (prefs.muted || []).map(idStr)
        const friends = (prefs.pinned_friends || []).map(idStr)
        const groups = (prefs.pinned_groups || []).map(idStr)
        const folders = (prefs.folders || []).map((f) => ({
          id: f.id || `f_${Date.now()}`,
          name: (f.name || '未命名').slice(0, 24),
          conversation_ids: (f.conversation_ids || []).map(idStr),
        }))
        const settings = useSettingsStore()
        const serverEmpty =
          muted.length === 0 &&
          friends.length === 0 &&
          groups.length === 0 &&
          folders.length === 0 &&
          !prefs.notify
        const localHas =
          this.mutedConvIds.length > 0 ||
          this.pinnedFriendIds.length > 0 ||
          this.pinnedGroupIds.length > 0 ||
          this.folders.length > 0 ||
          settings.notify.quietHoursEnabled
        if (serverEmpty && localHas) {
          await this.persistChatPrefs()
          return
        }
        this.mutedConvIds = muted
        this.pinnedFriendIds = friends
        this.pinnedGroupIds = groups
        this.folders = folders
        saveMuted(muted)
        savePinned(friends, groups)
        if (prefs.notify) {
          settings.applyCloudNotify(prefs.notify)
        }
      } catch {
        /* offline / table missing — keep local */
      }
    },
    async loadDrafts(): Promise<Record<string, string> | null> {
      try {
        const { data } = await http.get('/users/me/drafts')
        const res = unwrapApiData<{
          drafts?: Record<string, string>
          items?: { conversation_id: string; content: string; updated_at?: string }[]
        }>(data)
        this.draftItems = res.items || []
        return res.drafts || {}
      } catch {
        this.draftItems = []
        return null
      }
    },
    async persistDraft(conversationId: string, content: string) {
      await http.put('/users/me/drafts', {
        conversation_id: conversationId,
        content,
      })
    },
    toggleActiveMute() {
      if (this.activeConvId) this.toggleMute(this.activeConvId)
    },
    applyPeerTyping(convId: string, userId: string, typing: boolean) {
      const auth = useAuthStore()
      if (sameId(userId, auth.user?.id)) return
      const key = `${convId}:${userId}`
      if (this.typingClearTimers[key]) {
        window.clearTimeout(this.typingClearTimers[key])
        delete this.typingClearTimers[key]
      }
      const cur = new Set(this.typingUsers[convId] || [])
      if (typing) {
        cur.add(userId)
        this.typingUsers[convId] = [...cur]
        this.typingClearTimers[key] = window.setTimeout(() => {
          this.clearPeerTyping(convId, userId)
        }, 4000)
      } else {
        cur.delete(userId)
        this.typingUsers[convId] = [...cur]
      }
    },
    clearPeerTyping(convId: string, userId: string) {
      const key = `${convId}:${userId}`
      if (this.typingClearTimers[key]) {
        window.clearTimeout(this.typingClearTimers[key])
        delete this.typingClearTimers[key]
      }
      const cur = (this.typingUsers[convId] || []).filter((id) => !sameId(id, userId))
      this.typingUsers[convId] = cur
    },
    notifyTyping(typing: boolean) {
      if (!this.activeConvId) return
      const auth = useAuthStore()
      if (!auth.ws) return
      const now = Date.now()
      if (typing && now - this.lastTypingSentAt < 1500) return
      this.lastTypingSentAt = now
      auth.ws.sendTyping(this.activeConvId, typing)
    },
    clearSearch() {
      this.searchQuery = ''
      this.searchResults = []
      this.searchLoading = false
      this.activeHashtag = ''
      this.clearHighlight()
    },
    clearHighlight() {
      this.highlightClientMsgId = ''
      if (this.highlightTimer) {
        window.clearTimeout(this.highlightTimer)
        this.highlightTimer = 0
      }
    },
    setHighlight(clientMsgId: string) {
      this.highlightClientMsgId = clientMsgId
      if (this.highlightTimer) window.clearTimeout(this.highlightTimer)
      this.highlightTimer = window.setTimeout(() => {
        this.highlightClientMsgId = ''
        this.highlightTimer = 0
      }, 2600)
    },
    async loadHistory(convId: string) {
      const { data } = await http.get(`/conversations/${convId}/messages`, { params: { limit: 50 } })
      const res = unwrapApiData<{
        messages: ChatMessage[]
        reactions?: Record<string, ReactionSummary[]>
        stars?: Record<string, boolean>
        polls?: Record<
          string,
          { msg_id: string; total: number; counts: { option_id: string; count: number }[]; my_option_id?: string }
        >
      }>(data)
      const merged = (res.messages || []).map((m) => ({ ...m, from_user_id: idStr(m.from_user_id) }))
      merged.sort((a, b) => (a.seq || 0) - (b.seq || 0))
      this.messages[convId] = merged
      this.historyHasMore[convId] = merged.length >= 50
      if (res.reactions) {
        this.reactions = { ...this.reactions, ...res.reactions }
      }
      if (res.stars) {
        this.starredMsgIds = { ...this.starredMsgIds, ...res.stars }
      }
      if (res.polls) {
        this.polls = { ...this.polls, ...res.polls }
      }
    },
    async loadMoreHistory(convId: string) {
      const list = this.messages[convId] || []
      const minSeq = list.reduce((n, m) => (m.seq && m.seq < n ? m.seq : n), list[0]?.seq || 0)
      if (!minSeq) return
      const { data } = await http.get(`/conversations/${convId}/messages`, {
        params: { limit: 50, before_seq: minSeq },
      })
      const res = unwrapApiData<{
        messages: ChatMessage[]
        reactions?: Record<string, ReactionSummary[]>
        stars?: Record<string, boolean>
        polls?: Record<
          string,
          { msg_id: string; total: number; counts: { option_id: string; count: number }[]; my_option_id?: string }
        >
      }>(data)
      const older = (res.messages || []).map((m) => ({ ...m, from_user_id: idStr(m.from_user_id) }))
      if (older.length < 50) this.historyHasMore[convId] = false
      const merged = [...older]
      for (const m of list) {
        if (!merged.some((x) => x.client_msg_id === m.client_msg_id)) merged.push(m)
      }
      merged.sort((a, b) => (a.seq || 0) - (b.seq || 0))
      this.messages[convId] = merged
      if (res.reactions) {
        this.reactions = { ...this.reactions, ...res.reactions }
      }
      if (res.stars) {
        this.starredMsgIds = { ...this.starredMsgIds, ...res.stars }
      }
      if (res.polls) {
        this.polls = { ...this.polls, ...res.polls }
      }
    },
    reactionsFor(msgId?: string | number) {
      if (msgId == null || msgId === '') return [] as ReactionSummary[]
      return this.reactions[idStr(msgId)] || []
    },
    normalizePins(pins: PinnedItem[]) {
      return (pins || []).map((p) => ({
        ...p,
        pinned_by: idStr(p.pinned_by),
        message: {
          ...p.message,
          from_user_id: idStr(p.message.from_user_id),
          msg_id: p.message.msg_id,
          client_msg_id: p.message.client_msg_id,
        },
      }))
    },
    activePins(): PinnedItem[] {
      if (!this.activeConvId) return []
      return this.pinsByConv[this.activeConvId] || []
    },
    isMsgPinned(msgId?: string | number) {
      if (msgId == null || msgId === '' || !this.activeConvId) return false
      const id = idStr(msgId)
      return (this.pinsByConv[this.activeConvId] || []).some((p) => idStr(p.message.msg_id) === id)
    },
    async loadPins(convId?: string) {
      const id = convId || this.activeConvId
      if (!id) return
      try {
        const { data } = await http.get(`/conversations/${id}/pins`)
        const res = unwrapApiData<{ pins: PinnedItem[] }>(data)
        this.pinsByConv[id] = this.normalizePins(res.pins || [])
      } catch {
        /* older servers may lack pins */
      }
    },
    async togglePin(msg: ChatMessage) {
      if (!msg.msg_id || !msg.conversation_id) return
      const convId = msg.conversation_id
      const msgId = idStr(msg.msg_id)
      const pinned = this.isMsgPinned(msg.msg_id)
      try {
        if (pinned) {
          const { data } = await http.delete(`/conversations/${convId}/pins/${msgId}`)
          const res = unwrapApiData<{ pins: PinnedItem[] }>(data)
          this.pinsByConv[convId] = this.normalizePins(res.pins || [])
          this.setTransientNotice('已取消置顶')
        } else {
          const { data } = await http.post(`/conversations/${convId}/pins`, { msg_id: msgId })
          const res = unwrapApiData<{ pins: PinnedItem[] }>(data)
          this.pinsByConv[convId] = this.normalizePins(res.pins || [])
          this.setTransientNotice('已置顶消息')
        }
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '置顶失败')
      }
    },
    activeBookmarks(): ConversationBookmark[] {
      if (!this.activeConvId) return []
      return this.bookmarksByConv[this.activeConvId] || []
    },
    async loadBookmarks(convId?: string) {
      const id = convId || this.activeConvId
      if (!id) return
      try {
        const { data } = await http.get(`/conversations/${id}/bookmarks`)
        const res = unwrapApiData<{ bookmarks: ConversationBookmark[] }>(data)
        this.bookmarksByConv[id] = (res.bookmarks || []).map((b) => ({
          ...b,
          id: idStr(b.id),
          created_by: idStr(b.created_by),
        }))
      } catch {
        /* older servers */
      }
    },
    async addBookmark(title: string, url: string) {
      if (!this.activeConvId) return
      const convId = this.activeConvId
      try {
        const { data } = await http.post(`/conversations/${convId}/bookmarks`, { title, url })
        const res = unwrapApiData<{ bookmark: ConversationBookmark }>(data)
        const b = {
          ...res.bookmark,
          id: idStr(res.bookmark.id),
          created_by: idStr(res.bookmark.created_by),
        }
        this.bookmarksByConv[convId] = [b, ...(this.bookmarksByConv[convId] || [])]
        this.setTransientNotice('已添加书签')
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '添加书签失败')
      }
    },
    async deleteBookmark(bookmarkId: string) {
      if (!this.activeConvId) return
      const convId = this.activeConvId
      try {
        await http.delete(`/conversations/${convId}/bookmarks/${bookmarkId}`)
        this.bookmarksByConv[convId] = (this.bookmarksByConv[convId] || []).filter(
          (b) => idStr(b.id) !== idStr(bookmarkId),
        )
        this.setTransientNotice('已删除书签')
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '删除书签失败')
      }
    },
    translationFor(msgId?: string | number) {
      if (msgId == null || msgId === '') return null
      return this.translations[idStr(msgId)] || null
    },
    async translateMessage(msg: ChatMessage, targetLang?: string) {
      if (!msg.msg_id || !msg.conversation_id) return
      const id = idStr(msg.msg_id)
      this.translatingMsgId = id
      try {
        const { data } = await http.post(
          `/conversations/${msg.conversation_id}/messages/${msg.msg_id}/translate`,
          { target_lang: targetLang || '' },
        )
        const res = unwrapApiData<{ translation: string; target_lang: string }>(data)
        this.translations = {
          ...this.translations,
          [id]: { text: res.translation, target_lang: res.target_lang },
        }
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '翻译失败')
      } finally {
        if (this.translatingMsgId === id) this.translatingMsgId = ''
      }
    },
    clearTranslation(msgId?: string | number) {
      if (msgId == null || msgId === '') return
      const id = idStr(msgId)
      const next = { ...this.translations }
      delete next[id]
      this.translations = next
    },
    isStarred(msgId?: string | number) {
      if (msgId == null || msgId === '') return false
      return !!this.starredMsgIds[idStr(msgId)]
    },
    async toggleStar(msg: ChatMessage, opts?: { silent?: boolean; ensureStarred?: boolean }) {
      if (!msg.msg_id || !msg.conversation_id) return
      if (opts?.ensureStarred && this.isStarred(msg.msg_id)) return
      try {
        const { data } = await http.post(
          `/conversations/${msg.conversation_id}/messages/${msg.msg_id}/star`,
        )
        const res = unwrapApiData<{ msg_id: string; starred: boolean }>(data)
        const id = idStr(res.msg_id || msg.msg_id)
        this.starredMsgIds = { ...this.starredMsgIds, [id]: !!res.starred }
        if (!res.starred) {
          this.starredList = this.starredList.filter((s) => idStr(s.message.msg_id) !== id)
        }
        if (!opts?.silent) {
          this.setTransientNotice(res.starred ? '已收藏' : '已取消收藏')
        }
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '收藏失败')
      }
    },
    async batchStar(messages: ChatMessage[]) {
      let n = 0
      for (const m of messages) {
        if (!m.msg_id || m.msg_type === 4) continue
        if (this.isStarred(m.msg_id)) continue
        const before = this.isStarred(m.msg_id)
        await this.toggleStar(m, { silent: true, ensureStarred: true })
        if (!before && this.isStarred(m.msg_id)) n += 1
      }
      if (n > 0) this.setTransientNotice(`已收藏 ${n} 条消息`)
      else this.setTransientNotice('所选消息均已收藏或无法收藏')
      return n
    },
    async loadStarred() {
      try {
        const { data } = await http.get('/stars', { params: { limit: 50 } })
        const res = unwrapApiData<{
          stars: { message: ChatMessage; starred_at?: string }[]
        }>(data)
        this.starredList = (res.stars || []).map((s) => ({
          ...s,
          message: { ...s.message, from_user_id: idStr(s.message.from_user_id) },
        }))
        const map = { ...this.starredMsgIds }
        for (const s of this.starredList) {
          if (s.message.msg_id != null) map[idStr(s.message.msg_id)] = true
        }
        this.starredMsgIds = map
      } catch {
        /* ignore */
      }
    },
    async scheduleMessage(content: string, sendAtISO: string) {
      if (!this.activeConvId) return
      const payload: Record<string, unknown> = {
        conversation_id: this.activeConvId,
        content,
        send_at: sendAtISO,
      }
      if (this.activeGroupId) payload.conversation_type = 2
      else {
        payload.conversation_type = 1
        payload.to_user_id = idStr(this.activeToUser)
      }
      try {
        await http.post('/scheduled-messages', payload)
        this.setTransientNotice('已设置定时发送')
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '定时发送失败')
      }
    },
    async loadScheduled() {
      try {
        const { data } = await http.get('/scheduled-messages')
        const res = unwrapApiData<{
          items: {
            id: string
            conversation_id: string
            content: string
            send_at: string
          }[]
        }>(data)
        return res.items || []
      } catch {
        return []
      }
    },
    async remindMessage(msg: ChatMessage, remindAtISO: string) {
      if (!msg.msg_id || !msg.conversation_id) return
      try {
        const { data } = await http.post(
          `/conversations/${msg.conversation_id}/messages/${msg.msg_id}/remind`,
          { remind_at: remindAtISO },
        )
        const res = unwrapApiData<{
          item: {
            id: string
            conversation_id: string
            msg_id: string
            preview: string
            remind_at: string
          }
        }>(data)
        if (res.item) {
          const item = {
            ...res.item,
            id: idStr(res.item.id),
            msg_id: idStr(res.item.msg_id),
          }
          this.reminderList = [...this.reminderList.filter((r) => r.id !== item.id), item].sort(
            (a, b) => new Date(a.remind_at).getTime() - new Date(b.remind_at).getTime(),
          )
        }
        this.setTransientNotice('已设置提醒')
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '设置提醒失败')
      }
    },
    async loadReminders() {
      try {
        const { data } = await http.get('/reminders')
        const res = unwrapApiData<{
          items: {
            id: string
            conversation_id: string
            msg_id: string
            preview: string
            remind_at: string
          }[]
        }>(data)
        this.reminderList = (res.items || []).map((r) => ({
          ...r,
          id: idStr(r.id),
          msg_id: idStr(r.msg_id),
        }))
      } catch {
        /* ignore */
      }
    },
    async cancelReminder(id: string) {
      try {
        await http.delete(`/reminders/${id}`)
        this.reminderList = this.reminderList.filter((r) => idStr(r.id) !== idStr(id))
        this.setTransientNotice('已取消提醒')
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '取消提醒失败')
      }
    },
    async cancelScheduled(id: string) {
      try {
        await http.delete(`/scheduled-messages/${id}`)
        this.setTransientNotice('已取消定时')
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '取消失败')
      }
    },
    async toggleReaction(msg: ChatMessage, emoji: string) {
      if (!msg.msg_id || !msg.conversation_id) return
      try {
        const { data } = await http.post(
          `/conversations/${msg.conversation_id}/messages/${msg.msg_id}/reactions`,
          { emoji },
        )
        const res = unwrapApiData<{ msg_id: string; reactions: ReactionSummary[] }>(data)
        this.reactions[idStr(res.msg_id || msg.msg_id)] = res.reactions || []
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '回应失败')
      }
    },
    async searchMessages(q: string) {
      if (!this.activeConvId) return
      const keyword = q.trim()
      this.searchQuery = keyword
      this.activeHashtag = ''
      if (!keyword) {
        this.searchResults = []
        return
      }
      this.searchLoading = true
      try {
        const { data } = await http.get(`/conversations/${this.activeConvId}/messages/search`, {
          params: { q: keyword, limit: 30 },
        })
        const res = unwrapApiData<{ messages: ChatMessage[] }>(data)
        this.searchResults = (res.messages || []).map((m) => ({
          ...m,
          from_user_id: idStr(m.from_user_id),
        }))
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '搜索失败')
        this.searchResults = []
      } finally {
        this.searchLoading = false
      }
    },
    async loadHashtags() {
      if (!this.activeConvId) {
        this.hashtagList = []
        return
      }
      try {
        const { data } = await http.get(`/conversations/${this.activeConvId}/hashtags`, {
          params: { limit: 30 },
        })
        const res = unwrapApiData<{ hashtags: { tag: string; count: number }[] }>(data)
        this.hashtagList = res.hashtags || []
      } catch {
        this.hashtagList = []
      }
    },
    async searchByHashtag(tag: string) {
      if (!this.activeConvId) return
      const t = tag.trim().replace(/^#/, '')
      if (!t) return
      this.activeHashtag = t
      this.searchQuery = `#${t}`
      this.searchLoading = true
      try {
        const { data } = await http.get(`/conversations/${this.activeConvId}/messages/by-hashtag`, {
          params: { tag: t, limit: 40 },
        })
        const res = unwrapApiData<{ messages: ChatMessage[] }>(data)
        this.searchResults = (res.messages || []).map((m) => ({
          ...m,
          from_user_id: idStr(m.from_user_id),
        }))
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '话题筛选失败')
        this.searchResults = []
      } finally {
        this.searchLoading = false
      }
    },
    async loadMedia(kind: string = 'all') {
      if (!this.activeConvId) {
        this.mediaList = []
        return
      }
      this.mediaKind = kind || 'all'
      this.mediaLoading = true
      try {
        const { data } = await http.get(`/conversations/${this.activeConvId}/media`, {
          params: { kind: this.mediaKind, limit: 60 },
        })
        const res = unwrapApiData<{ messages: ChatMessage[] }>(data)
        this.mediaList = (res.messages || []).map((m) => ({
          ...m,
          from_user_id: idStr(m.from_user_id),
        }))
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '加载媒体失败')
        this.mediaList = []
      } finally {
        this.mediaLoading = false
      }
    },
    async searchGlobal(q: string) {
      const keyword = q.trim()
      if (!keyword) {
        this.globalSearchResults = []
        return
      }
      this.globalSearchLoading = true
      try {
        const { data } = await http.get('/messages/search', { params: { q: keyword, limit: 30 } })
        const res = unwrapApiData<{ messages: ChatMessage[] }>(data)
        this.globalSearchResults = (res.messages || []).map((m) => ({
          ...m,
          from_user_id: idStr(m.from_user_id),
        }))
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '全局搜索失败')
        this.globalSearchResults = []
      } finally {
        this.globalSearchLoading = false
      }
    },
    clearLocalMessages(convId?: string) {
      const id = convId || this.activeConvId
      if (!id) return
      this.messages[id] = []
      this.historyHasMore[id] = true
      this.clearHighlight()
      this.setTransientNotice('已清空本地消息，可重新加载历史')
    },
    async exportTranscript() {
      if (!this.activeConvId) return
      try {
        const { data } = await http.get(`/conversations/${this.activeConvId}/export`, {
          responseType: 'text',
          transformResponse: [(d) => d],
        })
        const text = typeof data === 'string' ? data : String(data ?? '')
        const blob = new Blob([text], { type: 'text/plain;charset=utf-8' })
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `squirtlechat-${this.activeConvId.slice(0, 12)}.txt`
        a.click()
        URL.revokeObjectURL(url)
        this.setTransientNotice('已导出聊天记录')
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '导出失败')
      }
    },
    async loadMentions() {
      try {
        const { data } = await http.get('/mentions', { params: { limit: 30 } })
        const res = unwrapApiData<{ messages: ChatMessage[] }>(data)
        this.mentionInbox = (res.messages || []).map((m) => ({
          ...m,
          from_user_id: idStr(m.from_user_id),
        }))
      } catch {
        this.mentionInbox = []
      }
    },
    async reloadActiveHistory() {
      if (!this.activeConvId) return
      await this.loadHistory(this.activeConvId)
      if (this.activeGroupId) await this.loadReadState(this.activeConvId).catch(() => undefined)
      await this.markRead(this.activeConvId)
    },
    async jumpToMessage(msg: ChatMessage) {
      if (!msg.conversation_id || !msg.seq) return
      const convId = msg.conversation_id
      const list = this.messages[convId] || []
      const exists = list.some((m) => m.client_msg_id === msg.client_msg_id)
      if (!exists) {
        const { data } = await http.get(`/conversations/${convId}/messages`, {
          params: { around_seq: msg.seq, limit: 50 },
        })
        const res = unwrapApiData<{ messages: ChatMessage[] }>(data)
        const around = (res.messages || []).map((m) => ({ ...m, from_user_id: idStr(m.from_user_id) }))
        const merged = [...around]
        for (const m of list) {
          if (!merged.some((x) => x.client_msg_id === m.client_msg_id)) merged.push(m)
        }
        merged.sort((a, b) => (a.seq || 0) - (b.seq || 0))
        this.messages[convId] = merged
        const minSeq = merged.reduce((n, m) => (m.seq && m.seq < n ? m.seq : n), merged[0]?.seq || 0)
        this.historyHasMore[convId] = minSeq > 1
      }
      this.setHighlight(msg.client_msg_id)
    },
    async loadReadState(convId: string) {
      try {
        const { data } = await http.get(`/conversations/${convId}/read-state`)
        const res = unwrapApiData<{ members: { user_id: string; read_seq: number }[] }>(data)
        const auth = useAuthStore()
        const members = (res.members || []).map((m) => ({
          user_id: idStr(m.user_id),
          read_seq: m.read_seq || 0,
        }))
        this.memberReadState[convId] = members
        const peers = members.filter((m) => !sameId(m.user_id, auth.user?.id))
        if (peers.length === 1) {
          this.peerReadSeq[convId] = peers[0].read_seq
        } else if (peers.length > 1) {
          // DM-style double-check uses min peer read for "all read" feel in 1:1 only;
          // groups use groupReadCount() instead.
          this.peerReadSeq[convId] = Math.min(...peers.map((p) => p.read_seq))
        }
      } catch {
        /* ignore */
      }
    },
    groupReadCount(convId: string, seq?: number) {
      if (!seq) return 0
      const auth = useAuthStore()
      const members = this.memberReadState[convId] || []
      return members.filter((m) => !sameId(m.user_id, auth.user?.id) && m.read_seq >= seq).length
    },
    groupReadMembers(convId: string, seq?: number) {
      if (!seq) return [] as { user_id: string; read_seq: number }[]
      const auth = useAuthStore()
      return (this.memberReadState[convId] || []).filter(
        (m) => !sameId(m.user_id, auth.user?.id) && m.read_seq >= seq,
      )
    },
    groupUnreadMembers(convId: string, seq?: number) {
      if (!seq) return [] as { user_id: string; read_seq: number }[]
      const auth = useAuthStore()
      return (this.memberReadState[convId] || []).filter(
        (m) => !sameId(m.user_id, auth.user?.id) && m.read_seq < seq,
      )
    },
    groupPeerCount(convId: string) {
      const auth = useAuthStore()
      return (this.memberReadState[convId] || []).filter((m) => !sameId(m.user_id, auth.user?.id)).length
    },
    bumpLocalReadSeq(convId: string, readSeq: number) {
      const conv = this.conversations.find((c) => c.conversation_id === convId)
      if (!conv) return
      const nextRead = Math.max(conv.last_read_seq || 0, readSeq)
      conv.last_read_seq = nextRead
      conv.unread_count = Math.max(0, (conv.last_seq || 0) - nextRead)
      this.conversations = [...this.conversations]
    },
    async markRead(convId: string) {
      const list = this.messages[convId] || []
      const maxSeq = list.reduce((n, m) => Math.max(n, m.seq || 0), 0)
      if (!maxSeq) return
      try {
        const { data } = await http.post('/sync/read', { conversation_id: convId, read_seq: maxSeq })
        unwrapApiData(data)
        this.bumpLocalReadSeq(convId, maxSeq)
        await this.loadConversations()
      } catch {
        /* ignore */
      }
    },
    async markAllRead() {
      const unread = this.conversations.filter((c) => (c.unread_count || 0) > 0 && (c.last_seq || 0) > 0)
      if (!unread.length) {
        this.setTransientNotice('没有未读消息')
        return
      }
      await Promise.allSettled(
        unread.map(async (c) => {
          const { data } = await http.post('/sync/read', {
            conversation_id: c.conversation_id,
            read_seq: c.last_seq,
          })
          unwrapApiData(data)
        }),
      )
      await this.loadConversations()
      this.setTransientNotice(`已将 ${unread.length} 个会话标为已读`)
    },
    async openDirect(friend: PublicProfile) {
      const auth = useAuthStore()
      const myId = auth.user?.id || ''
      const toUserId = idStr(friend.id)
      this.activeConvId = directConvId(myId, toUserId)
      this.activeToUser = toUserId
      this.activeGroupId = ''
      this.activeTitle = friendDisplayName(friend)
      this.clearError()
      this.clearSearch()
      await this.loadHistory(this.activeConvId)
      await this.loadReadState(this.activeConvId)
      await Promise.allSettled([this.loadPins(this.activeConvId), this.loadBookmarks(this.activeConvId)])
      await this.markRead(this.activeConvId)
    },
    async openGroup(group: GroupItem) {
      this.activeConvId = group.conversation_id
      this.activeGroupId = group.id
      this.activeToUser = ''
      this.activeTitle = group.name
      this.clearError()
      this.clearSearch()
      await Promise.allSettled([
        this.fetchGroup(group.id),
        this.loadHistory(this.activeConvId),
        this.loadPins(this.activeConvId),
        this.loadBookmarks(this.activeConvId),
      ])
      await this.markRead(this.activeConvId)
    },
    async forwardMessages(
      target: { conversationId: string; conversationType: 1 | 2; toUserId?: string; groupId?: string },
      messages: ChatMessage[],
    ) {
      const auth = useAuthStore()
      if (!auth.ws) {
        this.setError('未连接，无法转发')
        return
      }
      const MAX = 30
      const eligible = messages.filter((m) => m.msg_type !== 4 && m.content !== '[已撤回]')
      if (!eligible.length) {
        this.setTransientNotice('没有可转发的消息')
        return
      }
      if (eligible.length > MAX) {
        this.setError(`一次最多转发 ${MAX} 条，请减少选择`)
        return
      }
      let sent = 0
      for (const m of eligible) {
        let content = m.content
        let msgType = m.msg_type || 1
        if (msgType === 1) {
          const parsed = parseReplyContent(content)
          content = parsed.text || content
        }
        const clientMsgId = crypto.randomUUID()
        const payload: Record<string, unknown> = {
          client_msg_id: clientMsgId,
          conversation_id: target.conversationId,
          msg_type: msgType,
          content,
          conversation_type: target.conversationType,
        }
        if (target.conversationType === 1) {
          payload.to_user_id = idStr(target.toUserId)
        }
        const optimistic: ChatMessage = {
          client_msg_id: clientMsgId,
          conversation_id: target.conversationId,
          from_user_id: idStr(auth.user?.id),
          msg_type: msgType,
          content,
          created_at: new Date().toISOString(),
          status: 'sending',
        }
        this.mergeMessage(optimistic)
        const ok = auth.ws.sendMessage(payload)
        if (!ok) {
          this.markMessageFailed(target.conversationId, clientMsgId)
          this.setError('转发中断：连接已断开')
          break
        }
        sent += 1
        if (sent < eligible.length) {
          await new Promise((r) => setTimeout(r, 40))
        }
      }
      if (sent > 0) {
        this.setTransientNotice(`已转发 ${sent} 条消息`)
        void this.pullSync()
      }
    },
    async sendText(text: string, reply?: ReplyMeta | null) {
      if (!this.activeConvId) {
        this.error = '请先选择会话'
        return
      }
      this.clearError()
      this.notifyTyping(false)
      const auth = useAuthStore()
      const clientMsgId = crypto.randomUUID()
      const content = reply ? buildReplyContent(reply, text) : text
      const payload: Record<string, unknown> = {
        client_msg_id: clientMsgId,
        conversation_id: this.activeConvId,
        msg_type: 1,
        content,
      }
      if (this.activeGroupId) {
        payload.conversation_type = 2
      } else {
        payload.conversation_type = 1
        payload.to_user_id = idStr(this.activeToUser)
      }
      const optimistic: ChatMessage = {
        client_msg_id: clientMsgId,
        conversation_id: this.activeConvId,
        from_user_id: idStr(auth.user?.id),
        msg_type: 1,
        content,
        created_at: new Date().toISOString(),
        status: 'sending',
      }
      this.mergeMessage(optimistic)
      const ok = auth.ws?.sendMessage(payload)
      if (!ok) {
        this.markMessageFailed(this.activeConvId, clientMsgId)
        this.error = '连接已断开，可点击重试'
        this.forceReconnect()
      } else {
        void this.pullSync()
      }
    },
    async sendPoll(question: string, optionTexts: string[]) {
      if (!this.activeConvId) {
        this.error = '请先选择会话'
        return
      }
      const q = question.trim()
      const opts = optionTexts.map((t) => t.trim()).filter(Boolean)
      if (!q || opts.length < 2) {
        this.setError('投票至少需要问题和 2 个选项')
        return
      }
      this.clearError()
      const auth = useAuthStore()
      const clientMsgId = crypto.randomUUID()
      const content = JSON.stringify({
        question: q,
        options: opts.slice(0, 8).map((text, i) => ({ id: `o${i + 1}`, text })),
      })
      const payload: Record<string, unknown> = {
        client_msg_id: clientMsgId,
        conversation_id: this.activeConvId,
        msg_type: 6,
        content,
      }
      if (this.activeGroupId) payload.conversation_type = 2
      else {
        payload.conversation_type = 1
        payload.to_user_id = idStr(this.activeToUser)
      }
      this.mergeMessage({
        client_msg_id: clientMsgId,
        conversation_id: this.activeConvId,
        from_user_id: idStr(auth.user?.id),
        msg_type: 6,
        content,
        created_at: new Date().toISOString(),
        status: 'sending',
      })
      const ok = auth.ws?.sendMessage(payload)
      if (!ok) {
        this.markMessageFailed(this.activeConvId, clientMsgId)
        this.error = '连接已断开，可点击重试'
        this.forceReconnect()
      } else {
        this.setTransientNotice('投票已发送')
        void this.pullSync()
      }
    },
    pollFor(msgId?: string | number) {
      if (msgId == null || msgId === '') return null
      return this.polls[idStr(msgId)] || null
    },
    async votePoll(msg: ChatMessage, optionId: string) {
      if (!msg.msg_id || !msg.conversation_id) return
      try {
        const { data } = await http.post(
          `/conversations/${msg.conversation_id}/messages/${msg.msg_id}/poll/vote`,
          { option_id: optionId },
        )
        const res = unwrapApiData<{
          poll: {
            msg_id: string
            total: number
            counts: { option_id: string; count: number }[]
            my_option_id?: string
          }
        }>(data)
        if (res.poll) {
          this.polls[idStr(msg.msg_id)] = { ...res.poll, msg_id: idStr(msg.msg_id) }
        }
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '投票失败')
      }
    },
    markMessageFailed(convId: string, clientMsgId: string) {
      const list = this.messages[convId] || []
      const idx = list.findIndex((m) => m.client_msg_id === clientMsgId)
      if (idx >= 0) {
        list[idx] = { ...list[idx], status: 'failed' }
        this.messages[convId] = [...list]
      }
    },
    async retryMessage(clientMsgId: string) {
      if (!this.activeConvId) return
      const list = this.messages[this.activeConvId] || []
      const m = list.find((x) => x.client_msg_id === clientMsgId)
      if (!m || m.status !== 'failed') return
      const auth = useAuthStore()
      m.status = 'sending'
      this.messages[this.activeConvId] = [...list]
      const payload: Record<string, unknown> = {
        client_msg_id: clientMsgId,
        conversation_id: this.activeConvId,
        msg_type: m.msg_type,
        content: m.content,
      }
      if (this.activeGroupId) {
        payload.conversation_type = 2
      } else {
        payload.conversation_type = 1
        payload.to_user_id = idStr(this.activeToUser)
      }
      const ok = auth.ws?.sendMessage(payload)
      if (!ok) {
        this.markMessageFailed(this.activeConvId, clientMsgId)
        this.error = '连接已断开，请稍后重试'
        this.forceReconnect()
      }
    },
    async recallMessage(msg: ChatMessage) {
      if (!msg.msg_id || !msg.conversation_id) {
        this.setError('无法撤回该消息')
        return
      }
      try {
        const { data } = await http.post(
          `/conversations/${msg.conversation_id}/messages/${msg.msg_id}/recall`,
        )
        const res = unwrapApiData<{ message: ChatMessage }>(data)
        const updated = res.message || { ...msg, msg_type: 4, content: '[已撤回]' }
        const list = this.messages[msg.conversation_id] || []
        const idx = list.findIndex((m) => m.client_msg_id === msg.client_msg_id)
        if (idx >= 0) {
          list[idx] = { ...list[idx], ...updated, msg_type: 4, content: '[已撤回]' }
          this.messages[msg.conversation_id] = [...list]
        }
        await this.loadConversations()
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '撤回失败')
      }
    },
    async editMessage(msg: ChatMessage, content: string) {
      if (!msg.msg_id || !msg.conversation_id) {
        this.setError('无法编辑该消息')
        return
      }
      const text = content.trim()
      if (!text) {
        this.setError('内容不能为空')
        return
      }
      try {
        const { data } = await http.post(
          `/conversations/${msg.conversation_id}/messages/${msg.msg_id}/edit`,
          { content: text },
        )
        const res = unwrapApiData<{ message: ChatMessage }>(data)
        const updated = res.message
        const list = this.messages[msg.conversation_id] || []
        const idx = list.findIndex((m) => m.client_msg_id === msg.client_msg_id)
        if (idx >= 0 && updated) {
          list[idx] = {
            ...list[idx],
            content: updated.content,
            edited_at: updated.edited_at || new Date().toISOString(),
          }
          this.messages[msg.conversation_id] = [...list]
        }
        await this.loadConversations()
        this.setTransientNotice('已编辑')
      } catch (e) {
        this.setError(e instanceof ApiError ? e.message : '编辑失败')
      }
    },
    async uploadAndSend(file: File) {
      if (!this.activeConvId) {
        this.error = '请先选择会话'
        return
      }
      this.clearError()
      const auth = useAuthStore()
      const clientMsgId = crypto.randomUUID()
      const isImage = file.type.startsWith('image/')
      const localPreview = isImage ? URL.createObjectURL(file) : ''
      const placeholderContent = JSON.stringify({
        url: localPreview || '',
        filename: file.name,
        content_type: file.type,
        size: file.size,
      })
      this.mergeMessage({
        client_msg_id: clientMsgId,
        conversation_id: this.activeConvId,
        from_user_id: idStr(auth.user?.id),
        msg_type: isImage ? 2 : 3,
        content: placeholderContent,
        created_at: new Date().toISOString(),
        status: 'uploading',
        uploadProgress: 0,
        localPreview,
      })
      this.uploading = true
      this.uploadPercent = 0
      try {
        const form = new FormData()
        form.append('file', file)
        const { data } = await http.post('/files/upload', form, {
          onUploadProgress: (e) => {
            if (!e.total) return
            const pct = Math.min(99, Math.round((e.loaded / e.total) * 100))
            this.uploadPercent = pct
            const list = this.messages[this.activeConvId] || []
            const idx = list.findIndex((m) => m.client_msg_id === clientMsgId)
            if (idx >= 0) {
              list[idx] = { ...list[idx], uploadProgress: pct, status: 'uploading' }
              this.messages[this.activeConvId] = [...list]
            }
          },
        })
        const res = unwrapApiData<FilePayload & { file_id: number; filename: string; content_type: string; url: string }>(data)
        const content = JSON.stringify({
          url: res.url,
          filename: res.filename || file.name,
          content_type: res.content_type || file.type,
          size: file.size,
        })
        const payload: Record<string, unknown> = {
          client_msg_id: clientMsgId,
          conversation_id: this.activeConvId,
          msg_type: isImage ? 2 : 3,
          content,
        }
        if (this.activeGroupId) {
          payload.conversation_type = 2
        } else {
          payload.conversation_type = 1
          payload.to_user_id = idStr(this.activeToUser)
        }
        this.mergeMessage({
          client_msg_id: clientMsgId,
          conversation_id: this.activeConvId,
          from_user_id: idStr(auth.user?.id),
          msg_type: isImage ? 2 : 3,
          content,
          status: 'sending',
          uploadProgress: 100,
          localPreview: undefined,
        })
        if (localPreview) URL.revokeObjectURL(localPreview)
        const ok = auth.ws?.sendMessage(payload)
        if (!ok) {
          this.markMessageFailed(this.activeConvId, clientMsgId)
          this.error = '连接已断开，正在重连…'
          this.forceReconnect()
        } else {
          void this.pullSync()
        }
      } catch (e) {
        this.markMessageFailed(this.activeConvId, clientMsgId)
        this.setError(e instanceof ApiError ? e.message : '上传失败')
        if (localPreview) URL.revokeObjectURL(localPreview)
      } finally {
        this.uploading = false
        this.uploadPercent = 0
      }
    },
    async sendVoice(blob: Blob, durationSec: number) {
      if (!this.activeConvId) {
        this.error = '请先选择会话'
        return
      }
      this.clearError()
      const auth = useAuthStore()
      const clientMsgId = crypto.randomUUID()
      const localUrl = URL.createObjectURL(blob)
      const contentType = blob.type || 'audio/webm'
      const ext = contentType.includes('mp4') ? 'm4a' : contentType.includes('ogg') ? 'ogg' : 'webm'
      const file = new File([blob], `voice-${Date.now()}.${ext}`, { type: contentType })
      const placeholder = JSON.stringify({
        url: localUrl,
        filename: file.name,
        content_type: contentType,
        size: file.size,
        duration: durationSec,
      })
      this.mergeMessage({
        client_msg_id: clientMsgId,
        conversation_id: this.activeConvId,
        from_user_id: idStr(auth.user?.id),
        msg_type: 5,
        content: placeholder,
        created_at: new Date().toISOString(),
        status: 'uploading',
        uploadProgress: 0,
        localPreview: localUrl,
      })
      this.uploading = true
      try {
        const form = new FormData()
        form.append('file', file)
        const { data } = await http.post('/files/upload', form, {
          onUploadProgress: (e) => {
            if (!e.total) return
            const pct = Math.min(99, Math.round((e.loaded / e.total) * 100))
            this.uploadPercent = pct
            const list = this.messages[this.activeConvId] || []
            const idx = list.findIndex((m) => m.client_msg_id === clientMsgId)
            if (idx >= 0) {
              list[idx] = { ...list[idx], uploadProgress: pct, status: 'uploading' }
              this.messages[this.activeConvId] = [...list]
            }
          },
        })
        const res = unwrapApiData<FilePayload & { url: string; filename: string; content_type: string }>(data)
        const content = JSON.stringify({
          url: res.url,
          filename: res.filename || file.name,
          content_type: res.content_type || contentType,
          size: file.size,
          duration: Math.max(1, Math.round(durationSec)),
        })
        const payload: Record<string, unknown> = {
          client_msg_id: clientMsgId,
          conversation_id: this.activeConvId,
          msg_type: 5,
          content,
        }
        if (this.activeGroupId) payload.conversation_type = 2
        else {
          payload.conversation_type = 1
          payload.to_user_id = idStr(this.activeToUser)
        }
        this.mergeMessage({
          client_msg_id: clientMsgId,
          conversation_id: this.activeConvId,
          from_user_id: idStr(auth.user?.id),
          msg_type: 5,
          content,
          status: 'sending',
          uploadProgress: 100,
          localPreview: undefined,
        })
        URL.revokeObjectURL(localUrl)
        const ok = auth.ws?.sendMessage(payload)
        if (!ok) {
          this.markMessageFailed(this.activeConvId, clientMsgId)
          this.error = '连接已断开，正在重连…'
          this.forceReconnect()
        } else {
          void this.pullSync()
        }
      } catch (e) {
        this.markMessageFailed(this.activeConvId, clientMsgId)
        this.setError(e instanceof ApiError ? e.message : '语音发送失败')
        URL.revokeObjectURL(localUrl)
      } finally {
        this.uploading = false
        this.uploadPercent = 0
      }
    },
    async pullSync() {
      const auth = useAuthStore()
      const { data } = await http.get('/sync', {
        params: { since_seq: this.sinceSeq, device_id: auth.deviceId },
      })
      const res = unwrapApiData<{ messages: ChatMessage[]; next_cursor: number }>(data)
      for (const m of res.messages || []) {
        m.from_user_id = idStr(m.from_user_id)
        this.mergeMessage(m)
      }
      if (res.next_cursor) this.sinceSeq = res.next_cursor
      await this.loadConversations()
    },
  },
})
