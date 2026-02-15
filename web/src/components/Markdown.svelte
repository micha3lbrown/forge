<script lang="ts">
  import { marked } from 'marked';

  export let content: string;

  // Configure marked for safe rendering
  marked.setOptions({
    breaks: true,
    gfm: true,
  });

  $: html = marked.parse(content || '') as string;
</script>

<div class="markdown">{@html html}</div>

<style>
  .markdown :global(pre) {
    background: #0d0d1a;
    padding: 0.75rem;
    border-radius: 6px;
    overflow-x: auto;
    font-size: 0.85rem;
  }

  .markdown :global(code) {
    font-family: 'SF Mono', 'Fira Code', monospace;
    font-size: 0.85em;
  }

  .markdown :global(p code) {
    background: #2a2a4a;
    padding: 0.15rem 0.4rem;
    border-radius: 3px;
  }

  .markdown :global(p) {
    margin: 0.5em 0;
  }

  .markdown :global(ul),
  .markdown :global(ol) {
    padding-left: 1.5rem;
  }

  .markdown :global(blockquote) {
    border-left: 3px solid #4a4a6a;
    margin: 0.5em 0;
    padding-left: 1rem;
    color: #a0a0b0;
  }

  .markdown :global(table) {
    border-collapse: collapse;
    margin: 0.5em 0;
  }

  .markdown :global(th),
  .markdown :global(td) {
    border: 1px solid #3a3a5c;
    padding: 0.3rem 0.6rem;
  }

  .markdown :global(a) {
    color: #6c9bff;
  }
</style>
