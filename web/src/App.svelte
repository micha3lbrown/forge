<script lang="ts">
  import Sidebar from './components/Sidebar.svelte';
  import ChatView from './components/ChatView.svelte';
  import { activeSessionId } from './lib/stores';

  function handleSelectSession(id: string) {
    $activeSessionId = id;
  }
</script>

<main class="app">
  <Sidebar onSelectSession={handleSelectSession} />
  <ChatView />
</main>

<style>
  :global(*) {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }

  :global(body) {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: #0f0f23;
    color: #e0e0e0;
    overflow: hidden;
  }

  .app {
    display: flex;
    height: 100vh;
    width: 100vw;
  }
</style>
