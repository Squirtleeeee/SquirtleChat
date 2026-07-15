<script setup lang="ts">
import { computed, ref } from 'vue'
import { parseError } from '../api/errors'
import UserAvatar from './UserAvatar.vue'
import { friendDisplayName, useChatStore, type GroupPublicItem } from '../stores/chat'
import type { PublicProfile } from '../stores/auth'

const emit = defineEmits<{ close: [] }>()

const chat = useChatStore()
const tab = ref<'friend' | 'group' | 'create' | 'face' | 'link'>('friend')
const query = ref('')
const friendResults = ref<PublicProfile[]>([])
const groupResults = ref<GroupPublicItem[]>([])
const selectedGroup = ref<GroupPublicItem | null>(null)
const groupName = ref('')
const selectedFriendIds = ref<string[]>([])
const friendMessage = ref('')
const ownerFaceCode = ref('')
const memberFaceCode = ref('')
const faceStarting = ref(false)
const faceJoining = ref(false)
const loading = ref(false)
const inviteCodeInput = ref('')
const invitePreview = ref<{
  code: string
  group_name: string
  group_no: string
  member_count: number
  is_member?: boolean
  usable?: boolean
} | null>(null)
const linkJoining = ref(false)

const selectableFriends = computed(() =>
  chat.friends.filter((f) => f.username !== 'squirtle_ai'),
)

function toggleFriend(id: string) {
  const i = selectedFriendIds.value.indexOf(id)
  if (i >= 0) selectedFriendIds.value.splice(i, 1)
  else selectedFriendIds.value.push(id)
}

function isSelected(id: string) {
  return selectedFriendIds.value.includes(id)
}

async function searchFriend() {
  const q = query.value.trim()
  if (!q) return
  loading.value = true
  try {
    friendResults.value = await chat.searchUsers(q)
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    loading.value = false
  }
}

async function searchGroup() {
  const q = query.value.trim()
  if (!q) return
  loading.value = true
  selectedGroup.value = null
  try {
    groupResults.value = await chat.searchGroups(q)
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    loading.value = false
  }
}

function selectGroup(g: GroupPublicItem) {
  selectedGroup.value = g
}

async function addFriend(id: string) {
  try {
    await chat.requestFriend(id, friendMessage.value)
    friendResults.value = friendResults.value.filter((u) => u.id !== id)
    friendMessage.value = ''
  } catch (e) {
    chat.setError(parseError(e))
  }
}

async function requestJoinGroup(g: GroupPublicItem) {
  try {
    await chat.joinGroupByNo(g.group_no)
  } catch (e) {
    chat.setError(parseError(e))
  }
}

function extractInviteCode(raw: string) {
  const t = raw.trim()
  const m = t.match(/([A-Za-z0-9]{8})\s*$/)
  return m?.[1] || t
}

async function previewInvite() {
  const code = extractInviteCode(inviteCodeInput.value)
  if (!code) return
  loading.value = true
  invitePreview.value = null
  try {
    invitePreview.value = await chat.previewInviteLink(code)
    inviteCodeInput.value = invitePreview.value.code
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    loading.value = false
  }
}

async function joinInvite() {
  const code = invitePreview.value?.code || extractInviteCode(inviteCodeInput.value)
  if (!code) return
  linkJoining.value = true
  try {
    await chat.joinViaInviteLink(code)
    emit('close')
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    linkJoining.value = false
  }
}

async function createGroupWithFriends() {
  const name = groupName.value.trim()
  if (!name) {
    chat.setError('请输入群名称')
    return
  }
  try {
    const res = await chat.createGroup(name, selectedFriendIds.value)
    groupName.value = ''
    selectedFriendIds.value = []
    const g = chat.groups.find((x) => x.conversation_id === res.conversation_id)
    if (g) await chat.openGroup(g)
    emit('close')
  } catch (e) {
    chat.setError(parseError(e))
  }
}

async function startFaceAsOwner() {
  const code = ownerFaceCode.value.trim()
  if (!/^\d{4}$/.test(code)) {
    chat.setError('请输入 4 位数字建群码')
    return
  }
  faceStarting.value = true
  try {
    const res = await chat.startFaceToFace(code)
    const g = chat.groups.find((x) => x.conversation_id === res.conversation_id)
    if (g) await chat.openGroup(g)
    emit('close')
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    faceStarting.value = false
  }
}

async function joinFaceAsMember() {
  const code = memberFaceCode.value.trim()
  if (!/^\d{4}$/.test(code)) {
    chat.setError('请输入 4 位数字建群码')
    return
  }
  faceJoining.value = true
  try {
    const res = await chat.joinFaceToFace(code)
    memberFaceCode.value = ''
    if (res.conversation_id) {
      const g = chat.groups.find((x) => x.conversation_id === res.conversation_id)
      if (g) await chat.openGroup(g)
    }
    emit('close')
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    faceJoining.value = false
  }
}

