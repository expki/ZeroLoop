import { create } from 'zustand'
import type { Chat, Message } from '../types'

interface ChatState {
  chats: Chat[]
  selectedChatId: string | null
  messages: Message[]
  paused: boolean
  progress: string
  progressActive: boolean
  selectChat: (id: string | null) => void
  createChat: () => void
  deleteChat: (id: string) => void
  sendMessage: (content: string) => void
  togglePause: () => void
  clearChat: () => void
}

const mockMessages: Record<string, Message[]> = {
  'chat-1': [
    {
      id: 'msg-1', no: 1, type: 'user', heading: '', agentno: 0,
      content: 'Can you help me create a Python web scraper that extracts product prices from an e-commerce site?',
      timestamp: Date.now() - 300000,
    },
    {
      id: 'msg-2', no: 2, type: 'agent', heading: 'Analyzing request', agentno: 0,
      content: "I'll help you create a web scraper. Let me plan the approach and set up the necessary tools.",
      kvps: { tool_name: 'thinking', visual: 'plan' },
      timestamp: Date.now() - 299000,
    },
    {
      id: 'msg-3', no: 3, type: 'tool', heading: 'Installing dependencies', agentno: 0,
      content: 'pip install requests beautifulsoup4 lxml',
      kvps: { tool_name: 'code_execution', runtime: 'python' },
      timestamp: Date.now() - 298000,
    },
    {
      id: 'msg-4', no: 4, type: 'code_exe', heading: 'Creating scraper', agentno: 0,
      content: '$ python scraper.py\nScraping https://example-store.com/products...\nFound 24 products\nExtracting prices...\nDone! Saved to products.csv',
      kvps: { runtime: 'python', session: 'default' },
      timestamp: Date.now() - 297000,
    },
    {
      id: 'msg-5', no: 5, type: 'response', heading: 'Response', agentno: 0,
      content: "I've created a Python web scraper that:\n\n1. **Fetches** product pages using `requests`\n2. **Parses** HTML with `BeautifulSoup`\n3. **Extracts** product names, prices, and availability\n4. **Saves** results to `products.csv`\n\n```python\nimport requests\nfrom bs4 import BeautifulSoup\nimport csv\n\ndef scrape_products(url):\n    response = requests.get(url)\n    soup = BeautifulSoup(response.text, \"lxml\")\n    products = []\n    for item in soup.select(\".product-card\"):\n        name = item.select_one(\".title\").text.strip()\n        price = item.select_one(\".price\").text.strip()\n        products.append({\"name\": name, \"price\": price})\n    return products\n```\n\nThe scraper found **24 products** and saved them to `products.csv`. Would you like me to add any additional features like pagination or price monitoring?",
      timestamp: Date.now() - 296000,
    },
    {
      id: 'msg-6', no: 6, type: 'user', heading: '', agentno: 0,
      content: 'Yes, add pagination support and also filter products under $50.',
      timestamp: Date.now() - 200000,
    },
    {
      id: 'msg-7', no: 7, type: 'agent', heading: 'Enhancing scraper', agentno: 0,
      content: 'Adding pagination support and price filtering to the scraper.',
      kvps: { tool_name: 'thinking', visual: 'plan' },
      timestamp: Date.now() - 199000,
    },
    {
      id: 'msg-8', no: 8, type: 'code_exe', heading: 'Testing pagination', agentno: 0,
      content: '$ python scraper.py --pages 5 --max-price 50\nPage 1: 24 products (8 under $50)\nPage 2: 24 products (12 under $50)\nPage 3: 24 products (6 under $50)\nPage 4: 18 products (4 under $50)\nPage 5: 10 products (3 under $50)\nTotal: 33 products under $50\nSaved to products_filtered.csv',
      kvps: { runtime: 'python', session: 'default' },
      timestamp: Date.now() - 198000,
    },
    {
      id: 'msg-9', no: 9, type: 'response', heading: 'Response', agentno: 0,
      content: "Done! I've enhanced the scraper with:\n\n- **Pagination**: Automatically follows \"Next\" links across multiple pages\n- **Price filtering**: `--max-price 50` flag to only include products under $50\n- **Results**: Found **33 products** under $50 across 5 pages\n\nThe filtered results are saved to `products_filtered.csv`.",
      timestamp: Date.now() - 197000,
    },
  ],
  'chat-2': [
    {
      id: 'msg-20', no: 1, type: 'user', heading: '', agentno: 0,
      content: 'Set up a PostgreSQL database with tables for a blog application.',
      timestamp: Date.now() - 600000,
    },
    {
      id: 'msg-21', no: 2, type: 'agent', heading: 'Planning schema', agentno: 0,
      content: 'Designing the database schema for a blog application with users, posts, comments, and tags.',
      kvps: { tool_name: 'thinking', visual: 'plan' },
      timestamp: Date.now() - 599000,
    },
    {
      id: 'msg-22', no: 3, type: 'code_exe', heading: 'Creating tables', agentno: 0,
      content: '$ psql -U admin -d blogdb\nCREATE TABLE users...\nCREATE TABLE posts...\nCREATE TABLE comments...\nCREATE TABLE tags...\nAll tables created successfully.',
      kvps: { runtime: 'shell', session: 'psql' },
      timestamp: Date.now() - 598000,
    },
    {
      id: 'msg-23', no: 4, type: 'response', heading: 'Response', agentno: 0,
      content: "I've set up the PostgreSQL database with the following schema:\n\n- **users** - id, username, email, password_hash, created_at\n- **posts** - id, title, content, author_id (FK), published_at\n- **comments** - id, post_id (FK), author_id (FK), content, created_at\n- **tags** - id, name; with a many-to-many join table `post_tags`\n\nAll tables include proper indexes and foreign key constraints.",
      timestamp: Date.now() - 597000,
    },
  ],
  'chat-3': [
    {
      id: 'msg-30', no: 1, type: 'user', heading: '', agentno: 0,
      content: 'Help me optimize this slow SQL query that joins 5 tables and takes over 30 seconds.',
      timestamp: Date.now() - 900000,
    },
    {
      id: 'msg-31', no: 2, type: 'response', heading: 'Response', agentno: 0,
      content: "I'd be happy to help optimize your SQL query. Could you share the query and any relevant table schemas? Also, if you have the `EXPLAIN ANALYZE` output, that would be very helpful for identifying bottlenecks.",
      timestamp: Date.now() - 899000,
    },
  ],
}

