export type DesktopChatPayload = {
  type: 'friend' | 'group'
  id: string
  title?: string
}

export type SquirtleDesktopAPI = {
  isElectron: boolean
  openChatWindow: (payload: DesktopChatPayload) => Promise<{ ok: boolean; focused?: boolean }>
  setAlwaysOnTop: (flag: boolean) => Promise<{ ok: boolean }>
  getWindowRole: () => Promise<string>
  focusMain: () => Promise<{ ok: boolean }>
  setShellMode: (mode: 'login' | 'main') => Promise<{ ok: boolean; mode?: string }>
  windowMinimize: () => Promise<{ ok: boolean }>
  windowMaximize: () => Promise<{ ok: boolean; maximized?: boolean }>
  windowClose: () => Promise<{ ok: boolean }>
  isMaximized: () => Promise<boolean>
}

declare global {
  interface Window {
    squirtleDesktop?: SquirtleDesktopAPI
  }
}

export function isDesktopApp() {
  return !!window.squirtleDesktop?.isElectron
}

/** Open a conversation in a separate window (Electron IPC or browser popup). */
export async function openDetachedChat(payload: DesktopChatPayload) {
  const api = window.squirtleDesktop
  if (api?.openChatWindow) {
    return api.openChatWindow(payload)
  }
  const q = new URLSearchParams({
    type: payload.type,
    id: payload.id,
    title: payload.title || '',
  })
  const url = `${location.origin}${location.pathname}#/popup-chat?${q.toString()}`
  window.open(url, `squirtle-chat-${payload.type}-${payload.id}`, 'width=480,height=720,menubar=no,toolbar=no,location=no,status=no')
  return { ok: true }
}
