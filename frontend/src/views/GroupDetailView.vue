<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { parseError } from '../api/errors'
import UserAvatar from '../components/UserAvatar.vue'
import { friendDisplayName, useChatStore } from '../stores/chat'
import { useAuthStore, type PublicProfile } from '../stores/auth'
import { idStr, sameId } from '../utils/id'
import { mediaUrl } from '../utils/media'

const route = useRoute()
const router = useRouter()
const chat = useChatStore()
const auth = useAuthStore()

const loading = ref(true)
const name = ref('')
const groupNo = ref('')
const ownerId = ref('')
const conversationId = ref('')
const members = ref<PublicProfile[]>([])
const memberRoles = ref<Record<string, number>>({})
const notice = ref('')
const noticeDraft = ref('')
const editingNotice = ref(false)
const savingNotice = ref(false)
const showInvite = ref(false)
const inviteSelected = ref<string[]>([])
const inviting = ref(false)
const pendingInvites = ref<
  {
    id: string
    to_user_id: string
    to_name: string
    to_avatar?: string
    message: string
    invite_type: number
    created_at: string
  }[]
>([])
const showTransfer = ref(false)
const transferTarget = ref('')
const leaving = ref(false)

const groupId = () => idStr(String(route.params.id || ''))
const isOwner = computed(() => sameId(ownerId.value, auth.user?.id))

const myRole = computed(() => memberRoles.value[idStr(auth.user?.id || '')] ?? 0)

const isManager = computed(() => isOwner.value || myRole.value >= 1)

const invitableFriends = computed(() => {
  const memberIds = new Set(members.value.map((m) => idStr(m.id)))
  return chat.friends.filter((f) => f.username !== 'squirtle_ai' && !memberIds.has(idStr(f.id)))
})

const sortedMembers = computed(() => {
  const list = [...members.value]
  list.sort((a, b) => {
    const ao = sameId(a.id, ownerId.value) ? 1 : 0
    const bo = sameId(b.id, ownerId.value) ? 1 : 0
    if (ao !== bo) return bo - ao
    const aself = sameId(a.id, auth.user?.id) ? 1 : 0
    const bself = sameId(b.id, auth.user?.id) ? 1 : 0
    if (aself !== bself) return bself - aself
    return memberLabel(a).localeCompare(memberLabel(b), 'zh-CN')
  })
  return list
})

function memberLabel(m: PublicProfile) {
  const friend = chat.friends.find((f) => sameId(f.id, m.id))
  if (friend) return friendDisplayName(friend)
  return m.nickname || m.username
}

onMounted(async () => {
  try {
    const g = await chat.fetchGroup(groupId())
    name.value = g.name
    groupNo.value = g.group_no || ''
    ownerId.value = idStr(g.owner_id)
    conversationId.value = g.conversation_id
    notice.value = g.notice || ''
    noticeDraft.value = notice.value
    members.value = (g.members || []).map((m) => ({ ...m, id: idStr(m.id) }))
    memberRoles.value = g.member_roles || {}
    await loadPendingInvites()
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    loading.value = false
  }
})

function startEditNotice() {
  noticeDraft.value = notice.value
  editingNotice.value = true
}

async function saveNotice() {
  savingNotice.value = true
  try {
    await chat.setGroupNotice(groupId(), noticeDraft.value)
    notice.value = noticeDraft.value.trim()
    editingNotice.value = false
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    savingNotice.value = false
  }
}

function goBack() {
  router.push('/')
}

async function openChat() {
  const g = chat.groups.find((x) => x.id === groupId())
  if (g) {
    await chat.openGroup(g)
  } else {
    await chat.loadGroups()
    const found = chat.groups.find((x) => x.id === groupId())
    if (found) await chat.openGroup(found)
  }
  router.push('/')
}

function goMember(id: string) {
  router.push(`/profile/${id}`)
}

function avatarUrl(url?: string) {
  return mediaUrl(url)
}

function toggleInviteFriend(id: string) {
  const i = inviteSelected.value.indexOf(id)
  if (i >= 0) inviteSelected.value.splice(i, 1)
  else inviteSelected.value.push(id)
}

async function sendInvites() {
  if (!inviteSelected.value.length) return
  inviting.value = true
  try {
    await chat.inviteGroupMembers(groupId(), inviteSelected.value)
    inviteSelected.value = []
    showInvite.value = false
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    inviting.value = false
  }
}

