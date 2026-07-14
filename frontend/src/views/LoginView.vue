<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { parseError } from '../api/errors'
import { useAuthStore } from '../stores/auth'
import { isDesktopApp } from '../utils/desktop'

const REMEMBER_KEY = 'squirtlechat_remember_username'

const username = ref(localStorage.getItem(REMEMBER_KEY) || '')
const password = ref('')
const nickname = ref('')
const mode = ref<'login' | 'register'>('login')
const rememberUsername = ref(!!localStorage.getItem(REMEMBER_KEY))
const err = ref('')
const loading = ref(false)
const auth = useAuthStore()
const router = useRouter()
const desktop = isDesktopApp()

onMounted(() => {
  if (desktop && window.squirtleDesktop?.setShellMode) {
    void window.squirtleDesktop.setShellMode('login')
  }
  if (desktop) {
    document.documentElement.classList.add('desktop-login')
    document.body.classList.add('desktop-login')
  }
})

onUnmounted(() => {
  document.documentElement.classList.remove('desktop-login')
  document.body.classList.remove('desktop-login')
})

function setMode(next: 'login' | 'register') {
  mode.value = next
  err.value = ''
}

async function submit() {
  err.value = ''
  loading.value = true
  try {
    if (mode.value === 'login') {
      await auth.login(username.value, password.value)
    } else {
      await auth.register(username.value, password.value, nickname.value || username.value)
    }
    if (rememberUsername.value) {
      localStorage.setItem(REMEMBER_KEY, username.value.trim())
    } else {
      localStorage.removeItem(REMEMBER_KEY)
    }
    if (desktop && window.squirtleDesktop?.setShellMode) {
      await window.squirtleDesktop.setShellMode('main')
    }
    router.push('/')
  } catch (e: unknown) {
    err.value = parseError(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="auth-page" :class="{ compact: desktop }">
    <div v-if="!desktop" class="auth-bg" aria-hidden="true" />
    <div class="auth-container" :class="{ compact: desktop }">
      <section v-if="!desktop" class="auth-brand">
        <div class="logo" aria-hidden="true">
          <svg viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
            <circle cx="24" cy="24" r="22" fill="currentColor" opacity="0.15" />
            <path
              d="M24 8c8.8 0 16 6.5 16 14.5 0 5.2-3 9.8-7.5 12.3L30 38h-12l-2.5-3.2C11 32.3 8 27.7 8 22.5 8 14.5 15.2 8 24 8z"
              fill="currentColor"
            />
            <circle cx="17" cy="22" r="2.5" fill="#0f766e" />
            <circle cx="31" cy="22" r="2.5" fill="#0f766e" />
          </svg>
        </div>
        <h1 class="brand-title">SquirtleChat</h1>
        <p class="brand-desc">安全、流畅的企业级即时通讯</p>
        <ul class="brand-features">
          <li>端到端实时消息</li>
          <li>好友与群组协作</li>
          <li>多端同步</li>
        </ul>
      </section>

      <section class="auth-card">
        <div v-if="desktop" class="compact-brand">
          <div class="logo mini" aria-hidden="true">
            <svg viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
              <circle cx="24" cy="24" r="22" fill="#0d9488" opacity="0.15" />
              <path
                d="M24 8c8.8 0 16 6.5 16 14.5 0 5.2-3 9.8-7.5 12.3L30 38h-12l-2.5-3.2C11 32.3 8 27.7 8 22.5 8 14.5 15.2 8 24 8z"
                fill="#0d9488"
              />
              <circle cx="17" cy="22" r="2.5" fill="#0f766e" />
              <circle cx="31" cy="22" r="2.5" fill="#0f766e" />
            </svg>
          </div>
          <strong>SquirtleChat</strong>
        </div>

        <h2 class="card-title">{{ mode === 'login' ? '欢迎回来' : '创建账号' }}</h2>
        <p class="card-sub">{{ mode === 'login' ? '登录以继续聊天' : '填写信息完成注册' }}</p>

        <div class="tabs" role="tablist">
          <button
            type="button"
            role="tab"
            :aria-selected="mode === 'login'"
            class="tab"
            :class="{ active: mode === 'login' }"
            @click="setMode('login')"
          >
            登录
          </button>
          <button
            type="button"
            role="tab"
            :aria-selected="mode === 'register'"
            class="tab"
            :class="{ active: mode === 'register' }"
            @click="setMode('register')"
          >
            注册
          </button>
        </div>

        <form @submit.prevent="submit">
          <div class="field">
            <label class="field-label" for="username">用户名</label>
            <input
              id="username"
              v-model="username"
              class="input"
              type="text"
              autocomplete="username"
              placeholder="请输入用户名"
              required
            />
          </div>
          <div class="field">
            <label class="field-label" for="password">密码</label>
            <input
              id="password"
              v-model="password"
              class="input"
              type="password"
              autocomplete="current-password"
              placeholder="请输入密码"
              required
            />
          </div>
          <Transition name="field-slide">
            <div v-if="mode === 'register'" key="nickname" class="field">
              <label class="field-label" for="nickname">昵称</label>
              <input
                id="nickname"
                v-model="nickname"
                class="input"
                type="text"
                autocomplete="nickname"
                placeholder="显示名称（可选）"
              />
            </div>
          </Transition>
          <Transition name="field-slide">
            <label v-if="mode === 'login'" key="remember" class="remember-row">
              <input v-model="rememberUsername" type="checkbox" />
              <span>记住用户名</span>
            </label>
          </Transition>

          <Transition name="field-slide">
            <div v-if="err" key="err" class="alert alert-error" role="alert">{{ err }}</div>
          </Transition>

          <button type="submit" class="btn btn-primary btn-block auth-submit" :disabled="loading">
            <span v-if="loading" class="auth-spinner" aria-hidden="true" />
            {{ loading ? '处理中…' : mode === 'login' ? '登录' : '注册' }}
          </button>
        </form>
      </section>
    </div>
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-6);
  position: relative;
  overflow: hidden;
}

.auth-page.compact {
  padding: 0;
  background: #fff;
  align-items: stretch;
  height: 100%;
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}

.auth-bg {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(ellipse 80% 60% at 20% 20%, rgba(13, 148, 136, 0.18), transparent),
    radial-gradient(ellipse 60% 50% at 80% 80%, rgba(15, 118, 110, 0.12), transparent),
    linear-gradient(160deg, #f0fdfa 0%, #eef2f6 45%, #e0f2fe 100%);
  z-index: 0;
}

.auth-container {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: 1fr 1fr;
  max-width: 920px;
  width: 100%;
  background: var(--color-bg-elevated);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-lg);
  border: 1px solid var(--color-border);
  overflow: hidden;
}

.auth-container.compact {
  display: block;
  max-width: none;
  width: 100%;
  min-height: 100%;
  height: auto;
  border: none;
  border-radius: 0;
  box-shadow: none;
  background: #fff;
  overflow: visible;
}

.auth-brand {
  padding: var(--space-8);
  background: linear-gradient(145deg, #0f766e 0%, #0d9488 50%, #14b8a6 100%);
  color: var(--color-text-inverse);
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.logo {
  width: 56px;
  height: 56px;
  color: #fff;
  margin-bottom: var(--space-4);
}

.logo.mini {
  width: 40px;
  height: 40px;
  margin: 0;
  color: #0d9488;
}

.compact-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 20px;
}

.compact-brand strong {
  font-size: 16px;
  color: #0f766e;
}

.brand-title {
  margin: 0 0 var(--space-2);
  font-size: var(--text-2xl);
  font-weight: 700;
  letter-spacing: -0.02em;
}

.brand-desc {
  margin: 0 0 var(--space-6);
  opacity: 0.9;
  font-size: var(--text-base);
}

.brand-features {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  font-size: var(--text-sm);
  opacity: 0.95;
}

.brand-features li {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.brand-features li::before {
  content: '';
  width: 6px;
  height: 6px;
  border-radius: var(--radius-full);
  background: #99f6e4;
  flex-shrink: 0;
}

.auth-card {
  padding: var(--space-8);
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.auth-container.compact .auth-card {
  padding: 24px 28px 28px;
  height: auto;
  min-height: 100%;
  box-sizing: border-box;
  justify-content: flex-start;
}

.card-title {
  margin: 0 0 var(--space-1);
  font-size: var(--text-xl);
  font-weight: 700;
  color: var(--color-text-primary);
}

.card-sub {
  margin: 0 0 var(--space-6);
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
}

.tabs {
  display: flex;
  gap: var(--space-2);
  padding: var(--space-1);
  background: var(--color-bg-sidebar);
  border-radius: var(--radius-md);
  margin-bottom: var(--space-6);
}

.tab {
  flex: 1;
  min-height: 40px;
  border-radius: var(--radius-sm);
  font-weight: 600;
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  transition: all var(--transition-fast);
}

.tab:hover {
  color: var(--color-text-primary);
}

.tab.active {
  background: var(--color-bg-surface);
  color: var(--color-primary);
  box-shadow: var(--shadow-sm);
}

.tab:focus-visible {
  outline: none;
  box-shadow: var(--shadow-focus);
}

form .alert {
  margin-bottom: var(--space-4);
}

.remember-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin: 0 0 var(--space-4);
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  cursor: pointer;
  user-select: none;
}

.remember-row input {
  width: 16px;
  height: 16px;
  accent-color: var(--color-primary);
}

.auth-submit {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.auth-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, 0.35);
  border-top-color: #fff;
  border-radius: 50%;
  animation: auth-spin 0.7s linear infinite;
}

@keyframes auth-spin {
  to {
    transform: rotate(360deg);
  }
}

.field-slide-enter-active,
.field-slide-leave-active {
  transition:
    opacity var(--transition-base, 200ms ease),
    transform var(--transition-base, 200ms ease),
    max-height var(--transition-base, 200ms ease);
  overflow: hidden;
}
.field-slide-enter-from,
.field-slide-leave-to {
  opacity: 0;
  max-height: 0;
  transform: translateY(-4px);
}
.field-slide-enter-to,
.field-slide-leave-from {
  max-height: 96px;
}

@media (max-width: 768px) {
  .auth-container:not(.compact) {
    grid-template-columns: 1fr;
    max-width: 420px;
  }

  .auth-brand {
    padding: var(--space-6);
    text-align: center;
    align-items: center;
  }

  .brand-features {
    display: none;
  }

  .auth-card {
    padding: var(--space-6);
  }
}
</style>

<style>
html.desktop-login,
body.desktop-login,
body.desktop-login #app {
  margin: 0;
  height: 100%;
  overflow: hidden;
  background: #fff !important;
}
</style>
