import { useEffect, useRef, useState, useCallback, useMemo } from 'react';
import { useStore } from '../lib/store';
import {
  getMessages,
  createSession,
  updateSession,
  listSessions,
  listProviders,
} from '../lib/api';
import { ForgeWebSocket } from '../lib/ws';
import type { WSEvent, FallbackOption } from '../lib/ws';
import MessageBubble from './MessageBubble';
import ToolCallCard from './ToolCallCard';
import Markdown from './Markdown';
import ModelSelector from './ModelSelector';

export default function ChatView() {
  const activeSessionId = useStore((s) => s.activeSessionId);
  const sessions = useStore((s) => s.sessions);
  const messages = useStore((s) => s.messages);
  const isStreaming = useStore((s) => s.isStreaming);
  const streamingText = useStore((s) => s.streamingText);
  const streamingToolCalls = useStore((s) => s.streamingToolCalls);
  const errorMessage = useStore((s) => s.errorMessage);
  const showModelPicker = useStore((s) => s.showModelPicker);
  const fallbackOptions = useStore((s) => s.fallbackOptions);
  const systemMessage = useStore((s) => s.systemMessage);
  const store = useStore;

  const [input, setInput] = useState('');
  const chatRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<ForgeWebSocket | null>(null);

  const activeSession = useMemo(
    () => sessions.find((s) => s.id === activeSessionId) ?? null,
    [sessions, activeSessionId],
  );

  // Build tool results map
  const toolResults = useMemo(() => {
    const map = new Map<string, string>();
    for (const m of messages) {
      if (m.role === 'tool' && m.tool_call_id) {
        map.set(m.tool_call_id, m.content || '');
      }
    }
    return map;
  }, [messages]);

  const displayMessages = useMemo(
    () => messages.filter((m) => m.role === 'user' || m.role === 'assistant'),
    [messages],
  );

  const handleWSEvent = useCallback((event: WSEvent) => {
    const s = store.getState();
    switch (event.type) {
      case 'text_delta':
        s.addStreamDelta(event.content || '');
        break;
      case 'tool_call':
        s.addStreamToolCall({
          name: event.name || '',
          args: event.args || {},
        });
        break;
      case 'tool_result':
        s.updateToolCallResult(event.name || '', event.content || '');
        break;
      case 'done': {
        const sid = store.getState().activeSessionId;
        if (sid) {
          getMessages(sid).then((msgs) => s.setMessages(msgs));
          listSessions().then((sess) => s.setSessions(sess));
        }
        s.resetStreaming();
        break;
      }
      case 'error': {
        s.resetStreaming();
        const raw = event.content || 'Unknown error';
        if (event.fallback_options && event.fallback_options.length > 0) {
          const msg =
            raw.includes('connection refused') || raw.includes('ECONNREFUSED')
              ? 'Could not connect to the LLM provider. Is it running?'
              : raw.includes('not found') || raw.includes('404')
                ? 'Model not found. Try switching to an available provider:'
                : `Error: ${raw}`;
          s.setError(msg, event.fallback_options);
        } else {
          const modelMatch = raw.match(/model '([^']+)' not found/);
          if (modelMatch) {
            s.setError(`Model "${modelMatch[1]}" is not available.`);
          } else if (raw.includes('404')) {
            s.setError('Model not found. It may need to be pulled or the provider is unreachable.');
          } else if (raw.includes('connection refused') || raw.includes('ECONNREFUSED')) {
            s.setError('Could not connect to the LLM provider. Is it running?');
          } else {
            s.setError(raw);
          }
        }
        break;
      }
    }
  }, [store]);

  // Connect/disconnect WS when session changes
  useEffect(() => {
    if (!activeSessionId) return;

    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    store.getState().resetStreaming();

    getMessages(activeSessionId)
      .then((msgs) => store.getState().setMessages(msgs))
      .catch(() => store.getState().setMessages([]));

    const ws = new ForgeWebSocket(activeSessionId, handleWSEvent);
    ws.connect();
    wsRef.current = ws;

    return () => {
      ws.close();
      if (wsRef.current === ws) wsRef.current = null;
    };
  }, [activeSessionId, handleWSEvent, store]);

  // Auto-scroll
  useEffect(() => {
    if (chatRef.current) {
      chatRef.current.scrollTop = chatRef.current.scrollHeight;
    }
  }, [messages, streamingText, streamingToolCalls]);

  async function handleSlashCommand(text: string): Promise<boolean> {
    if (!text.startsWith('/')) return false;
    const parts = text.split(/\s+/);
    const cmd = parts[0].toLowerCase();
    const arg = parts.slice(1).join(' ');
    const s = store.getState();

    switch (cmd) {
      case '/model': {
        if (!arg) {
          if (activeSession) {
            s.setSystemMessage(`Current model: ${activeSession.provider}/${activeSession.model}`);
          }
          return true;
        }
        if (!activeSessionId || !activeSession) return true;
        let provider = activeSession.provider;
        let model = arg;
        if (arg.includes('/')) {
          [provider, model] = arg.split('/', 2);
        } else {
          try {
            const providers = await listProviders();
            const match = providers.find((p) => p.name === arg);
            if (match) {
              provider = arg;
              model = match.models?.default || arg;
            }
          } catch {
            /* treat as model name */
          }
        }
        try {
          const updated = await updateSession(activeSessionId, { provider, model });
          s.updateSessionInList(updated);
          s.setSystemMessage(`Switched to ${updated.provider}/${updated.model}`);
          if (wsRef.current) {
            wsRef.current.close();
            const ws = new ForgeWebSocket(activeSessionId, handleWSEvent);
            ws.connect();
            wsRef.current = ws;
          }
        } catch (err: unknown) {
          s.setSystemMessage(`Error: ${err instanceof Error ? err.message : String(err)}`);
        }
        return true;
      }
      case '/reset':
        s.setMessages([]);
        s.setSystemMessage('Conversation history cleared.');
        return true;
      case '/help':
        s.setSystemMessage('Commands: /model [provider/model], /reset, /help');
        return true;
      default:
        return false;
    }
  }

  async function handleSend() {
    const text = input.trim();
    if (!text || isStreaming) return;

    setInput('');
    const s = store.getState();
    s.setSystemMessage('');
    s.clearError();

    if (await handleSlashCommand(text)) return;

    let sid = activeSessionId;
    if (!sid) {
      try {
        const sess = await createSession({});
        s.prependSession(sess);
        s.setActiveSessionId(sess.id);
        sid = sess.id;
        // Wait for WS connect — the useEffect will handle it
        await new Promise<void>((resolve) => {
          const check = () => {
            if (wsRef.current?.connected) resolve();
            else setTimeout(check, 50);
          };
          setTimeout(check, 50);
        });
      } catch (err) {
        console.error('Failed to create session:', err);
        return;
      }
    }

    s.setIsStreaming(true);
    s.addStreamDelta(''); // reset
    store.setState({ streamingText: '', streamingToolCalls: [] });
    s.addUserMessage(text);

    // Wait for WS if needed
    await new Promise<void>((resolve) => {
      const check = () => {
        if (wsRef.current?.connected) {
          resolve();
          return;
        }
        setTimeout(check, 50);
      };
      check();
    });
    wsRef.current!.send(text);
  }

  function handleKeydown(e: React.KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }

  async function handleModelChange(provider: string, model: string) {
    if (!activeSessionId) return;
    const s = store.getState();
    try {
      const updated = await updateSession(activeSessionId, { provider, model });
      s.updateSessionInList(updated);
      s.clearError();
      s.setSystemMessage(`Switched to ${updated.provider}/${updated.model}`);
      if (wsRef.current) {
        wsRef.current.close();
        const ws = new ForgeWebSocket(activeSessionId, handleWSEvent);
        ws.connect();
        wsRef.current = ws;
      }
    } catch (err: unknown) {
      s.setError(`Failed to switch model: ${err instanceof Error ? err.message : String(err)}`);
    }
  }

  async function handleFallbackSwitch(opt: FallbackOption) {
    if (!activeSessionId) return;
    const s = store.getState();
    try {
      const updated = await updateSession(activeSessionId, {
        provider: opt.provider,
        model: opt.model,
      });
      s.updateSessionInList(updated);
      s.clearError();
      s.setSystemMessage(`Switched to ${updated.provider}/${updated.model}`);
      if (wsRef.current) {
        wsRef.current.close();
        const ws = new ForgeWebSocket(activeSessionId, handleWSEvent);
        ws.connect();
        wsRef.current = ws;
      }
    } catch (err: unknown) {
      s.setError(`Failed to switch: ${err instanceof Error ? err.message : String(err)}`);
    }
  }

  if (!activeSessionId) {
    return (
      <div className="chat-view">
        <div className="empty-state">
          <h2>Welcome to Forge</h2>
          <p>Select a session from the sidebar or start a new chat.</p>
          <button
            className="start-btn"
            onClick={async () => {
              const s = store.getState();
              const sess = await createSession({});
              s.prependSession(sess);
              s.setActiveSessionId(sess.id);
            }}
          >
            Start New Chat
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="chat-view">
      <div className="chat-header">
        {activeSession && (
          <>
            <span className="chat-title">{activeSession.title || 'New Chat'}</span>
            <span className="chat-meta">{activeSession.provider}/{activeSession.model}</span>
          </>
        )}
      </div>

      <div className="chat-messages" ref={chatRef}>
        {displayMessages.map((msg, i) => (
          <MessageBubble key={i} message={msg} toolResults={toolResults} />
        ))}

        {systemMessage && <div className="system-message">{systemMessage}</div>}

        {errorMessage && (
          <div className="error-banner">
            <span className="error-text">{errorMessage}</span>
            {showModelPicker && fallbackOptions.length > 0 && (
              <div className="fallback-buttons">
                <p className="error-hint">Quick switch:</p>
                {fallbackOptions.map((opt) => (
                  <button
                    key={`${opt.provider}/${opt.model}`}
                    className="fallback-btn"
                    onClick={() => handleFallbackSwitch(opt)}
                  >
                    {opt.provider}/{opt.model}
                  </button>
                ))}
              </div>
            )}
            {showModelPicker && fallbackOptions.length === 0 && activeSession && (
              <>
                <p className="error-hint">Choose an available model:</p>
                <ModelSelector
                  selectedProvider={activeSession.provider}
                  selectedModel={activeSession.model}
                  onChange={handleModelChange}
                />
              </>
            )}
          </div>
        )}

        {isStreaming && (
          <div className="bubble assistant streaming">
            <div className="role">Forge</div>
            {streamingToolCalls.map((tc, i) => (
              <ToolCallCard key={i} name={tc.name} args={tc.args} result={tc.result} />
            ))}
            {streamingText && <Markdown content={streamingText} />}
            <span className="cursor">|</span>
          </div>
        )}
      </div>

      <div className="chat-input">
        <textarea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeydown}
          placeholder="Type a message... (Enter to send, Shift+Enter for newline)"
          disabled={isStreaming}
          rows={1}
        />
        <button onClick={handleSend} disabled={isStreaming || !input.trim()}>
          Send
        </button>
      </div>
    </div>
  );
}
