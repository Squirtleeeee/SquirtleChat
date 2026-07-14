<script setup lang="ts">
import { onMounted, ref } from 'vue'

withDefaults(
  defineProps<{
    allowMaximize?: boolean
  }>(),
  { allowMaximize: true },
)

const maximized = ref(false)

onMounted(async () => {
  if (window.squirtleDesktop?.isMaximized) {
    maximized.value = await window.squirtleDesktop.isMaximized()
  }
})

async function minimize() {
  await window.squirtleDesktop?.windowMinimize()
}

async function maximize() {
  const res = await window.squirtleDesktop?.windowMaximize()
  if (res && typeof res.maximized === 'boolean') maximized.value = res.maximized
  else if (window.squirtleDesktop?.isMaximized) maximized.value = await window.squirtleDesktop.isMaximized()
}

async function closeWin() {
  await window.squirtleDesktop?.windowClose()
}
</script>

<template>
  <div class="desk-chrome" aria-hidden="false">
    <div class="desk-drag" title="拖动窗口" />
    <div class="desk-controls">
      <button type="button" class="win-btn" title="最小化" @click="minimize">─</button>
      <button
        v-if="allowMaximize"
        type="button"
        class="win-btn"
        :title="maximized ? '还原' : '最大化'"
        @click="maximize"
      >
        {{ maximized ? '❐' : '□' }}
      </button>
      <button type="button" class="win-btn close" title="关闭" @click="closeWin">×</button>
    </div>
  </div>
</template>

<style scoped>
.desk-chrome {
  flex-shrink: 0;
  height: 36px;
  display: flex;
  align-items: stretch;
  background: transparent;
  user-select: none;
  z-index: 50;
}

.desk-drag {
  flex: 1;
  min-width: 0;
  -webkit-app-region: drag;
  app-region: drag;
}

.desk-controls {
  display: flex;
  -webkit-app-region: no-drag;
  app-region: no-drag;
}

.win-btn {
  width: 46px;
  border: none;
  background: transparent;
  font-size: 14px;
  line-height: 36px;
  color: #334155;
  cursor: pointer;
  padding: 0;
}

.win-btn:hover {
  background: rgba(15, 23, 42, 0.06);
}

.win-btn.close:hover {
  background: #e81123;
  color: #fff;
}
</style>
