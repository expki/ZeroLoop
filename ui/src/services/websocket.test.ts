import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

describe('WebSocketService', () => {
  let mockInstances: MockWebSocket[]

  class MockWebSocket {
    static OPEN = 1
    static CLOSED = 3
    static CONNECTING = 0
    static CLOSING = 2
    readyState = MockWebSocket.OPEN
    onopen: ((ev: Event) => void) | null = null
    onmessage: ((ev: MessageEvent) => void) | null = null
    onclose: ((ev: CloseEvent) => void) | null = null
    onerror: ((ev: Event) => void) | null = null

    send = vi.fn()
    close = vi.fn()
    addEventListener = vi.fn()
    removeEventListener = vi.fn()
    dispatchEvent = vi.fn(() => true)

    constructor(public url: string) {
      mockInstances.push(this)
    }
  }

  beforeEach(() => {
    mockInstances = []
    vi.resetModules()
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('exports a ws singleton with the expected interface', async () => {
    const { ws } = await import('./websocket')
    expect(ws).toBeDefined()
    expect(typeof ws.connect).toBe('function')
    expect(typeof ws.send).toBe('function')
    expect(typeof ws.on).toBe('function')
    expect(typeof ws.disconnect).toBe('function')
  })

  it('connect creates a WebSocket connection', async () => {
    const { ws } = await import('./websocket')
    ws.connect()
    expect(mockInstances.length).toBe(1)
    expect(mockInstances[0].url).toContain('/ws')
  })

  it('does not create duplicate connections if already open', async () => {
    const { ws } = await import('./websocket')
    ws.connect()
    ws.connect()
    expect(mockInstances.length).toBe(1)
  })

  it('send serializes and sends JSON when connected', async () => {
    const { ws } = await import('./websocket')
    ws.connect()
    const mockWs = mockInstances[0]
    ws.send('test_type', { data: 'hello' })
    expect(mockWs.send).toHaveBeenCalledWith(
      JSON.stringify({ type: 'test_type', payload: { data: 'hello' } })
    )
  })

  it('send does nothing when not connected', async () => {
    const { ws } = await import('./websocket')
    // Don't connect, just try to send
    ws.send('test', { data: 1 })
    // No error should be thrown, and no WebSocket instances created
    expect(mockInstances.length).toBe(0)
  })

  it('on registers a handler and returns an unsubscribe function', async () => {
    const { ws } = await import('./websocket')
    const handler = vi.fn()
    const unsub = ws.on('test_event', handler)
    expect(typeof unsub).toBe('function')
  })

  it('disconnect closes the connection', async () => {
    const { ws } = await import('./websocket')
    ws.connect()
    const mockWs = mockInstances[0]
    ws.disconnect()
    expect(mockWs.close).toHaveBeenCalled()
  })

  it('emits events to registered handlers on message', async () => {
    const { ws } = await import('./websocket')
    const handler = vi.fn()
    ws.on('chat_msg', handler)
    ws.connect()
    const mockWs = mockInstances[0]

    // Simulate incoming message
    const msgEvent = { data: JSON.stringify({ type: 'chat_msg', payload: { text: 'hi' } }) }
    mockWs.onmessage?.(msgEvent as MessageEvent)

    expect(handler).toHaveBeenCalledWith({ text: 'hi' })
  })

  it('unsubscribe removes the handler', async () => {
    const { ws } = await import('./websocket')
    const handler = vi.fn()
    const unsub = ws.on('chat_msg', handler)
    unsub()

    ws.connect()
    const mockWs = mockInstances[0]
    const msgEvent = { data: JSON.stringify({ type: 'chat_msg', payload: { text: 'hi' } }) }
    mockWs.onmessage?.(msgEvent as MessageEvent)

    expect(handler).not.toHaveBeenCalled()
  })

  it('ignores malformed messages without throwing', async () => {
    const { ws } = await import('./websocket')
    ws.connect()
    const mockWs = mockInstances[0]

    // Should not throw
    const msgEvent = { data: 'not valid json' }
    expect(() => {
      mockWs.onmessage?.(msgEvent as MessageEvent)
    }).not.toThrow()
  })
})
