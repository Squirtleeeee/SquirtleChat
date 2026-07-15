<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { parseError } from '../api/errors'
import UserAvatar from '../components/UserAvatar.vue'
import { useAuthStore, type PublicProfile } from '../stores/auth'
import { useChatStore } from '../stores/chat'
import { sameId } from '../utils/id'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const chat = useChatStore()

const profile = ref<PublicProfile | null>(null)
const loading = ref(true)
const friendRemark = ref('')
const savingRemark = ref(false)

const userId = computed(() => String(route.params.id || auth.user?.id || ''))
const isSelf = computed(() => sameId(userId.value, auth.user?.id))
const isFriend = computed(() => !isSelf.value && chat.isFriend(userId.value))

const genderLabel = (g?: number) => {
  if (g === 1) return '男'
  if (g === 2) return '女'
  return ''
}

onMounted(async () => {
  if (!auth.user) await auth.restoreSession()
  if (!chat.friends.length) await chat.loadFriends().catch(() => {})
  try {
    if (isSelf.value && auth.user) {
      profile.value = auth.toPublicProfile(auth.user)
    } else {
      profile.value = await auth.fetchPublicProfile(userId.value)
      const f = chat.friends.find((x) => sameId(x.id, userId.value))
      friendRemark.value = (f as { remark?: string })?.remark || ''
    }
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    loading.value = false
  }
})

function goEdit() {
  router.push('/profile/edit')
}

function goBack() {
  router.push('/')
}

function displayName() {
  if (!profile.value) return ''
  return profile.value.nickname || profile.value.username
}

async function sendMessage() {
  const friend = chat.friends.find((f) => sameId(f.id, userId.value))
  if (!friend) return
  await chat.openDirect(friend)
  router.push('/')
}

async function saveRemark() {
  savingRemark.value = true
  try {
    await chat.setFriendRemark(userId.value, friendRemark.value.trim())
    chat.setNotice('备注已保存')
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    savingRemark.value = false
  }
}
</script>

<template>
  <div class="profile-page">
    <header class="profile-header">
      <button type="button" class="btn btn-ghost btn-sm" @click="goBack">← 返回</button>
      <h1>详细资料</h1>
      <button v-if="isSelf" type="button" class="btn btn-ghost btn-sm" @click="goEdit">编辑</button>
      <span v-else />
    </header>

    <div v-if="loading" class="profile-loading" aria-busy="true">
      <div class="profile-skel-avatar" />
      <div class="profile-skel-line w50" />
      <div class="profile-skel-line w30" />
      <div class="profile-skel-card">
        <div class="profile-skel-line w40" />
        <div class="profile-skel-line w70" />
        <div class="profile-skel-line w55" />
      </div>
    </div>
    <Transition name="fade" mode="out-in">
      <div v-if="!loading && profile" key="body" class="profile-body">
      <div class="profile-hero">
        <UserAvatar :src="profile.avatar" :name="displayName()" :size="80" />
        <h2>{{ displayName() }}</h2>
        <p class="username">@{{ profile.username }}</p>
        <p v-if="profile.status_text || profile.status_emoji" class="status-line">
          <span v-if="profile.status_emoji">{{ profile.status_emoji }}</span>
          {{ profile.status_text }}
        </p>
      </div>

      <dl class="profile-fields">
        <template v-if="profile.status_text || profile.status_emoji">
          <dt>状态</dt>
          <dd>{{ profile.status_emoji }} {{ profile.status_text }}</dd>
        </template>
        <template v-if="profile.nickname">
          <dt>昵称</dt>
          <dd>{{ profile.nickname }}</dd>
        </template>
        <template v-if="profile.gender">
          <dt>性别</dt>
          <dd>{{ genderLabel(profile.gender) }}</dd>
        </template>
        <template v-if="profile.birthday">
          <dt>生日</dt>
          <dd>{{ profile.birthday }}</dd>
        </template>
      </dl>

      <button v-if="isSelf" type="button" class="btn btn-primary btn-block" @click="goEdit">编辑个人资料</button>
      <template v-else-if="isFriend">
        <section class="remark-box">
          <label class="field-label" for="friend-remark">备注名</label>
          <input id="friend-remark" v-model="friendRemark" class="input" type="text" maxlength="64" placeholder="仅自己可见的备注" />
          <button type="button" class="btn btn-secondary btn-block" :disabled="savingRemark" @click="saveRemark">保存备注</button>
        </section>
        <button type="button" class="btn btn-primary btn-block" @click="sendMessage">发消息</button>
      </template>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.profile-page {
  min-height: 100vh;
  background: var(--color-bg-app);
}

.profile-header {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  min-height: var(--header-height);
  padding: 0 var(--space-4);
  background: var(--color-bg-surface);
  border-bottom: 1px solid var(--color-border);
}

.profile-header h1 {
  margin: 0;
  font-size: var(--text-base);
  font-weight: 600;
  text-align: center;
}

.profile-loading {
  max-width: 480px;
  margin: 0 auto;
  padding: var(--space-8) var(--space-4);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-3);
}

.profile-skel-avatar {
  width: 80px;
  height: 80px;
  border-radius: 50%;
  background: linear-gradient(90deg, #e2e8f0 25%, #f1f5f9 50%, #e2e8f0 75%);
  background-size: 200% 100%;
  animation: profile-shimmer 1.2s ease-in-out infinite;
}

.profile-skel-line {
  height: 12px;
  border-radius: var(--radius-sm);
  background: linear-gradient(90deg, #e2e8f0 25%, #f1f5f9 50%, #e2e8f0 75%);
  background-size: 200% 100%;
  animation: profile-shimmer 1.2s ease-in-out infinite;
}

.profile-skel-line.w30 { width: 30%; }
.profile-skel-line.w40 { width: 40%; }
.profile-skel-line.w50 { width: 50%; }
.profile-skel-line.w55 { width: 55%; }
.profile-skel-line.w70 { width: 70%; }

.profile-skel-card {
  width: 100%;
  margin-top: var(--space-4);
  padding: var(--space-4);
  border-radius: var(--radius-md);
  background: var(--color-bg-surface);
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

@keyframes profile-shimmer {
  0% { background-position: 100% 0; }
  100% { background-position: -100% 0; }
}

.profile-body {
  max-width: 480px;
  margin: 0 auto;
  padding: var(--space-6) var(--space-4);
  animation: profile-body-in var(--transition-base, 200ms ease);
}

@keyframes profile-body-in {
  from { opacity: 0; transform: translateY(6px); }
  to { opacity: 1; transform: translateY(0); }
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--transition-base, 200ms ease);
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.profile-hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-2);
  margin-bottom: var(--space-6);
}

.profile-hero h2 {
  margin: var(--space-2) 0 0;
  font-size: var(--text-xl);
}

.username {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.status-line {
  margin: 6px 0 0;
  font-size: var(--text-sm);
  color: var(--color-text);
}

.profile-fields {
  margin: 0 0 var(--space-6);
  padding: var(--space-4);
  background: var(--color-bg-surface);
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
}

.profile-fields dt {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  margin-top: var(--space-3);
}

.profile-fields dt:first-child {
  margin-top: 0;
}

.profile-fields dd {
  margin: var(--space-1) 0 0;
  font-size: var(--text-sm);
}

.remark-box {
  margin-bottom: var(--space-4);
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}
</style>
