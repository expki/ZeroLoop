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

  it('lists agents', async () => {
    const agents = [{ id: '1', project_id: 'p1', name: 'Test', created_at: '2024-01-01', running: false }]
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(agents),
    })

    const result = await api.listAgents()
    expect(result).toEqual(agents)
    expect(mockFetch).toHaveBeenCalledWith('/api/agents', expect.objectContaining({
      headers: { 'Content-Type': 'application/json' },
    }))
  })

  it('creates an agent', async () => {
    const agent = { id: '2', project_id: 'p1', name: 'My Agent', created_at: '2024-01-01', running: false }
    mockFetch.mockResolvedValue({
      ok: true,
      status: 201,
      json: () => Promise.resolve(agent),
    })

    const result = await api.createAgent('p1', 'My Agent')
    expect(result).toEqual(agent)
    expect(mockFetch).toHaveBeenCalledWith('/api/agents', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ name: 'My Agent', project_id: 'p1' }),
    }))
  })

  it('creates an agent with default name', async () => {
    const agent = { id: '3', project_id: 'p1', name: 'New Agent', created_at: '2024-01-01', running: false }
    mockFetch.mockResolvedValue({
      ok: true,
      status: 201,
      json: () => Promise.resolve(agent),
    })

    const result = await api.createAgent('p1')
    expect(result).toEqual(agent)
    expect(mockFetch).toHaveBeenCalledWith('/api/agents', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ name: 'New Agent', project_id: 'p1' }),
    }))
  })

  it('deletes an agent', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
    })

    await api.deleteAgent('abc')
    expect(mockFetch).toHaveBeenCalledWith('/api/agents/abc', expect.objectContaining({
      method: 'DELETE',
    }))
  })

  it('gets agent messages', async () => {
    const messages = [{ id: 'm1', no: 1, type: 'user', heading: '', content: 'hello', timestamp: '2024-01-01', agentno: 0 }]
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(messages),
    })

    const result = await api.getAgentMessages('agent1')
    expect(result).toEqual(messages)
    expect(mockFetch).toHaveBeenCalledWith('/api/agents/agent1/messages', expect.objectContaining({
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

    await expect(api.listAgents()).rejects.toThrow('something went wrong')
  })

  it('handles API errors with non-JSON response', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: () => Promise.reject(new Error('not json')),
    })

    await expect(api.listAgents()).rejects.toThrow('Internal Server Error')
  })
})
