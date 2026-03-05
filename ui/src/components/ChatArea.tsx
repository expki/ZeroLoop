import TopBar from './TopBar'
import MessageList from './MessageList'
import ChatInput from './ChatInput'
import './ChatArea.css'

function ChatArea() {
  return (
    <div className="chat-area">
      <TopBar />
      <MessageList />
      <ChatInput />
    </div>
  )
}

export default ChatArea
