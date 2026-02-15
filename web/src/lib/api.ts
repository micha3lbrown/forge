const BASE = '/api';

export interface Session {
  id: string;
  title: string;
  status: string;
  provider: string;
  model: string;
  profile: string;
  created_at: string;
  updated_at: string;
}

export interface Message {
  role: string;
  content?: string;
  tool_calls?: ToolCall[];
  tool_call_id?: string;
}

export interface ToolCall {
  id: string;
  name: string;
  arguments: Record<string, any>;
}

export interface Provider {
  name: string;
  models: Record<string, string>;
  is_ollama: boolean;
}

export interface ModelInfo {
  name: string;
  size: number;
  modified_at: string;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(BASE + path, init);
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(body.error || resp.statusText);
  }
  if (resp.status === 204) return undefined as T;
  return resp.json();
}

export function listSessions(status?: string): Promise<Session[]> {
  const params = status ? `?status=${status}` : '';
  return request(`/sessions${params}`);
}

export function createSession(opts: { provider?: string; model?: string; profile?: string }): Promise<Session> {
  return request('/sessions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(opts),
  });
}

export function getSession(id: string): Promise<Session> {
  return request(`/sessions/${id}`);
}

export function deleteSession(id: string): Promise<void> {
  return request(`/sessions/${id}`, { method: 'DELETE' });
}

export function getMessages(sessionId: string): Promise<Message[]> {
  return request(`/sessions/${sessionId}/messages`);
}

export function sendMessage(sessionId: string, content: string): Promise<{ content: string }> {
  return request(`/sessions/${sessionId}/messages`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content }),
  });
}

export function listProviders(): Promise<Provider[]> {
  return request('/providers');
}

export function listModels(provider: string): Promise<ModelInfo[]> {
  return request(`/models/${provider}`);
}
