<script lang="ts">
  import { onMount } from 'svelte';
  import { listProviders, listModels } from '../lib/api';
  import type { Provider, ModelInfo } from '../lib/api';

  export let selectedProvider: string = '';
  export let selectedModel: string = '';
  export let onChange: (provider: string, model: string) => void = () => {};

  let providers: Provider[] = [];
  let models: ModelInfo[] = [];
  let loading = false;

  onMount(async () => {
    try {
      providers = await listProviders();
      if (!selectedProvider && providers.length > 0) {
        selectedProvider = providers[0].name;
        await loadModels();
      }
    } catch (err) {
      console.error('Failed to load providers:', err);
    }
  });

  async function loadModels() {
    if (!selectedProvider) return;
    loading = true;
    try {
      models = await listModels(selectedProvider);
      const provider = providers.find(p => p.name === selectedProvider);
      if (provider?.models?.default && !selectedModel) {
        selectedModel = provider.models.default;
      }
    } catch (err) {
      console.error('Failed to load models:', err);
      models = [];
    }
    loading = false;
  }

  function handleProviderChange() {
    selectedModel = '';
    loadModels();
  }

  function handleApply() {
    onChange(selectedProvider, selectedModel);
  }
</script>

<div class="model-selector">
  <select bind:value={selectedProvider} on:change={handleProviderChange}>
    {#each providers as p}
      <option value={p.name}>{p.name}</option>
    {/each}
  </select>

  <select bind:value={selectedModel} disabled={loading}>
    {#if loading}
      <option>Loading...</option>
    {:else}
      {#each models as m}
        <option value={m.name}>{m.name}</option>
      {/each}
    {/if}
  </select>

  <button on:click={handleApply}>Apply</button>
</div>

<style>
  .model-selector {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }

  select {
    background: #2a2a4a;
    color: #e0e0e0;
    border: 1px solid #3a3a5c;
    border-radius: 4px;
    padding: 0.3rem 0.5rem;
    font-size: 0.8rem;
  }

  button {
    background: #3a3a5c;
    color: #e0e0e0;
    border: 1px solid #4a4a6a;
    border-radius: 4px;
    padding: 0.3rem 0.75rem;
    cursor: pointer;
    font-size: 0.8rem;
  }

  button:hover {
    background: #4a4a6a;
  }
</style>
