import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { api } from './api'

describe('api', () => {
  const mockFetch = vi.fn()

  beforeEach(() => {
    (globalThis as Record<string, unknown>).fetch = mockFetch
    mockFetch.mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('lists chats', async () => {
    const chats = [{ id: '1', project_id: 'p1', name: 'Test', created_at: '2024-01-01', running: false }]
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(chats),
    })

    const result = await api.listChats()
    expect(result).toEqual(chats)
    expect(mockFetch).toHaveBeenCalledWith('/api/chats', expect.objectContaining({
      headers: { 'Content-Type': 'application/json' },
    }))
  })

  it('creates a chat', async () => {
    const chat = { id: '2', project_id: 'p1', name: 'My Chat', created_at: '2024-01-01', running: false }
    mockFetch.mockResolvedValue({
      ok: true,
      status: 201,
      json: () => Promise.resolve(chat),
    })

    const result = await api.createChat('p1', 'My Chat')
    expect(result).toEqual(chat)
    expect(mockFetch).toHaveBeenCalledWith('/api/chats', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ name: 'My Chat', project_id: 'p1' }),
    }))
  })

  it('creates a chat with default name', async () => {
    const chat = { id: '3', project_id: 'p1', name: 'New Chat', created_at: '2024-01-01', running: false }
    mockFetch.mockResolvedValue({
      ok: true,
      status: 201,
      json: () => Promise.resolve(chat),
    })

    const result = await api.createChat('p1')
    expect(result).toEqual(chat)
    expect(mockFetch).toHaveBeenCalledWith('/api/chats', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ name: 'New Chat', project_id: 'p1' }),
    }))
  })

  it('deletes a chat', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
    })

    await api.deleteChat('abc')
    expect(mockFetch).toHaveBeenCalledWith('/api/chats/abc', expect.objectContaining({
      method: 'DELETE',
    }))
  })

  it('gets chat messages', async () => {
    const messages = [{ id: 'm1', no: 1, type: 'user', heading: '', content: 'hello', timestamp: '2024-01-01', agentno: 0 }]
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(messages),
    })

    const result = await api.getChatMessages('chat1')
    expect(result).toEqual(messages)
    expect(mockFetch).toHaveBeenCalledWith('/api/chats/chat1/messages', expect.objectContaining({
      headers: { 'Content-Type': 'application/json' },
    }))
  })

  it('handles API errors with JSON error body', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: () => Promise.resolve({ error: 'something went wrong' }),
    })

    await expect(api.listChats()).rejects.toThrow('something went wrong')
  })

  it('handles API errors with non-JSON response', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: () => Promise.reject(new Error('not json')),
    })

    await expect(api.listChats()).rejects.toThrow('Internal Server Error')
  })
})