function memberRole(m: PublicProfile) {
  return memberRoles.value[idStr(m.id)] ?? 0
}

function isAdmin(m: PublicProfile) {
  return memberRole(m) === 1
}

async function toggleAdmin(m: PublicProfile) {
  if (!isOwner.value || sameId(m.id, ownerId.value)) return
  try {
    await chat.setGroupAdmin(groupId(), m.id, !isAdmin(m))
    const g = await chat.fetchGroup(groupId())
    memberRoles.value = g.member_roles || {}
  } catch (e) {
    chat.setError(parseError(e))
  }
}

const transferCandidates = computed(() =>
  members.value.filter((m) => !sameId(m.id, ownerId.value)),
)

function canKick(m: PublicProfile) {
  if (sameId(m.id, auth.user?.id)) return false
  if (sameId(m.id, ownerId.value)) return false
  if (isOwner.value) return !sameId(m.id, auth.user?.id)
  if (isManager.value && !isAdmin(m)) return true
  return false
}

async function loadPendingInvites() {
  if (!isManager.value) return
  try {
    pendingInvites.value = await chat.listGroupPendingInvites(groupId())
  } catch {
    pendingInvites.value = []
  }
}

async function cancelInvite(inviteId: string) {
  try {
    await chat.cancelGroupInvite(groupId(), inviteId)
    await loadPendingInvites()
  } catch (e) {
    chat.setError(parseError(e))
  }
}

async function kickMember(m: PublicProfile) {
  if (!canKick(m)) return
  if (!confirm(`确定将 ${memberLabel(m)} 移出群聊？`)) return
  try {
    await chat.kickGroupMember(groupId(), m.id)
    members.value = members.value.filter((x) => !sameId(x.id, m.id))
    delete memberRoles.value[idStr(m.id)]
  } catch (e) {
    chat.setError(parseError(e))
  }
}

async function doTransfer() {
  if (!transferTarget.value) return
  try {
    await chat.transferGroupOwner(groupId(), transferTarget.value)
    ownerId.value = transferTarget.value
    const g = await chat.fetchGroup(groupId())
    memberRoles.value = g.member_roles || {}
    showTransfer.value = false
    transferTarget.value = ''
  } catch (e) {
    chat.setError(parseError(e))
  }
}

async function leaveGroup() {
  if (!confirm('确定退出该群聊？')) return
  leaving.value = true
  try {
    await chat.leaveGroup(groupId())
    router.push('/')
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    leaving.value = false
  }
}
</script>

