import { defineStore } from 'pinia'

export type LayoutMode = 'embedded' | 'detached'

const LAYOUT_KEY = 'squirtlechat_layout_mode'
const AOT_KEY = 'squirtlechat_chat_always_on_top'

function loadLayout(): LayoutMode {
  const v = localStorage.getItem(LAYOUT_KEY)
  return v === 'detached' ? 'detached' : 'embedded'
}

export const useSettingsStore = defineStore('settings', {
  state: () => ({
    layoutMode: loadLayout() as LayoutMode,
    chatAlwaysOnTop: localStorage.getItem(AOT_KEY) === '1',
  }),
  actions: {
    setLayoutMode(mode: LayoutMode) {
      this.layoutMode = mode
      localStorage.setItem(LAYOUT_KEY, mode)
    },
    setChatAlwaysOnTop(on: boolean) {
      this.chatAlwaysOnTop = on
      localStorage.setItem(AOT_KEY, on ? '1' : '0')
      const api = window.squirtleDesktop
      if (api?.setAlwaysOnTop) void api.setAlwaysOnTop(on)
    },
  },
})
