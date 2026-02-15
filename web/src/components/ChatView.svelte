<script lang="ts">
  import { afterUpdate, onDestroy } from 'svelte';
  import { messages, isStreaming, streamingText, streamingToolCalls, activeSessionId, activeSession, sessions } from '../lib/stores';
  import { getMessages, createSession } from '../lib/api';
  import { ForgeWebSocket } from '../lib/ws';
  import type { WSEvent } from '../lib/ws';
  import type { Message } from '../lib/api';
  import MessageBubble from './MessageBubble.svelte';
  import ToolCallCard from './ToolCallCard.svelte';
  import Markdown from './Markdown.svelte';

  let input = '';
  let chatContainer: HTMLElement;
  let ws: ForgeWebSocket | null = null;

  // Build a map of tool call ID -> result from message history
  $: toolResults = buildToolResultMap($messages);

  function buildToolResultMap(msgs: Message[]): Map<string, string> {
    const map = new Map<string, string>();
    for (const m of msgs) {
      if (m.role === 'tool' && m.tool_call_id) {
        map.set(m.tool_call_id, m.content || '');
      }
    }
    return map;
  }

  // When active session changes, load messages and connect WebSocket
  $: if ($activeSessionId) {
    loadSession($activeSessionId);
  }

  async function loadSession(sessionId: string) {
    // Disconnect old WebSocket
    if (ws) {
      ws.close();
      ws = null;
    }

    // Reset streaming state
    $isStreaming = false;
    $streamingText = '';
    $streamingToolCalls = [];

    // Load messages
    try {
      $messages = await getMessages(sessionId);
    } catch (err) {
      console.error('Failed to load messages:', err);
      $messages = [];
    }

    // Connect WebSocket
    ws = new ForgeWebSocket(sessionId, handleWSEvent);
    ws.connect();
  }

  function handleWSEvent(event: WSEvent) {
    switch (event.type) {
      case 'text_delta':
        $streamingText += event.content || '';
        break;

      case 'tool_call':
        $streamingToolCalls = [...$streamingToolCalls, {
          name: event.name || '',
          args: event.args || {},
        }];
        break;

      case 'tool_result': {
        const calls = $streamingToolCalls;
        const idx = calls.findLastIndex(tc => tc.name === event.name && !tc.result);
        if (idx >= 0) {
          calls[idx].result = event.content;
          $streamingToolCalls = [...calls];
        }
        break;
      }

      case 'done':
        // Reload messages to get the full history
        if ($activeSessionId) {
          getMessages($activeSessionId).then(msgs => {
            $messages = msgs;
          });
          // Refresh session list to get updated title
          import('../lib/api').then(api => {
            api.listSessions().then(s => { $sessions = s; });
          });
        }
        $isStreaming = false;
        $streamingText = '';
        $streamingToolCalls = [];
        break;

      case 'error':
        $isStreaming = false;
        $streamingText = '';
        $streamingToolCalls = [];
        console.error('Agent error:', event.content);
        break;
    }
  }

  async function handleSend() {
    const text = input.trim();
    if (!text || $isStreaming) return;

    // If no active session, create one
    if (!$activeSessionId) {
      try {
        const sess = await createSession({});
        $sessions = [sess, ...$sessions];
        $activeSessionId = sess.id;
        // Wait for WebSocket to connect
        await new Promise(resolve => setTimeout(resolve, 200));
      } catch (err) {
        console.error('Failed to create session:', err);
        return;
      }
    }

    input = '';
    $isStreaming = true;
    $streamingText = '';
    $streamingToolCalls = [];

    // Add user message to display immediately
    $messages = [...$messages, { role: 'user', content: text }];

    // Send via WebSocket
    if (ws?.connected) {
      ws.send(text);
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }

  // Auto-scroll to bottom
  afterUpdate(() => {
    if (chatContainer) {
      chatContainer.scrollTop = chatContainer.scrollHeight;
    }
  });

  onDestroy(() => {
    if (ws) {
      ws.close();
    }
  });

  // Filter out system and tool messages for display
  $: displayMessages = $messages.filter(m => m.role === 'user' || m.role === 'assistant');
</script>

<div class="chat-view">
  {#if $activeSessionId}
    <div class="chat-header">
      {#if $activeSession}
        <span class="chat-title">{$activeSession.title || 'New Chat'}</span>
        <span class="chat-meta">{$activeSession.provider}/{$activeSession.model}</span>
      {/if}
    </div>

    <div class="chat-messages" bind:this={chatContainer}>
      {#each displayMessages as message}
        <MessageBubble {message} {toolResults} />
      {/each}

      {#if $isStreaming}
        <div class="bubble assistant streaming">
          <div class="role">Forge</div>
          {#each $streamingToolCalls as tc}
            <ToolCallCard name={tc.name} args={tc.args} result={tc.result} />
          {/each}
          {#if $streamingText}
            <Markdown content={$streamingText} />
          {/if}
          <span class="cursor">|</span>
        </div>
      {/if}
    </div>

    <div class="chat-input">
      <textarea
        bind:value={input}
        on:keydown={handleKeydown}
        placeholder="Type a message... (Enter to send, Shift+Enter for newline)"
        disabled={$isStreaming}
        rows="1"
      ></textarea>
      <button on:click={handleSend} disabled={$isStreaming || !input.trim()}>Send</button>
    </div>
  {:else}
    <div class="empty-state">
      <h2>Welcome to Forge</h2>
      <p>Select a session from the sidebar or start a new chat.</p>
      <button class="start-btn" on:click={() => {
        createSession({}).then(sess => {
          $sessions = [sess, ...$sessions];
          $activeSessionId = sess.id;
        });
      }}>Start New Chat</button>
    </div>
  {/if}
</div>

<style>
  .chat-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    height: 100vh;
    background: #0f0f23;
  }

  .chat-header {
    padding: 0.75rem 1rem;
    border-bottom: 1px solid #2a2a4a;
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .chat-title {
    font-weight: 600;
    color: #e0e0e0;
  }

  .chat-meta {
    font-size: 0.75rem;
    color: #808090;
  }

  .chat-messages {
    flex: 1;
    overflow-y: auto;
    padding: 1rem;
    display: flex;
    flex-direction: column;
  }

  .chat-input {
    padding: 0.75rem 1rem;
    border-top: 1px solid #2a2a4a;
    display: flex;
    gap: 0.5rem;
  }

  textarea {
    flex: 1;
    background: #1a1a2e;
    color: #e0e0e0;
    border: 1px solid #3a3a5c;
    border-radius: 6px;
    padding: 0.6rem;
    font-size: 0.9rem;
    font-family: inherit;
    resize: none;
    min-height: 2.4rem;
    max-height: 8rem;
  }

  textarea:focus {
    outline: none;
    border-color: #6c9bff;
  }

  textarea:disabled {
    opacity: 0.5;
  }

  .chat-input button {
    background: #4a6fa5;
    color: white;
    border: none;
    border-radius: 6px;
    padding: 0.5rem 1.25rem;
    cursor: pointer;
    font-size: 0.9rem;
    align-self: flex-end;
  }

  .chat-input button:hover:not(:disabled) {
    background: #5a7fb5;
  }

  .chat-input button:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .bubble.streaming {
    background: #1e1e36;
    border: 1px solid #2a2a4a;
    padding: 0.75rem 1rem;
    border-radius: 8px;
    margin-bottom: 0.75rem;
  }

  .role {
    font-size: 0.7rem;
    color: #808090;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-bottom: 0.3rem;
  }

  .cursor {
    animation: blink 1s step-end infinite;
    color: #6c9bff;
  }

  @keyframes blink {
    50% { opacity: 0; }
  }

  .empty-state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    color: #808090;
  }

  .empty-state h2 {
    color: #e0e0e0;
    margin-bottom: 0.5rem;
  }

  .start-btn {
    margin-top: 1.5rem;
    background: #4a6fa5;
    color: white;
    border: none;
    border-radius: 6px;
    padding: 0.6rem 1.5rem;
    cursor: pointer;
    font-size: 1rem;
  }

  .start-btn:hover {
    background: #5a7fb5;
  }
</style>
