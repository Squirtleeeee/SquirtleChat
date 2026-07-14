<script setup lang="ts">
import { computed } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import { useChatStore } from './stores/chat'
import { isDesktopApp } from './utils/desktop'
import DesktopWindowChrome from './components/DesktopWindowChrome.vue'

const chat = useChatStore()
const route = useRoute()
const desktop = isDesktopApp()
const showChrome = computed(() => desktop && route.name !== 'popup-chat')
const allowMaximize = computed(() => route.name !== 'login')
</script>

<template>
  <div class="app-frame" :class="{ desktop: showChrome }">
    <DesktopWindowChrome v-if="showChrome" :allow-maximize="allowMaximize" />
    <div class="app-body">
      <RouterView v-slot="{ Component, route: r }">
        <KeepAlive include="ChatView">
          <Transition v-if="r.name !== 'popup-chat'" name="page" mode="out-in">
            <component
              :is="Component"
              :key="r.name === 'chat' ? 'chat-shell' : String(r.fullPath)"
            />
          </Transition>
          <component v-else :is="Component" :key="String(r.fullPath)" />
        </KeepAlive>
      </RouterView>
    </div>
  </div>

  <Teleport to="body">
    <Transition name="toast">
      <div v-if="chat.notice" class="app-toast-wrap" role="status">
        <div class="app-toast">{{ chat.notice }}</div>
      </div>
    </Transition>
  </Teleport>
</template>

<style>
.app-frame {
  height: 100%;
  min-height: 100%;
}

.app-frame.desktop {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.app-frame.desktop .app-body {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  position: relative;
}

.app-body {
  height: 100%;
}

.page-enter-active,
.page-leave-active {
  transition: opacity var(--transition-base, 200ms ease), transform var(--transition-base, 200ms ease);
}

.page-enter-from {
  opacity: 0;
  transform: translateY(6px);
}

.page-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

.app-toast-wrap {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 88px;
  z-index: 1300;
  display: flex;
  justify-content: center;
  pointer-events: none;
  padding: 0 16px;
}

.app-toast {
  max-width: min(90vw, 360px);
  padding: 10px 16px;
  font-size: 14px;
  color: #fff;
  background: rgba(15, 23, 42, 0.88);
  border-radius: 999px;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.2);
}

.toast-enter-active,
.toast-leave-active {
  transition: opacity 200ms ease, transform 200ms ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(10px);
}
</style>
