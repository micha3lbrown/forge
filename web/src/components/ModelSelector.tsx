import { useState, useEffect } from 'react';
import { listProviders, listModels } from '../lib/api';
import type { Provider, ModelInfo } from '../lib/api';

interface Props {
  selectedProvider: string;
  selectedModel: string;
  onChange: (provider: string, model: string) => void;
}

export default function ModelSelector({ selectedProvider, selectedModel, onChange }: Props) {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [provider, setProvider] = useState(selectedProvider);
  const [model, setModel] = useState(selectedModel);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState('');

  useEffect(() => {
    listProviders()
      .then((p) => {
        setProviders(p);
        if (!provider && p.length > 0) {
          setProvider(p[0].name);
        }
      })
      .catch(console.error);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!provider) return;
    setLoading(true);
    setLoadError('');
    listModels(provider)
      .then((m) => {
        setModels(m);
        if (m.length === 0) {
          setLoadError('No models found');
        } else if (!model || !m.some((x) => x.name === model)) {
          const prov = providers.find((p) => p.name === provider);
          setModel(prov?.models?.default || m[0].name);
        }
      })
      .catch(() => {
        setLoadError('Could not load models');
        setModels([]);
      })
      .finally(() => setLoading(false));
  }, [provider]); // eslint-disable-line react-hooks/exhaustive-deps

  function handleProviderChange(e: React.ChangeEvent<HTMLSelectElement>) {
    setProvider(e.target.value);
    setModel('');
  }

  return (
    <div className="model-selector">
      <select value={provider} onChange={handleProviderChange}>
        {providers.map((p) => (
          <option key={p.name} value={p.name}>{p.name}</option>
        ))}
      </select>

      <select
        value={model}
        onChange={(e) => setModel(e.target.value)}
        disabled={loading || models.length === 0}
      >
        {loading ? (
          <option>Loading...</option>
        ) : loadError ? (
          <option>{loadError}</option>
        ) : (
          models.map((m) => (
            <option key={m.name} value={m.name}>{m.name}</option>
          ))
        )}
      </select>

      <button
        onClick={() => onChange(provider, model)}
        disabled={loading || models.length === 0}
      >
        Apply
      </button>
    </div>
  );
}
