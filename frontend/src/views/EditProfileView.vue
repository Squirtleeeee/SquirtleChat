<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { parseError } from '../api/errors'
import AvatarCropper from '../components/AvatarCropper.vue'
import UserAvatar from '../components/UserAvatar.vue'
import { useAuthStore, type UserPrivacy } from '../stores/auth'
import { useChatStore } from '../stores/chat'
import { mediaUrl } from '../utils/media'

const router = useRouter()
const auth = useAuthStore()
const chat = useChatStore()

const nickname = ref('')
const statusText = ref('')
const statusEmoji = ref('')
const gender = ref(0)
const birthday = ref('')
const privacy = ref<UserPrivacy>({
  show_nickname: true,
  show_gender: false,
  show_birthday: false,
  show_avatar: true,
})
const showPrivacy = ref(false)
const cropFile = ref<File | null>(null)
const avatarInput = ref<HTMLInputElement | null>(null)
const saving = ref(false)

const STATUS_PRESETS = [
  { emoji: '💼', text: '工作中' },
  { emoji: '🏠', text: '远程办公' },
  { emoji: '☕', text: '稍后再回' },
  { emoji: '✈️', text: '出差中' },
  { emoji: '', text: '' },
]

onMounted(async () => {
  if (!auth.user) await auth.restoreSession()
  const u = auth.user
  if (!u) {
    router.replace('/login')
    return
  }
  nickname.value = u.nickname || ''
  statusText.value = u.status_text || ''
  statusEmoji.value = u.status_emoji || ''
  gender.value = u.gender || 0
  birthday.value = u.birthday || ''
  if (u.privacy) privacy.value = { ...u.privacy }
})

function onPickAvatar() {
  avatarInput.value?.click()
}

function onAvatarFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (file && file.type.startsWith('image/')) cropFile.value = file
}

async function onCropConfirm(blob: Blob) {
  cropFile.value = null
  try {
    await auth.uploadAvatar(blob)
    chat.setNotice('头像已更新')
  } catch (e) {
    chat.setError(parseError(e))
  }
}

async function saveProfile() {
  saving.value = true
  try {
    await auth.updateProfile({
      nickname: nickname.value.trim(),
      status_text: statusText.value.trim(),
      status_emoji: statusEmoji.value.trim(),
      gender: gender.value,
      birthday: birthday.value,
    })
    await auth.updatePrivacy(privacy.value)
    chat.setNotice('资料已保存')
    router.push('/profile')
  } catch (e) {
    chat.setError(parseError(e))
  } finally {
    saving.value = false
  }
}

function goBack() {
  router.push('/profile')
}

function displayName() {
  return auth.user?.nickname || auth.user?.username || ''
}

function avatarUrl() {
  return mediaUrl(auth.user?.avatar)
}
</script>

<template>
  <div class="edit-page">
    <header class="edit-header">
      <button type="button" class="btn btn-ghost btn-sm" @click="goBack">取消</button>
      <h1>编辑资料</h1>
      <button type="button" class="btn btn-primary btn-sm save-btn" :disabled="saving" @click="saveProfile">
        <span v-if="saving" class="save-spinner" aria-hidden="true" />
        {{ saving ? '保存中' : '保存' }}
      </button>
    </header>

    <div class="edit-body">
      <section class="edit-section">
        <button type="button" class="avatar-edit" @click="onPickAvatar">
          <UserAvatar :src="avatarUrl()" :name="displayName()" :size="72" />
          <span>点击更换头像</span>
        </button>
        <input ref="avatarInput" type="file" accept="image/*" class="hidden" @change="onAvatarFile" />
      </section>

      <section class="edit-section">
        <label class="field-label" for="nick">名字</label>
        <input id="nick" v-model="nickname" class="input" type="text" maxlength="32" />
      </section>

      <section class="edit-section">
        <label class="field-label" for="status">状态</label>
        <div class="status-row">
          <input
            id="status-emoji"
            v-model="statusEmoji"
            class="input status-emoji"
            type="text"
            maxlength="8"
            placeholder="🙂"
            aria-label="状态表情"
          />
          <input
            id="status"
            v-model="statusText"
            class="input"
            type="text"
            maxlength="64"
            placeholder="如：工作中 / 稍后再回"
          />
        </div>
        <div class="status-presets">
          <button
            v-for="(p, i) in STATUS_PRESETS"
            :key="i"
            type="button"
            class="preset-chip"
            @click="statusEmoji = p.emoji; statusText = p.text"
          >
            {{ p.emoji || '清空' }}{{ p.text ? ` ${p.text}` : '' }}
          </button>
        </div>
      </section>

      <section class="edit-section">
        <label class="field-label" for="gender">性别</label>
        <select id="gender" v-model.number="gender" class="input">
          <option :value="0">未设置</option>
          <option :value="1">男</option>
          <option :value="2">女</option>
        </select>
      </section>

      <section class="edit-section">
        <label class="field-label" for="bday">生日</label>
        <input id="bday" v-model="birthday" class="input" type="date" />
      </section>

      <section class="edit-section">
        <button type="button" class="privacy-toggle" @click="showPrivacy = !showPrivacy">
          <span>隐私设置</span>
          <span class="privacy-chevron" :class="{ open: showPrivacy }">›</span>
        </button>
        <Transition name="privacy">
          <div v-if="showPrivacy" class="privacy-panel">
            <label class="check-row">
              <input v-model="privacy.show_nickname" type="checkbox" />
              允许他人查看昵称
            </label>
            <label class="check-row">
              <input v-model="privacy.show_avatar" type="checkbox" />
              允许他人查看头像
            </label>
            <label class="check-row">
              <input v-model="privacy.show_gender" type="checkbox" />
              允许他人查看性别
            </label>
            <label class="check-row">
              <input v-model="privacy.show_birthday" type="checkbox" />
              允许他人查看生日
            </label>
            <p class="privacy-note">关闭后，其他用户只能看到您的用户名</p>
          </div>
        </Transition>
      </section>
    </div>

    <Transition name="fade">
      <div v-if="cropFile" class="modal-backdrop">
        <div class="modal-card crop-modal">
          <h3>裁剪头像</h3>
          <AvatarCropper :file="cropFile" @confirm="onCropConfirm" />
          <button type="button" class="btn btn-secondary btn-block" @click="cropFile = null">取消</button>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.edit-page {
  min-height: 100vh;
  background: var(--color-bg-app);
}

