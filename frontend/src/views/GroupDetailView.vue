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
const welcomeText = ref('')
const welcomeDraft = ref('')
const editingWelcome = ref(false)
const savingWelcome = ref(false)
const adminOnly = ref(false)
const togglingAdminOnly = ref(false)
const slowModeSecs = ref(0)
const savingSlowMode = ref(false)
const memberMuted = ref<Record<string, boolean>>({})
const memberNicknames = ref<Record<string, string>>({})
const memberRemarks = ref<Record<string, string>>({})
const remarkTarget = ref<PublicProfile | null>(null)
const remarkDraft = ref('')
const savingRemark = ref(false)
const myNicknameDraft = ref('')
const savingNickname = ref(false)
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
const inviteLinks = ref<
  {
    id: string
    code: string
    max_uses: number
    use_count: number
    expires_at?: string
    expired?: boolean
  }[]
>([])
const creatingLink = ref(false)
const linkMaxUses = ref(0)
const linkExpiresHours = ref(72)
const copiedLinkId = ref('')

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
  const remark = memberRemarks.value[idStr(m.id)]?.trim()
  if (remark) return remark
  const gn = memberNicknames.value[idStr(m.id)]?.trim()
  if (gn) return gn
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
    welcomeText.value = g.welcome_text || ''
    welcomeDraft.value = welcomeText.value
    adminOnly.value = !!g.admin_only
    slowModeSecs.value = g.slow_mode_secs || 0
    members.value = (g.members || []).map((m) => ({ ...m, id: idStr(m.id) }))
    memberRoles.value = g.member_roles || {}
    memberMuted.value = g.member_muted || {}
    memberNicknames.value = g.member_nicknames || {}
    memberRemarks.value = g.member_remarks || {}
    myNicknameDraft.value = memberNicknames.value[idStr(auth.user?.id)] || ''
    await loadPendingInvites()
    if (isManager.value) await loadInviteLinks()
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    loading.value = false
  }
})

async function loadInviteLinks() {
  try {
    inviteLinks.value = await chat.listGroupInviteLinks(groupId())
  } catch {
    inviteLinks.value = []
  }
}

async function createInviteLink() {
  creatingLink.value = true
  try {
    await chat.createGroupInviteLink(groupId(), {
      max_uses: Number(linkMaxUses.value) || 0,
      expires_hours: Number(linkExpiresHours.value) || 0,
    })
    await loadInviteLinks()
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    creatingLink.value = false
  }
}

async function revokeInviteLink(id: string) {
  if (!confirm('确定撤销该邀请链接？')) return
  try {
    await chat.revokeGroupInviteLink(groupId(), id)
    await loadInviteLinks()
  } catch (e) {
    chat.setError(parseError(e))
  }
}

async function copyInviteCode(code: string, id: string) {
  try {
    await navigator.clipboard.writeText(code)
    copiedLinkId.value = id
    chat.setTransientNotice('邀请码已复制')
    setTimeout(() => {
      if (copiedLinkId.value === id) copiedLinkId.value = ''
    }, 2000)
  } catch {
    chat.setError('复制失败')
  }
}

async function saveMyNickname() {
  savingNickname.value = true
  try {
    await chat.setMyGroupNickname(groupId(), myNicknameDraft.value)
    const uid = idStr(auth.user?.id)
    const nick = myNicknameDraft.value.trim()
    if (nick) memberNicknames.value = { ...memberNicknames.value, [uid]: nick }
    else {
      const next = { ...memberNicknames.value }
      delete next[uid]
      memberNicknames.value = next
    }
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    savingNickname.value = false
  }
}

function startRemark(m: PublicProfile) {
  remarkTarget.value = m
  remarkDraft.value = memberRemarks.value[idStr(m.id)] || ''
}

async function saveRemark() {
  if (!remarkTarget.value) return
  savingRemark.value = true
  try {
    const uid = idStr(remarkTarget.value.id)
    await chat.setGroupMemberRemark(groupId(), uid, remarkDraft.value)
    const r = remarkDraft.value.trim()
    if (r) memberRemarks.value = { ...memberRemarks.value, [uid]: r }
    else {
      const next = { ...memberRemarks.value }
      delete next[uid]
      memberRemarks.value = next
    }
    remarkTarget.value = null
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    savingRemark.value = false
  }
}

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