const mockChats: Chat[] = [
  { id: 'chat-1', name: 'Web scraper project', created_at: new Date(Date.now() - 300000).toISOString(), running: true },
  { id: 'chat-2', name: 'Database setup', created_at: new Date(Date.now() - 600000).toISOString(), running: false },
  { id: 'chat-3', name: 'SQL optimization', created_at: new Date(Date.now() - 900000).toISOString(), running: false },
]

export const useChatStore = create<ChatState>((set, get) => ({
  chats: mockChats,
  selectedChatId: null,
  messages: [],
  paused: false,
  progress: '',
  progressActive: false,

  selectChat: (id) => {
    set({
      selectedChatId: id,
      messages: id ? (mockMessages[id] || []) : [],
    })
  },

  createChat: () => {
    const id = `chat-${Date.now()}`
    const chat: Chat = {
      id,
      name: 'New Chat',
      created_at: new Date().toISOString(),
      running: false,
    }
    set((state) => ({
      chats: [chat, ...state.chats],
      selectedChatId: id,
      messages: [],
    }))
  },

  deleteChat: (id) => {
    set((state) => ({
      chats: state.chats.filter((c) => c.id !== id),
      selectedChatId: state.selectedChatId === id ? null : state.selectedChatId,
      messages: state.selectedChatId === id ? [] : state.messages,
    }))
  },

  sendMessage: (content) => {
    const { selectedChatId, messages } = get()
    if (!selectedChatId || !content.trim()) return
    const newMessage: Message = {
      id: `msg-${Date.now()}`,
      no: messages.length + 1,
      type: 'user',
      heading: '',
      content: content.trim(),
      timestamp: Date.now(),
      agentno: 0,
    }
    set((state) => ({
      messages: [...state.messages, newMessage],
    }))
  },

  togglePause: () => set((state) => ({ paused: !state.paused })),
  clearChat: () => set({ messages: [] }),
}))