<template>
  <div class="group-page">
    <header class="page-header">
      <button type="button" class="btn btn-ghost btn-sm" @click="goBack">← 返回</button>
      <h1>群聊信息</h1>
      <span />
    </header>

    <div v-if="loading" class="loading" aria-busy="true">
      <div class="g-skel-avatar" />
      <div class="g-skel-line w40" />
      <div class="g-skel-line w25" />
      <div class="g-skel-block" />
      <div class="g-skel-block short" />
    </div>
    <div v-else class="body">
      <div class="hero">
        <UserAvatar name="群" :size="72" />
        <h2>{{ name }}</h2>
        <p v-if="groupNo" class="meta">群号 {{ groupNo }}</p>
        <p class="meta">{{ members.length }} 位成员</p>
      </div>

      <button type="button" class="btn btn-primary btn-block" @click="openChat">进入群聊</button>

      <button
        v-if="isManager && invitableFriends.length"
        type="button"
        class="btn btn-secondary btn-block"
        @click="showInvite = !showInvite"
      >
        {{ showInvite ? '收起邀请' : '邀请好友进群' }}
      </button>

      <section v-if="showInvite && isManager" class="invite-section">
        <p class="invite-hint">勾选好友发送入群邀请（对方接受后加入）</p>
        <ul class="invite-list">
          <li
            v-for="f in invitableFriends"
            :key="f.id"
            class="invite-item"
            :class="{ selected: inviteSelected.includes(f.id) }"
            @click="toggleInviteFriend(f.id)"
          >
            <UserAvatar :src="avatarUrl(f.avatar)" :name="memberLabel(f)" :size="32" />
            <span>{{ memberLabel(f) }}</span>
            <span class="check">{{ inviteSelected.includes(f.id) ? '✓' : '' }}</span>
          </li>
        </ul>
        <button
          type="button"
          class="btn btn-primary btn-sm"
          :disabled="!inviteSelected.length || inviting"
          @click="sendInvites"
        >
          {{ inviting ? '发送中…' : '发送邀请' }}
        </button>
      </section>

      <section v-if="isManager && pendingInvites.length" class="pending-invites">
        <h3>待处理入群邀请（{{ pendingInvites.length }}）</h3>
        <ul>
          <li v-for="inv in pendingInvites" :key="inv.id" class="pending-invite-row">
            <UserAvatar :src="avatarUrl(inv.to_avatar)" :name="inv.to_name" :size="32" />
            <div class="pending-invite-meta">
              <span>{{ inv.to_name }}</span>
              <span class="pending-invite-sub">{{ inv.message || '等待对方接受' }}</span>
            </div>
            <button type="button" class="btn btn-ghost btn-sm" @click="cancelInvite(inv.id)">撤销</button>
          </li>
        </ul>
      </section>

      <section class="notice-section">
        <div class="notice-head">
          <h3>群公告</h3>
          <button
            v-if="isOwner && !editingNotice"
            type="button"
            class="btn btn-ghost btn-sm"
            @click="startEditNotice"
          >
            {{ notice ? '编辑' : '设置' }}
          </button>
        </div>
        <div v-if="editingNotice" class="notice-edit">
          <textarea
            v-model="noticeDraft"
            class="notice-input"
            rows="3"
            maxlength="200"
            placeholder="输入群公告（最多 200 字）"
          />
          <div class="notice-actions">
            <button type="button" class="btn btn-secondary btn-sm" @click="editingNotice = false">取消</button>
            <button type="button" class="btn btn-primary btn-sm" :disabled="savingNotice" @click="saveNotice">
              {{ savingNotice ? '保存中…' : '保存' }}
            </button>
          </div>
        </div>
        <p v-else class="notice-body">{{ notice || '暂无公告' }}</p>
      </section>

      <section class="members">
        <h3>群成员（{{ members.length }}）</h3>
        <ul>
          <li v-for="m in sortedMembers" :key="m.id" class="member-row" @click="goMember(m.id)">
            <UserAvatar :src="avatarUrl(m.avatar)" :name="memberLabel(m)" :size="40" />
            <div class="member-info">
              <span class="member-name">
                {{ memberLabel(m) }}
                <span v-if="sameId(m.id, auth.user?.id)" class="me-tag">我</span>
              </span>
              <span class="member-sub">@{{ m.username }}</span>
            </div>
            <span v-if="sameId(m.id, ownerId)" class="tag">群主</span>
            <span v-else-if="isAdmin(m)" class="tag admin">管理员</span>
            <button
              v-if="isOwner && !sameId(m.id, ownerId) && !sameId(m.id, auth.user?.id)"
              type="button"
              class="btn btn-ghost btn-sm admin-btn"
              @click.stop="toggleAdmin(m)"
            >
              {{ isAdmin(m) ? '取消管理' : '设管理员' }}
            </button>
            <button
              v-if="canKick(m)"
              type="button"
              class="btn btn-ghost btn-sm kick-btn"
              @click.stop="kickMember(m)"
            >
              移出
            </button>
          </li>
        </ul>
      </section>

      <section v-if="isOwner && transferCandidates.length" class="transfer-section">
        <button type="button" class="btn btn-secondary btn-block" @click="showTransfer = !showTransfer">
          {{ showTransfer ? '取消转让' : '转让群主' }}
        </button>
        <div v-if="showTransfer" class="transfer-box">
          <select v-model="transferTarget" class="input">
            <option value="">选择新群主</option>
            <option v-for="m in transferCandidates" :key="m.id" :value="m.id">
              {{ memberLabel(m) }}
            </option>
          </select>
          <button type="button" class="btn btn-primary btn-sm" :disabled="!transferTarget" @click="doTransfer">
            确认转让
          </button>
        </div>
      </section>

      <button
        v-if="!isOwner"
        type="button"
        class="btn btn-ghost btn-block leave-btn"
        :disabled="leaving"
        @click="leaveGroup"
      >
        {{ leaving ? '退出中…' : '退出群聊' }}
      </button>
      <p v-else class="owner-leave-hint">群主需先转让群主后才能退出</p>
    </div>
  </div>
</template>

<style scoped>
.group-page {
  min-height: 100vh;
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

.loading {
  max-width: 480px;
  margin: 0 auto;
  padding: var(--space-8) var(--space-4);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-3);
}

