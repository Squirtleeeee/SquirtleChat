import { defineStore } from 'pinia'

export type LayoutMode = 'embedded' | 'detached'

const LAYOUT_KEY = 'squirtlechat_layout_mode'
const AOT_KEY = 'squirtlechat_chat_always_on_top'
const NOTIFY_KEY = 'squirtlechat_notify_prefs'

export type NotifyPrefs = {
  desktopEnabled: boolean
  quietHoursEnabled: boolean
  quietStart: string // HH:mm
  quietEnd: string
}

function loadLayout(): LayoutMode {
  const v = localStorage.getItem(LAYOUT_KEY)
  return v === 'detached' ? 'detached' : 'embedded'
}

function loadNotify(): NotifyPrefs {
  try {
    const raw = localStorage.getItem(NOTIFY_KEY)
    if (raw) {
      const p = JSON.parse(raw) as Partial<NotifyPrefs>
      return {
        desktopEnabled: p.desktopEnabled !== false,
        quietHoursEnabled: !!p.quietHoursEnabled,
        quietStart: p.quietStart || '22:00',
        quietEnd: p.quietEnd || '08:00',
      }
    }
  } catch {
    /* ignore */
  }
  return {
    desktopEnabled: true,
    quietHoursEnabled: false,
    quietStart: '22:00',
    quietEnd: '08:00',
  }
}

function minutesOf(hhmm: string): number {
  const [h, m] = hhmm.split(':').map((x) => parseInt(x, 10) || 0)
  return h * 60 + m
}

/** True if now falls in quiet window (supports overnight ranges). */
export function inQuietHours(now: Date, start: string, end: string): boolean {
  const cur = now.getHours() * 60 + now.getMinutes()
  const s = minutesOf(start)
  const e = minutesOf(end)
  if (s === e) return true
  if (s < e) return cur >= s && cur < e
  return cur >= s || cur < e
}

export const useSettingsStore = defineStore('settings', {
  state: () => ({
    layoutMode: loadLayout() as LayoutMode,
    chatAlwaysOnTop: localStorage.getItem(AOT_KEY) === '1',
    notify: loadNotify() as NotifyPrefs,
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
    persistNotify() {
      localStorage.setItem(NOTIFY_KEY, JSON.stringify(this.notify))
    },
    setNotifyEnabled(on: boolean) {
      this.notify.desktopEnabled = on
      this.persistNotify()
    },
    setQuietHours(enabled: boolean, start?: string, end?: string) {
      this.notify.quietHoursEnabled = enabled
      if (start) this.notify.quietStart = start
      if (end) this.notify.quietEnd = end
      this.persistNotify()
    },
    applyCloudNotify(n: {
      desktop_enabled?: boolean
      quiet_hours_enabled?: boolean
      quiet_start?: string
      quiet_end?: string
    }) {
      this.notify = {
        desktopEnabled: n.desktop_enabled !== false,
        quietHoursEnabled: !!n.quiet_hours_enabled,
        quietStart: n.quiet_start || '22:00',
        quietEnd: n.quiet_end || '08:00',
      }
      this.persistNotify()
    },
    cloudNotifyPayload() {
      return {
        desktop_enabled: this.notify.desktopEnabled,
        quiet_hours_enabled: this.notify.quietHoursEnabled,
        quiet_start: this.notify.quietStart,
        quiet_end: this.notify.quietEnd,
      }
    },
    /** Whether desktop toast should fire for an incoming message. */
    shouldDesktopNotify(): boolean {
      if (!this.notify.desktopEnabled) return false
      if (
        this.notify.quietHoursEnabled &&
        inQuietHours(new Date(), this.notify.quietStart, this.notify.quietEnd)
      ) {
        return false
      }
      return true
    },
  },
})
