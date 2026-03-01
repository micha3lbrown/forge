import Markdown from './Markdown';
import ToolCallCard from './ToolCallCard';
import type { Message } from '../lib/api';

interface Props {
  message: Message;
  toolResults: Map<string, string>;
}

export default function MessageBubble({ message, toolResults }: Props) {
  if (message.role === 'user') {
    return (
      <div className="bubble user">
        <div className="role">You</div>
        <div className="content">{message.content}</div>
      </div>
    );
  }

  if (message.role === 'assistant') {
    return (
      <div className="bubble assistant">
        <div className="role">Forge</div>
        {message.content && <Markdown content={message.content} />}
        {message.tool_calls?.map((tc) => (
          <ToolCallCard
            key={tc.id}
            name={tc.name}
            args={tc.arguments}
            result={toolResults.get(tc.id)}
          />
        ))}
      </div>
    );
  }

  // Tool and system messages are not rendered directly
  return null;
}