function displayName(u: PublicProfile) {
  return u.nickname || u.username
}
</script>

<template>
  <div class="modal-backdrop" @click.self="emit('close')">
    <div class="modal-card add-modal" role="dialog" aria-labelledby="add-title">
      <header class="modal-header">
        <h3 id="add-title">添加好友 / 群聊</h3>
        <button type="button" class="btn btn-ghost btn-sm" aria-label="关闭" @click="emit('close')">×</button>
      </header>

      <div class="modal-tabs">
        <button type="button" class="modal-tab" :class="{ active: tab === 'friend' }" @click="tab = 'friend'">搜索好友</button>
        <button type="button" class="modal-tab" :class="{ active: tab === 'group' }" @click="tab = 'group'">搜索群聊</button>
        <button type="button" class="modal-tab" :class="{ active: tab === 'link' }" @click="tab = 'link'">邀请码</button>
        <button type="button" class="modal-tab" :class="{ active: tab === 'create' }" @click="tab = 'create'">创建群聊</button>
        <button type="button" class="modal-tab" :class="{ active: tab === 'face' }" @click="tab = 'face'">面对面建群</button>
      </div>

      <div v-if="tab === 'friend'" class="modal-body">
        <p class="field-hint">输入用户名或用户 ID 搜索</p>
        <div class="search-row">
          <input v-model="query" class="input" type="text" placeholder="用户名 / ID" @keyup.enter="searchFriend" />
          <button type="button" class="btn btn-primary btn-sm" :disabled="loading" @click="searchFriend">搜索</button>
        </div>
        <input v-model="friendMessage" class="input" type="text" maxlength="100" placeholder="验证消息（可选）" />
        <ul v-if="friendResults.length" class="result-list">
          <li v-for="u in friendResults" :key="u.id" class="result-item">
            <UserAvatar :src="u.avatar" :name="displayName(u)" :size="36" />
            <span class="result-name">{{ displayName(u) }}</span>
            <button type="button" class="btn btn-primary btn-sm" @click="addFriend(u.id)">添加</button>
          </li>
        </ul>
        <p v-else-if="query && !loading" class="empty-search">未找到用户</p>
      </div>

      <div v-else-if="tab === 'group'" class="modal-body">
        <p class="field-hint">输入群名称或约 10 位群号搜索，查看信息后申请加入</p>
        <div class="search-row">
          <input
            v-model="query"
            class="input"
            type="text"
            placeholder="群名称 / 群号"
            @keyup.enter="searchGroup"
          />
          <button type="button" class="btn btn-primary btn-sm" :disabled="loading" @click="searchGroup">搜索</button>
        </div>
        <ul v-if="groupResults.length" class="result-list">
          <li
            v-for="g in groupResults"
            :key="g.id"
            class="result-item clickable"
            :class="{ selected: selectedGroup?.id === g.id }"
            @click="selectGroup(g)"
          >
            <UserAvatar name="群" :size="36" />
            <div class="result-meta">
              <span class="result-name">{{ g.name }}</span>
              <span class="result-sub">群号 {{ g.group_no }} · {{ g.member_count }} 人</span>
            </div>
          </li>
        </ul>
        <p v-else-if="query && !loading" class="empty-search">未找到群聊</p>

        <div v-if="selectedGroup" class="group-preview">
          <h4>{{ selectedGroup.name }}</h4>
          <p class="preview-meta">群号 {{ selectedGroup.group_no }} · {{ selectedGroup.member_count }} 位成员</p>
          <button
            v-if="selectedGroup.is_member"
            type="button"
            class="btn btn-secondary btn-block"
            disabled
          >
            您已在该群中
          </button>
          <button
            v-else
            type="button"
            class="btn btn-primary btn-block"
            @click="requestJoinGroup(selectedGroup)"
          >
            申请加入
          </button>
        </div>
      </div>

      <div v-else-if="tab === 'link'" class="modal-body">
        <p class="field-hint">粘贴群邀请码（8 位），预览后直接加入</p>
        <div class="search-row">
          <input
            v-model="inviteCodeInput"
            class="input"
            type="text"
            maxlength="64"
            placeholder="邀请码"
            @keyup.enter="previewInvite"
          />
          <button type="button" class="btn btn-primary btn-sm" :disabled="loading" @click="previewInvite">
            预览
          </button>
        </div>
        <div v-if="invitePreview" class="group-preview">
          <h4>{{ invitePreview.group_name }}</h4>
          <p class="preview-meta">
            群号 {{ invitePreview.group_no }} · {{ invitePreview.member_count }} 位成员
            <template v-if="invitePreview.usable === false"> · 链接不可用</template>
          </p>
          <button
            v-if="invitePreview.is_member"
            type="button"
            class="btn btn-secondary btn-block"
            disabled
          >
            您已在该群中
          </button>
          <button
            v-else
            type="button"
            class="btn btn-primary btn-block"
            :disabled="linkJoining || invitePreview.usable === false"
            @click="joinInvite"
          >
            {{ linkJoining ? '加入中…' : '加入群聊' }}
          </button>
        </div>
      </div>

      <div v-else-if="tab === 'create'" class="modal-body">
        <input v-model="groupName" class="input" type="text" placeholder="群名称" />
        <p class="field-hint">勾选好友发送入群邀请（对方接受后加入）</p>
        <ul v-if="selectableFriends.length" class="friend-pick-list">
          <li
            v-for="f in selectableFriends"
            :key="f.id"
            class="friend-pick-item"
            :class="{ selected: isSelected(f.id) }"
            @click="toggleFriend(f.id)"
          >
            <UserAvatar :src="f.avatar" :name="friendDisplayName(f)" :size="32" />
            <span>{{ friendDisplayName(f) }}</span>
            <span class="check">{{ isSelected(f.id) ? '✓' : '' }}</span>
          </li>
        </ul>
        <p v-else class="empty-search">暂无好友可邀请</p>
        <button type="button" class="btn btn-primary btn-block" @click="createGroupWithFriends">创建并发送邀请</button>
      </div>

      <div v-else class="modal-body">
        <section class="face-panel">
          <h4>我是群主</h4>
          <p class="field-hint">输入 4 位数字作为建群码，让好友在「面对面建群」中填入相同数字加入（10 分钟内有效）</p>
          <div class="search-row">
            <input
              v-model="ownerFaceCode"
              class="input face-input"
              type="text"
              maxlength="4"
              inputmode="numeric"
              placeholder="4 位建群码"
            />
            <button
              type="button"
              class="btn btn-primary btn-sm"
              :disabled="faceStarting"
              @click="startFaceAsOwner"
            >
              {{ faceStarting ? '创建中…' : '开始建群' }}
            </button>
          </div>
        </section>
        <div class="face-divider">或</div>
        <section class="face-panel">
          <h4>我是群员</h4>
          <p class="field-hint">输入群主告知的 4 位建群码即可加入</p>
          <div class="search-row">
            <input
              v-model="memberFaceCode"
              class="input face-input"
              type="text"
              maxlength="4"
              inputmode="numeric"
              placeholder="4 位建群码"
            />
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="faceJoining"
              @click="joinFaceAsMember"
            >
              {{ faceJoining ? '加入中…' : '加入群聊' }}
            </button>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 200;
  padding: var(--space-4);
}

