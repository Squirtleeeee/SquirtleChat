<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useSettingsStore, type LayoutMode } from '../stores/settings'
import { isDesktopApp } from '../utils/desktop'
import { useChatStore } from '../stores/chat'

const router = useRouter()
const settings = useSettingsStore()
const chat = useChatStore()
const desktop = isDesktopApp()

function goBack() {
  router.push('/')
}

function onLayoutChange(e: Event) {
  const v = (e.target as HTMLSelectElement).value as LayoutMode
  settings.setLayoutMode(v)
  chat.setTransientNotice(
    v === 'detached' ? '已切换为会话独立窗口模式' : '已切换为经典单窗口模式',
  )
}

function onAlwaysOnTop(e: Event) {
  const on = (e.target as HTMLInputElement).checked
  settings.setChatAlwaysOnTop(on)
}
</script>

<template>
  <div class="settings-page">
    <header class="page-header">
      <button type="button" class="btn btn-ghost btn-sm" @click="goBack">← 返回</button>
      <h1>设置</h1>
      <span />
    </header>

    <div class="body">
      <section class="block">
        <h2>排版模式</h2>
        <p class="hint">参考微信电脑版：可在单窗口内切换会话，或每个会话单独弹出窗口。</p>
        <label class="field">
          <span class="label">会话布局</span>
          <select class="input" :value="settings.layoutMode" @change="onLayoutChange">
            <option value="embedded">经典单窗口（侧栏 + 聊天区）</option>
            <option value="detached">会话独立窗口（点击会话弹出小窗）</option>
          </select>
        </label>
        <ul class="tips">
          <li>经典模式：与现在一致，所有聊天在同一窗口右侧进行。</li>
          <li>独立窗口：点击好友/群聊时打开独立聊天窗，适合多会话并行。</li>
          <li>任意模式下均可在会话列表右键选择「独立窗口打开」。</li>
        </ul>
      </section>

      <section v-if="desktop" class="block">
        <h2>桌面端</h2>
        <label class="check-row">
          <input type="checkbox" :checked="settings.chatAlwaysOnTop" @change="onAlwaysOnTop" />
          <span>聊天独立窗口默认置顶</span>
        </label>
        <p class="hint">关闭主窗口时会最小化到系统托盘，可在托盘图标右键退出。</p>
      </section>

      <section class="block">
        <h2>关于</h2>
        <p class="meta">SquirtleChat {{ desktop ? '桌面版' : '网页版' }}</p>
      </section>
    </div>
  </div>
</template>

<style scoped>
.settings-page {
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

.body {
  max-width: 560px;
  margin: 0 auto;
  padding: var(--space-5) var(--space-4);
  display: flex;
  flex-direction: column;
  gap: var(--space-5);
}

.block h2 {
  margin: 0 0 var(--space-2);
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.hint {
  margin: 0 0 var(--space-3);
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  line-height: 1.5;
}

.field {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.label {
  font-size: var(--text-sm);
  font-weight: 600;
}

.tips {
  margin: var(--space-3) 0 0;
  padding-left: 1.2em;
  font-size: var(--text-xs);
  color: var(--color-text-secondary);
  line-height: 1.6;
}

.check-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--text-sm);
}

.meta {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
}
</style>
