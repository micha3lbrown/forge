import { writable, derived } from 'svelte/store';
import type { Session, Message } from './api';

// Current session list
export const sessions = writable<Session[]>([]);

// Currently selected session ID
export const activeSessionId = writable<string | null>(null);

// Messages for the active session
export const messages = writable<Message[]>([]);

// Whether the agent is currently processing
export const isStreaming = writable(false);

// Streaming text accumulator for the current response
export const streamingText = writable('');

// Active tool calls during streaming
export interface StreamingToolCall {
  name: string;
  args: Record<string, any>;
  result?: string;
}
export const streamingToolCalls = writable<StreamingToolCall[]>([]);

// Derived: get the active session object
export const activeSession = derived(
  [sessions, activeSessionId],
  ([$sessions, $activeSessionId]) => {
    if (!$activeSessionId) return null;
    return $sessions.find(s => s.id === $activeSessionId) ?? null;
  }
);
