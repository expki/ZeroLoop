// eslint-disable-next-line @typescript-eslint/no-explicit-any
type WSMessageHandler = (payload: any) => void

interface WSMessage {
  type: string
  payload: any
}

type ReconnectCallback = () => void

class WebSocketService {
  private ws: WebSocket | null = null
  private url: string
  private handlers: Map<string, WSMessageHandler[]> = new Map()
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private reconnectDelay = 1000
  private maxReconnectDelay = 30000
  private intentionalClose = false
  private reconnectCallbacks: ReconnectCallback[] = []

  constructor() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    this.url = `${protocol}//${window.location.host}/ws`
  }

  connect() {
    if (this.ws?.readyState === WebSocket.OPEN) return

    this.intentionalClose = false
    this.ws = new WebSocket(this.url)

    this.ws.onopen = () => {
      this.reconnectDelay = 1000
      this.emit('_status', { status: 'connected' })
      // Fire reconnect callbacks to resubscribe
      for (const cb of this.reconnectCallbacks) {
        cb()
      }
    }

    this.ws.onmessage = (event) => {
      try {
        const msg: WSMessage = JSON.parse(event.data)
        this.emit(msg.type, msg.payload)
      } catch {
        // ignore malformed messages
      }
    }

    this.ws.onclose = () => {
      this.emit('_status', { status: 'disconnected' })
      if (!this.intentionalClose) {
        this.scheduleReconnect()
      }
    }

    this.ws.onerror = () => {
      this.emit('_status', { status: 'disconnected' })
    }
  }

  disconnect() {
    this.intentionalClose = true
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    this.ws?.close()
    this.ws = null
  }

  private scheduleReconnect() {
    if (this.reconnectTimer) return
    this.emit('_status', { status: 'degraded' })
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay)
      this.connect()
    }, this.reconnectDelay)
  }

  send(type: string, payload: any) {
    if (this.ws?.readyState !== WebSocket.OPEN) return
    this.ws.send(JSON.stringify({ type, payload }))
  }

  on(type: string, handler: WSMessageHandler) {
    const handlers = this.handlers.get(type) || []
    handlers.push(handler)
    this.handlers.set(type, handlers)
    return () => {
      const h = this.handlers.get(type) || []
      this.handlers.set(type, h.filter((fn) => fn !== handler))
    }
  }

  /** Register a callback that fires on every (re)connect. Returns unsubscribe function. */
  onReconnect(cb: ReconnectCallback) {
    this.reconnectCallbacks.push(cb)
    return () => {
      this.reconnectCallbacks = this.reconnectCallbacks.filter((fn) => fn !== cb)
    }
  }

  private emit(type: string, payload: any) {
    const handlers = this.handlers.get(type) || []
    for (const handler of handlers) {
      handler(payload)
    }
  }
}

export const ws = new WebSocketService()
