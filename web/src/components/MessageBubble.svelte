<script lang="ts">
  import Markdown from './Markdown.svelte';
  import ToolCallCard from './ToolCallCard.svelte';
  import type { Message } from '../lib/api';

  export let message: Message;
  export let toolResults: Map<string, string> = new Map();
</script>

{#if message.role === 'user'}
  <div class="bubble user">
    <div class="role">You</div>
    <div class="content">{message.content}</div>
  </div>
{:else if message.role === 'assistant'}
  <div class="bubble assistant">
    <div class="role">Forge</div>
    {#if message.content}
      <Markdown content={message.content} />
    {/if}
    {#if message.tool_calls}
      {#each message.tool_calls as tc}
        <ToolCallCard
          name={tc.name}
          args={tc.arguments}
          result={toolResults.get(tc.id)}
        />
      {/each}
    {/if}
  </div>
{:else if message.role === 'tool'}
  <!-- Tool results are rendered inline with their tool call -->
{:else if message.role === 'system'}
  <!-- System messages hidden in UI -->
{/if}

<style>
  .bubble {
    padding: 0.75rem 1rem;
    border-radius: 8px;
    margin-bottom: 0.75rem;
    max-width: 90%;
  }

  .bubble.user {
    background: #2a4a6a;
    align-self: flex-end;
    margin-left: auto;
  }

  .bubble.assistant {
    background: #1e1e36;
    border: 1px solid #2a2a4a;
  }

  .role {
    font-size: 0.7rem;
    color: #808090;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-bottom: 0.3rem;
  }

  .content {
    white-space: pre-wrap;
    word-break: break-word;
    line-height: 1.5;
  }
</style>