.edit-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: var(--header-height);
  padding: 0 var(--space-4);
  background: var(--color-bg-surface);
  border-bottom: 1px solid var(--color-border);
}

.edit-header h1 {
  margin: 0;
  font-size: var(--text-base);
  font-weight: 600;
}

.edit-body {
  max-width: 480px;
  margin: 0 auto;
  padding: var(--space-4);
}

.edit-section {
  margin-bottom: var(--space-5);
  padding-bottom: var(--space-4);
  border-bottom: 1px solid var(--color-border);
}

.avatar-edit {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-2);
  width: 100%;
  padding: var(--space-3);
  color: var(--color-primary);
  font-size: var(--text-sm);
}

.status-row {
  display: flex;
  gap: var(--space-2);
}

.status-emoji {
  width: 56px;
  flex-shrink: 0;
  text-align: center;
}

.status-presets {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: var(--space-2);
}

.preset-chip {
  border: 1px solid var(--color-border);
  background: var(--color-bg-surface);
  border-radius: var(--radius-sm);
  padding: 4px 8px;
  font-size: var(--text-xs);
  cursor: pointer;
}

.preset-chip:hover {
  border-color: var(--color-primary);
  color: var(--color-primary);
}

.hidden {
  display: none;
}

.privacy-toggle {
  display: flex;
  width: 100%;
  justify-content: space-between;
  align-items: center;
  padding: var(--space-2) 0;
  font-weight: 600;
  font-size: var(--text-sm);
}

.privacy-chevron {
  display: inline-block;
  font-size: 1.25rem;
  line-height: 1;
  transform: rotate(90deg);
  transition: transform var(--transition-fast, 150ms ease);
  color: var(--color-text-muted);
}

.privacy-chevron.open {
  transform: rotate(-90deg);
}

.save-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 64px;
  justify-content: center;
}

.save-spinner {
  width: 12px;
  height: 12px;
  border: 2px solid rgba(255, 255, 255, 0.35);
  border-top-color: #fff;
  border-radius: 50%;
  animation: save-spin 0.7s linear infinite;
}

@keyframes save-spin {
  to { transform: rotate(360deg); }
}

.privacy-enter-active,
.privacy-leave-active {
  transition:
    opacity var(--transition-base, 200ms ease),
    max-height var(--transition-base, 200ms ease),
    transform var(--transition-base, 200ms ease);
  overflow: hidden;
}
.privacy-enter-from,
.privacy-leave-to {
  opacity: 0;
  max-height: 0;
  transform: translateY(-4px);
}
.privacy-enter-to,
.privacy-leave-from {
  max-height: 240px;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--transition-base, 200ms ease);
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 300;
  padding: var(--space-4);
}

.crop-modal {
  width: 100%;
  max-width: 360px;
  padding: var(--space-5);
  background: var(--color-bg-surface);
  border-radius: var(--radius-lg);
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  animation: crop-in var(--transition-base, 200ms ease);
}

@keyframes crop-in {
  from { opacity: 0; transform: translateY(8px) scale(0.98); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}

.privacy-panel {
  margin-top: var(--space-3);
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.check-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-sm);
}

.privacy-note {
  margin: var(--space-2) 0 0;
  font-size: var(--text-xs);
  color: var(--color-text-muted);
}

.crop-modal h3 {
  margin: 0;
  text-align: center;
}
</style>