.add-modal {
  width: 100%;
  max-width: 460px;
  max-height: 85vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: var(--color-bg-surface);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-4) var(--space-5);
  border-bottom: 1px solid var(--color-border);
}

.modal-header h3 {
  margin: 0;
  font-size: var(--text-lg);
}

.modal-tabs {
  display: flex;
  gap: var(--space-1);
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--color-border);
  overflow-x: auto;
}

.modal-tab {
  flex: 1;
  min-width: 72px;
  min-height: 36px;
  border-radius: var(--radius-sm);
  font-size: var(--text-xs);
  font-weight: 600;
  color: var(--color-text-secondary);
  white-space: nowrap;
}

.modal-tab.active {
  background: var(--color-primary-muted);
  color: var(--color-primary);
}

.modal-body {
  padding: var(--space-4) var(--space-5) var(--space-5);
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.field-hint {
  margin: 0;
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}

.search-row {
  display: flex;
  gap: var(--space-2);
}

.search-row .input {
  flex: 1;
}

.face-input {
  font-size: var(--text-lg);
  letter-spacing: 0.2em;
  text-align: center;
}

.result-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.result-item {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) 0;
  border-bottom: 1px solid var(--color-border);
}

.result-item.clickable {
  cursor: pointer;
}

.result-item.selected {
  background: var(--color-primary-muted);
  margin: 0 calc(-1 * var(--space-3));
  padding-left: var(--space-3);
  padding-right: var(--space-3);
  border-radius: var(--radius-sm);
}

.result-meta {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.result-name {
  font-size: var(--text-sm);
  font-weight: 500;
}

.result-sub {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}

.group-preview {
  padding: var(--space-3);
  background: var(--color-bg-muted);
  border-radius: var(--radius-md);
}

.group-preview h4 {
  margin: 0 0 var(--space-1);
}

.preview-meta {
  margin: 0 0 var(--space-3);
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}

.empty-search {
  margin: 0;
  text-align: center;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.friend-pick-list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 200px;
  overflow-y: auto;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
}

.friend-pick-item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  cursor: pointer;
  font-size: var(--text-sm);
}

.friend-pick-item.selected {
  background: var(--color-primary-muted);
}

.friend-pick-item .check {
  margin-left: auto;
  color: var(--color-primary);
  font-weight: 700;
}

.face-panel h4 {
  margin: 0 0 var(--space-2);
  font-size: var(--text-sm);
}

.face-divider {
  text-align: center;
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}
</style>
