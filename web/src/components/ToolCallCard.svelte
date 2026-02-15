<script lang="ts">
  export let name: string;
  export let args: Record<string, any> = {};
  export let result: string | undefined = undefined;

  let expanded = false;
</script>

<div class="tool-card">
  <button class="tool-header" on:click={() => expanded = !expanded}>
    <span class="tool-icon">&#9889;</span>
    <span class="tool-name">{name}</span>
    <span class="tool-toggle">{expanded ? '&#9660;' : '&#9654;'}</span>
  </button>

  {#if expanded}
    <div class="tool-body">
      <div class="tool-section">
        <div class="tool-label">Arguments</div>
        <pre class="tool-pre">{JSON.stringify(args, null, 2)}</pre>
      </div>
      {#if result !== undefined}
        <div class="tool-section">
          <div class="tool-label">Result</div>
          <pre class="tool-pre">{result}</pre>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .tool-card {
    background: #1a1a2e;
    border: 1px solid #2a2a4a;
    border-radius: 6px;
    margin: 0.5rem 0;
    overflow: hidden;
  }

  .tool-header {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0.75rem;
    background: none;
    border: none;
    color: #e0b050;
    cursor: pointer;
    font-size: 0.85rem;
    text-align: left;
  }

  .tool-header:hover {
    background: #2a2a4a;
  }

  .tool-icon {
    font-size: 0.9rem;
  }

  .tool-name {
    flex: 1;
    font-family: 'SF Mono', 'Fira Code', monospace;
  }

  .tool-toggle {
    font-size: 0.7rem;
    color: #808090;
  }

  .tool-body {
    padding: 0 0.75rem 0.75rem;
  }

  .tool-section {
    margin-top: 0.5rem;
  }

  .tool-label {
    font-size: 0.75rem;
    color: #808090;
    margin-bottom: 0.25rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .tool-pre {
    background: #0d0d1a;
    padding: 0.5rem;
    border-radius: 4px;
    font-size: 0.8rem;
    overflow-x: auto;
    margin: 0;
    white-space: pre-wrap;
    word-break: break-word;
    max-height: 300px;
    overflow-y: auto;
  }
</style>