.g-skel-avatar {
  width: 72px;
  height: 72px;
  border-radius: 50%;
  background: linear-gradient(90deg, #e2e8f0 25%, #f1f5f9 50%, #e2e8f0 75%);
  background-size: 200% 100%;
  animation: g-shimmer 1.2s ease-in-out infinite;
}

.g-skel-line {
  height: 12px;
  border-radius: var(--radius-sm);
  background: linear-gradient(90deg, #e2e8f0 25%, #f1f5f9 50%, #e2e8f0 75%);
  background-size: 200% 100%;
  animation: g-shimmer 1.2s ease-in-out infinite;
}
.g-skel-line.w25 { width: 25%; }
.g-skel-line.w40 { width: 40%; }

.g-skel-block {
  width: 100%;
  height: 72px;
  margin-top: var(--space-3);
  border-radius: var(--radius-md);
  background: linear-gradient(90deg, #e2e8f0 25%, #f1f5f9 50%, #e2e8f0 75%);
  background-size: 200% 100%;
  animation: g-shimmer 1.2s ease-in-out infinite;
}
.g-skel-block.short { height: 120px; }

@keyframes g-shimmer {
  0% { background-position: 100% 0; }
  100% { background-position: -100% 0; }
}

.body {
  max-width: 480px;
  margin: 0 auto;
  padding: var(--space-6) var(--space-4);
  animation: body-in var(--transition-base, 200ms ease);
}

@keyframes body-in {
  from { opacity: 0; transform: translateY(6px); }
  to { opacity: 1; transform: translateY(0); }
}

.hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-2);
  margin-bottom: var(--space-5);
}

.hero h2 {
  margin: var(--space-2) 0 0;
}

.meta {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.notice-section {
  margin-top: var(--space-6);
  padding: var(--space-4);
  background: var(--color-bg-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.notice-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-2);
}

.notice-head h3 {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.notice-body {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-text);
  white-space: pre-wrap;
  word-break: break-word;
}

.notice-input {
  width: 100%;
  padding: var(--space-2) var(--space-3);
  font: inherit;
  font-size: var(--text-sm);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  resize: vertical;
  background: var(--color-bg-chat);
}

.notice-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  margin-top: var(--space-2);
}

.members {
  margin-top: var(--space-6);
}

.members h3 {
  margin: 0 0 var(--space-3);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.invite-section {
  padding: var(--space-3);
  background: var(--color-bg-muted);
  border-radius: var(--radius-md);
}

.invite-hint {
  margin: 0 0 var(--space-2);
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}

.invite-list {
  list-style: none;
  margin: 0 0 var(--space-3);
  padding: 0;
  max-height: 180px;
  overflow-y: auto;
}

.invite-item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2);
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--text-sm);
}

.invite-item.selected {
  background: var(--color-primary-muted);
}

.tag.admin {
  background: #dbeafe;
  color: #1d4ed8;
}

.admin-btn {
  margin-left: auto;
  flex-shrink: 0;
}

.pending-invites h3 {
  margin: 0 0 var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.pending-invite-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) 0;
  border-bottom: 1px solid var(--color-border);
}

.pending-invite-meta {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  font-size: var(--text-sm);
}

.pending-invite-sub {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}

.kick-btn {
  color: #dc2626;
  flex-shrink: 0;
}

.transfer-section {
  margin-top: var(--space-2);
}

.transfer-box {
  margin-top: var(--space-3);
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.leave-btn {
  margin-top: var(--space-4);
  color: #dc2626;
}

.owner-leave-hint {
  margin: var(--space-4) 0 0;
  text-align: center;
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}

.members ul {
  list-style: none;
  margin: 0;
  padding: 0;
  background: var(--color-bg-surface);
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
}

.member-row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  cursor: pointer;
  border-bottom: 1px solid var(--color-border);
}

.member-row:last-child {
  border-bottom: none;
}

.member-row:hover {
  background: var(--color-primary-muted);
}

.member-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.member-name {
  font-size: var(--text-sm);
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 6px;
}

.member-sub {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}

.me-tag {
  font-size: var(--text-xs);
  font-weight: 500;
  color: var(--color-text-muted);
}

.tag {
  font-size: var(--text-xs);
  color: var(--color-primary);
  background: var(--color-primary-muted);
  padding: 2px 8px;
  border-radius: var(--radius-full);
}
</style>
