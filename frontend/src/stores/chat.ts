import { defineStore } from 'pinia'
import http, { unwrapApiData } from '../api/http'
import { ApiError } from '../api/errors'
import type { AckPayload, WSFrame, WSStatus } from '../api/ws'
import { useAuthStore, type PublicProfile } from './auth'
import { directConvId, idStr, sameId, type UserId } from '../utils/id'
import { isAgentProfile, AGENT_NICKNAME } from '../constants/agent'
import { previewMessage } from '../utils/format'
import { buildReplyContent, type ReplyMeta } from '../utils/reply'
import { showMessageNotification } from '../utils/notify'

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
  status?: 'sending' | 'sent' | 'failed' | 'uploading'
  /** 0–100 while uploading */
  uploadProgress?: number
  /** blob: URL for local preview before upload finishes */
  localPreview?: string
}

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
    highlightClientMsgId: '',
    highlightTimer: 0 as number,
    /** conversation_id -> peer user ids currently typing */
    typingUsers: {} as Record<string, string[]>,
    typingClearTimers: {} as Record<string, number>,
    lastTypingSentAt: 0,
    /** group_id -> members (for @mention) */
    groupMembers: {} as Record<string, PublicProfile[]>,
    groupMemberRoles: {} as Record<string, Record<string, number>>,
    historyHasMore: {} as Record<string, boolean>,
    peerReadSeq: {} as Record<string, number>,
    pinnedFriendIds: initialPinned.friends,
    pinnedGroupIds: initialPinned.groups,
    mutedConvIds: initialMuted,
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
      if (frame.type === 'typing') {
        const p = frame.payload as { conversation_id?: string; user_id?: string; typing?: boolean }
        if (!p?.conversation_id || !p.user_id) return
        this.applyPeerTyping(p.conversation_id, idStr(p.user_id), p.typing !== false)
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
        if (
          !this.isMuted(msg.conversation_id) &&
          (document.hidden || msg.conversation_id !== this.activeConvId)
        ) {
          const friend = this.friends.find((f) => sameId(f.id, msg.from_user_id))
          const title = friend ? friendDisplayName(friend) : '新消息'
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
    async loadFriends() {
      const { data } = await http.get('/friends')
      const res = unwrapApiData<{ friends: (PublicProfile & { remark?: string })[] }>(data)
      this.friends = (res.friends || []).map((f) => ({ ...f, id: idStr(f.id) }))
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
        member_roles?: Record<string, number>
        members: PublicProfile[]
      }>(data)
      this.groupMembers[gid] = (res.members || []).map((m) => ({ ...m, id: idStr(m.id) }))
      if (res.member_roles) {
        this.groupMemberRoles[gid] = res.member_roles
      }
      const idx = this.groups.findIndex((g) => g.id === gid)
      if (idx >= 0) {
        this.groups[idx] = {
          ...this.groups[idx],
          notice: res.notice || '',
          name: res.name,
          owner_id: idStr(res.owner_id),
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
    mentionName(user: PublicProfile) {
      const friend = this.friends.find((f) => sameId(f.id, user.id))
      if (friend) return friendDisplayName(friend)
      return user.nickname || user.username
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
    },
    togglePinGroup(groupId: string) {
      const id = idStr(groupId)
      const i = this.pinnedGroupIds.indexOf(id)
      if (i >= 0) this.pinnedGroupIds.splice(i, 1)
      else this.pinnedGroupIds.unshift(id)
      savePinned([...this.pinnedFriendIds], [...this.pinnedGroupIds])
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
    toggleMute(convId: string) {
      if (!convId) return
      const i = this.mutedConvIds.indexOf(convId)
      if (i >= 0) this.mutedConvIds.splice(i, 1)
      else this.mutedConvIds.push(convId)
      saveMuted([...this.mutedConvIds])
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
      const res = unwrapApiData<{ messages: ChatMessage[] }>(data)
      const merged = (res.messages || []).map((m) => ({ ...m, from_user_id: idStr(m.from_user_id) }))
      merged.sort((a, b) => (a.seq || 0) - (b.seq || 0))
      this.messages[convId] = merged
      this.historyHasMore[convId] = merged.length >= 50
    },
    async loadMoreHistory(convId: string) {
      const list = this.messages[convId] || []
      const minSeq = list.reduce((n, m) => (m.seq && m.seq < n ? m.seq : n), list[0]?.seq || 0)
      if (!minSeq) return
      const { data } = await http.get(`/conversations/${convId}/messages`, {
        params: { limit: 50, before_seq: minSeq },
      })
      const res = unwrapApiData<{ messages: ChatMessage[] }>(data)
      const older = (res.messages || []).map((m) => ({ ...m, from_user_id: idStr(m.from_user_id) }))
      if (older.length < 50) this.historyHasMore[convId] = false
      const merged = [...older]
      for (const m of list) {
        if (!merged.some((x) => x.client_msg_id === m.client_msg_id)) merged.push(m)
      }
      merged.sort((a, b) => (a.seq || 0) - (b.seq || 0))
      this.messages[convId] = merged
    },
    async searchMessages(q: string) {
      if (!this.activeConvId) return
      const keyword = q.trim()
      this.searchQuery = keyword
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
    clearLocalMessages(convId?: string) {
      const id = convId || this.activeConvId
      if (!id) return
      this.messages[id] = []
      this.historyHasMore[id] = true
      this.clearHighlight()
      this.setTransientNotice('已清空本地消息，可重新加载历史')
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
        const peer = (res.members || []).find((m) => !sameId(m.user_id, auth.user?.id))
        if (peer) this.peerReadSeq[convId] = peer.read_seq
      } catch {
        /* ignore */
      }
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
      await this.markRead(this.activeConvId)
    },
    async openGroup(group: GroupItem) {
      this.activeConvId = group.conversation_id
      this.activeGroupId = group.id
      this.activeToUser = ''
      this.activeTitle = group.name
      this.clearError()
      this.clearSearch()
      await Promise.allSettled([this.fetchGroup(group.id), this.loadHistory(this.activeConvId)])
      await this.markRead(this.activeConvId)
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
