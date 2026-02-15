<script lang="ts">
  import { sessions, activeSessionId } from '../lib/stores';
  import { createSession, deleteSession, listSessions } from '../lib/api';

  export let onSelectSession: (id: string) => void;

  async function handleNewChat() {
    try {
      const sess = await createSession({});
      $sessions = [sess, ...$sessions];
      onSelectSession(sess.id);
    } catch (err) {
      console.error('Failed to create session:', err);
    }
  }

  async function handleDelete(e: Event, id: string) {
    e.stopPropagation();
    try {
      await deleteSession(id);
      $sessions = $sessions.filter(s => s.id !== id);
      if ($activeSessionId === id) {
        $activeSessionId = null;
      }
    } catch (err) {
      console.error('Failed to delete session:', err);
    }
  }

  async function refresh() {
    try {
      $sessions = await listSessions();
    } catch (err) {
      console.error('Failed to list sessions:', err);
    }
  }

  // Load sessions on mount
  refresh();
</script>

<aside class="sidebar">
  <div class="sidebar-header">
    <h2>Forge</h2>
    <button class="new-chat" on:click={handleNewChat}>+ New Chat</button>
  </div>

  <div class="session-list">
    {#each $sessions as session (session.id)}
      <div
        class="session-item"
        class:active={$activeSessionId === session.id}
        on:click={() => onSelectSession(session.id)}
        on:keydown={(e) => e.key === 'Enter' && onSelectSession(session.id)}
        role="button"
        tabindex="0"
      >
        <span class="session-title">{session.title || 'New Chat'}</span>
        <span class="session-meta">{session.provider}/{session.model}</span>
        <button class="delete-btn" on:click={(e) => handleDelete(e, session.id)} title="Delete">x</button>
      </div>
    {/each}

    {#if $sessions.length === 0}
      <p class="empty">No sessions yet</p>
    {/if}
  </div>
</aside>

<style>
  .sidebar {
    width: 280px;
    background: #1a1a2e;
    border-right: 1px solid #2a2a4a;
    display: flex;
    flex-direction: column;
    height: 100vh;
  }

  .sidebar-header {
    padding: 1rem;
    border-bottom: 1px solid #2a2a4a;
  }

  h2 {
    margin: 0 0 0.75rem 0;
    color: #e0e0e0;
    font-size: 1.25rem;
  }

  .new-chat {
    width: 100%;
    padding: 0.5rem;
    background: #3a3a5c;
    color: #e0e0e0;
    border: 1px solid #4a4a6a;
    border-radius: 6px;
    cursor: pointer;
    font-size: 0.9rem;
  }

  .new-chat:hover {
    background: #4a4a6a;
  }

  .session-list {
    flex: 1;
    overflow-y: auto;
    padding: 0.5rem;
  }

  .session-item {
    padding: 0.6rem 0.75rem;
    border-radius: 6px;
    cursor: pointer;
    margin-bottom: 2px;
    position: relative;
    color: #c0c0c0;
  }

  .session-item:hover {
    background: #2a2a4a;
  }

  .session-item.active {
    background: #3a3a5c;
    color: #fff;
  }

  .session-title {
    display: block;
    font-size: 0.85rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    padding-right: 1.5rem;
  }

  .session-meta {
    display: block;
    font-size: 0.7rem;
    color: #808090;
    margin-top: 2px;
  }

  .delete-btn {
    position: absolute;
    right: 0.5rem;
    top: 0.5rem;
    background: none;
    border: none;
    color: #808090;
    cursor: pointer;
    font-size: 0.8rem;
    padding: 2px 6px;
    border-radius: 3px;
    opacity: 0;
  }

  .session-item:hover .delete-btn {
    opacity: 1;
  }

  .delete-btn:hover {
    background: #c0392b;
    color: white;
  }

  .empty {
    color: #606070;
    text-align: center;
    padding: 2rem 1rem;
    font-size: 0.85rem;
  }
</style>