async function saveWelcome() {
  savingWelcome.value = true
  try {
    await chat.setGroupWelcome(groupId(), welcomeDraft.value)
    welcomeText.value = welcomeDraft.value.trim()
    editingWelcome.value = false
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    savingWelcome.value = false
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

function canMute(m: PublicProfile) {
  if (!isManager.value) return false
  if (sameId(m.id, auth.user?.id)) return false
  if (sameId(m.id, ownerId.value)) return false
  if (isAdmin(m) && !isOwner.value) return false
  return true
}

function isMuted(m: PublicProfile) {
  return !!memberMuted.value[idStr(m.id)]
}

async function toggleMute(m: PublicProfile) {
  if (!canMute(m)) return
  try {
    const next = !isMuted(m)
    await chat.setGroupMemberMuted(groupId(), m.id, next)
    const map = { ...memberMuted.value }
    if (next) map[idStr(m.id)] = true
    else delete map[idStr(m.id)]
    memberMuted.value = map
  } catch (e) {
    chat.setError(parseError(e))
  }
}

async function toggleAdminOnly() {
  if (!isManager.value || togglingAdminOnly.value) return
  togglingAdminOnly.value = true
  try {
    const next = !adminOnly.value
    await chat.setGroupAdminOnly(groupId(), next)
    adminOnly.value = next
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    togglingAdminOnly.value = false
  }
}

async function saveSlowMode() {
  if (!isManager.value || savingSlowMode.value) return
  savingSlowMode.value = true
  try {
    const secs = Math.max(0, Math.min(3600, Number(slowModeSecs.value) || 0))
    await chat.setGroupSlowMode(groupId(), secs)
    slowModeSecs.value = secs
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    savingSlowMode.value = false
  }
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

      <section v-if="isManager" class="notice-section">
        <div class="notice-head">
          <h3>入群欢迎语</h3>
          <button
            v-if="!editingWelcome"
            type="button"
            class="btn btn-ghost btn-sm"
            @click="editingWelcome = true; welcomeDraft = welcomeText"
          >
            {{ welcomeText ? '编辑' : '设置' }}
          </button>
        </div>
        <div v-if="editingWelcome" class="notice-edit">
          <textarea
            v-model="welcomeDraft"
            class="notice-input"
            rows="2"
            maxlength="200"
            placeholder="新成员入群后自动发送（最多 200 字）"
          />
          <div class="notice-actions">
            <button type="button" class="btn btn-secondary btn-sm" @click="editingWelcome = false">取消</button>
            <button
              type="button"
              class="btn btn-primary btn-sm"
              :disabled="savingWelcome"
              @click="saveWelcome"
            >
              {{ savingWelcome ? '保存中…' : '保存' }}
            </button>
          </div>
        </div>
        <p v-else class="notice-body">{{ welcomeText || '未设置（仍会提示「xxx 加入了群聊」）' }}</p>
      </section>

      <section class="nickname-section">
        <h3>我的群名片</h3>
        <p class="nickname-hint">仅在本群显示；留空则使用个人昵称。</p>
        <div class="nickname-edit">
          <input
            v-model="myNicknameDraft"
            class="nickname-input"
            type="text"
            maxlength="32"
            placeholder="输入群内昵称"
          />
          <button type="button" class="btn btn-primary btn-sm" :disabled="savingNickname" @click="saveMyNickname">
            {{ savingNickname ? '保存中…' : '保存' }}
          </button>
        </div>
      </section>

      <section v-if="isManager" class="mute-section">
        <h3>发言管理</h3>
        <label class="mute-toggle">
          <input
            type="checkbox"
            :checked="adminOnly"
            :disabled="togglingAdminOnly"
            @change="toggleAdminOnly"
          />
          <span>全员禁言（仅管理员/群主可发言）</span>
        </label>
        <p class="mute-hint">也可在成员列表对个人禁言。</p>
        <div class="slow-mode-row">
          <label class="invite-link-field">
            <span>慢速模式（秒）</span>
            <input v-model.number="slowModeSecs" class="nickname-input" type="number" min="0" max="3600" />
          </label>
          <button type="button" class="btn btn-primary btn-sm" :disabled="savingSlowMode" @click="saveSlowMode">
            {{ savingSlowMode ? '保存中…' : '保存' }}
          </button>
        </div>
        <p class="mute-hint">普通成员两次发言最小间隔；0 为关闭。管理员不受限。</p>
      </section>

      <section v-if="isManager" class="invite-link-section">
        <h3>邀请链接</h3>
        <p class="nickname-hint">分享邀请码，对方可直接入群；可限次、设过期并随时撤销。</p>
        <div class="invite-link-form">
          <label class="invite-link-field">
            <span>次数上限</span>
            <input v-model.number="linkMaxUses" class="nickname-input" type="number" min="0" max="10000" />
          </label>
          <label class="invite-link-field">
            <span>有效小时</span>
            <input v-model.number="linkExpiresHours" class="nickname-input" type="number" min="0" max="2160" />
          </label>
          <button type="button" class="btn btn-primary btn-sm" :disabled="creatingLink" @click="createInviteLink">
            {{ creatingLink ? '创建中…' : '生成链接' }}
          </button>
        </div>
        <p class="mute-hint">次数/小时填 0 表示不限制。</p>
        <ul v-if="inviteLinks.length" class="invite-link-list">
          <li v-for="link in inviteLinks" :key="link.id" class="invite-link-row">
            <div class="invite-link-meta">
              <code class="invite-code">{{ link.code }}</code>
              <span class="member-sub">
                已用 {{ link.use_count }}{{ link.max_uses ? ` / ${link.max_uses}` : '' }}
                <template v-if="link.expires_at"> · 至 {{ link.expires_at.slice(0, 16).replace('T', ' ') }}</template>
                <template v-if="link.expired"> · 已失效</template>
              </span>
            </div>
            <button type="button" class="btn btn-ghost btn-sm" @click="copyInviteCode(link.code, link.id)">
              {{ copiedLinkId === link.id ? '已复制' : '复制' }}
            </button>
            <button type="button" class="btn btn-ghost btn-sm" @click="revokeInviteLink(link.id)">撤销</button>
          </li>
        </ul>
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
            <span v-if="isMuted(m)" class="tag muted">禁言</span>
            <button
              v-if="!sameId(m.id, auth.user?.id)"
              type="button"
              class="btn btn-ghost btn-sm"
              @click.stop="startRemark(m)"
            >
              备注
            </button>
            <button
              v-if="isOwner && !sameId(m.id, ownerId) && !sameId(m.id, auth.user?.id)"
              type="button"
              class="btn btn-ghost btn-sm admin-btn"
              @click.stop="toggleAdmin(m)"
            >
              {{ isAdmin(m) ? '取消管理' : '设管理员' }}
            </button>
            <button
              v-if="canMute(m)"
              type="button"
              class="btn btn-ghost btn-sm mute-btn"
              @click.stop="toggleMute(m)"
            >
              {{ isMuted(m) ? '解禁' : '禁言' }}
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

    <div v-if="remarkTarget" class="remark-overlay" @click.self="remarkTarget = null">
      <div class="remark-dialog" role="dialog" aria-label="设置群成员备注">
        <h3>备注 {{ memberLabel(remarkTarget) }}</h3>
        <p class="nickname-hint">仅你可见，不影响对方在群内的显示名。</p>
        <input v-model="remarkDraft" class="nickname-input" type="text" maxlength="32" placeholder="输入备注" />
        <div class="remark-actions">
          <button type="button" class="btn btn-secondary btn-sm" @click="remarkTarget = null">取消</button>
          <button type="button" class="btn btn-primary btn-sm" :disabled="savingRemark" @click="saveRemark">
            {{ savingRemark ? '保存中…' : '保存' }}
          </button>
        </div>
      </div>
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

.nickname-section {
  margin-top: var(--space-6);
  padding: var(--space-4);
  background: var(--color-bg-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.nickname-section h3 {
  margin: 0 0 var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.nickname-hint {
  margin: 0 0 var(--space-2);
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}

.nickname-edit {
  display: flex;
  gap: var(--space-2);
  align-items: center;
}

.nickname-input {
  flex: 1;
  min-width: 0;
  padding: var(--space-2) var(--space-3);
  font: inherit;
  font-size: var(--text-sm);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-bg-chat);
}

.mute-section {
  margin-top: var(--space-6);
  padding: var(--space-4);
  background: var(--color-bg-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.mute-section h3 {
  margin: 0 0 var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.mute-toggle {
  display: flex;
  align-items: flex-start;
  gap: var(--space-2);
  font-size: var(--text-sm);
  cursor: pointer;
}

.mute-hint {
  margin: var(--space-2) 0 0;
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}

.slow-mode-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: flex-end;
  margin-top: var(--space-3);
}

.tag.muted {
  background: var(--color-danger-muted, #fde8e8);
  color: var(--color-danger, #c62828);
}

.mute-btn {
  flex-shrink: 0;
}

.invite-link-section {
  margin-top: var(--space-6);
  padding: var(--space-4);
  background: var(--color-bg-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.invite-link-section h3 {
  margin: 0 0 var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.invite-link-form {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: flex-end;
}

.invite-link-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  min-width: 96px;
}

.invite-link-list {
  list-style: none;
  margin: var(--space-3) 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.invite-link-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.invite-link-meta {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.invite-code {
  font-size: var(--text-sm);
  letter-spacing: 0.04em;
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

.remark-overlay {
  position: fixed;
  inset: 0;
  z-index: 40;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.35);
  padding: var(--space-4);
}

.remark-dialog {
  width: min(360px, 100%);
  padding: var(--space-4);
  background: var(--color-bg-surface);
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
}

.remark-dialog h3 {
  margin: 0 0 var(--space-2);
  font-size: var(--text-base);
}

.remark-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  margin-top: var(--space-3);
}
</style>
