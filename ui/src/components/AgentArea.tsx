import TopBar from './TopBar'
import MessageList from './MessageList'
import AgentInput from './AgentInput'
import './AgentArea.css'

function AgentArea() {
  return (
    <div className="agent-area">
      <TopBar />
      <MessageList />
      <AgentInput />
    </div>
  )
}

export default AgentArea
