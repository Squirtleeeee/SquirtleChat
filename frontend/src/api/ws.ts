export type WSFrame = { type: string; payload?: unknown }
export type WSStatus = 'connecting' | 'open' | 'closed'

type AckPayload = { client_msg_id: string; msg_id?: number; seq?: number; status?: string }

const BASE_DELAY_MS = 1500
const MAX_DELAY_MS = 30000

export class WSClient {
  private ws: WebSocket | null = null
  private wsBase = ''
  private deviceId = ''
  private token = ''
  private tokenFn: (() => string) | null = null
  private handlers: Array<(f: WSFrame) => void> = []
  private statusHandlers: Array<(s: WSStatus) => void> = []
  private reconnectTimer = 0
  private closed = false
  private attempt = 0
  status: WSStatus = 'closed'

  /** Consecutive failed reconnect attempts (0 when connected or first try). */
  get reconnectAttempt() {
    return this.attempt
  }

  connect(wsBase: string, token: string, deviceId: string) {
    this.closed = false
    this.wsBase = wsBase
    this.token = token
    this.deviceId = deviceId
    this.attempt = 0
    this.open()
  }

  onTokenRefresh(fn: () => string) {
    this.tokenFn = fn
  }

  refreshToken(token: string) {
    this.token = token
  }

  private setStatus(s: WSStatus) {
    this.status = s
    this.statusHandlers.forEach((h) => h(s))
  }

  onStatus(fn: (s: WSStatus) => void) {
    this.statusHandlers = [fn]
    fn(this.status)
  }

  private buildUrl() {
    const t = this.tokenFn?.() || this.token
    return `${this.wsBase}/ws?token=${encodeURIComponent(t)}&device_id=${encodeURIComponent(this.deviceId)}`
  }

  private nextDelay() {
    const delay = Math.min(BASE_DELAY_MS * 2 ** this.attempt, MAX_DELAY_MS)
    this.attempt += 1
    return delay
  }

  private detachSocket() {
    if (!this.ws) return
    this.ws.onopen = null
    this.ws.onmessage = null
    this.ws.onclose = null
    this.ws.onerror = null
    try {
      this.ws.close()
    } catch {
      /* ignore */
    }
    this.ws = null
  }

  private open() {
    if (this.closed) return
    window.clearTimeout(this.reconnectTimer)
    this.detachSocket()
    this.setStatus('connecting')
    const socket = new WebSocket(this.buildUrl())
    this.ws = socket
    socket.onopen = () => {
      if (this.ws !== socket) return
      this.attempt = 0
      this.setStatus('open')
    }
    socket.onmessage = (e) => {
      if (this.ws !== socket) return
      try {
        const frame = JSON.parse(e.data) as WSFrame
        if (frame.type === 'ping') {
          this.send({ type: 'pong' })
          return
        }
        this.handlers.forEach((h) => h(frame))
      } catch {
        /* ignore */
      }
    }
    socket.onclose = () => {
      if (this.ws !== socket) return
      this.ws = null
      this.setStatus('closed')
      if (this.closed) return
      window.clearTimeout(this.reconnectTimer)
      const delay = this.nextDelay()
      this.reconnectTimer = window.setTimeout(() => this.open(), delay)
    }
  }

  /** Force an immediate reconnect attempt (resets backoff). */
  forceReconnect() {
    if (this.closed) return
    window.clearTimeout(this.reconnectTimer)
    this.attempt = 0
    this.open()
  }

  on(fn: (f: WSFrame) => void) {
    this.handlers = [fn]
  }

  send(frame: WSFrame): boolean {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(frame))
      return true
    }
    return false
  }

  sendMessage(payload: Record<string, unknown>): boolean {
    return this.send({ type: 'message', payload })
  }

  sendTyping(conversationId: string, typing: boolean): boolean {
    return this.send({
      type: 'typing',
      payload: { conversation_id: conversationId, typing },
    })
  }

  close() {
    this.closed = true
    window.clearTimeout(this.reconnectTimer)
    this.detachSocket()
    this.handlers = []
    this.setStatus('closed')
  }
}

export type { AckPayload }
