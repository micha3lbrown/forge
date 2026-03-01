import { create } from 'zustand';
import type { Session, Message } from './api';
import type { FallbackOption } from './ws';

export interface StreamingToolCall {
  name: string;
  args: Record<string, unknown>;
  result?: string;
}

interface ForgeState {
  sessions: Session[];
  activeSessionId: string | null;
  messages: Message[];
  isStreaming: boolean;
  streamingText: string;
  streamingToolCalls: StreamingToolCall[];
  errorMessage: string;
  showModelPicker: boolean;
  fallbackOptions: FallbackOption[];
  systemMessage: string;

  setSessions: (sessions: Session[]) => void;
  setActiveSessionId: (id: string | null) => void;
  setMessages: (messages: Message[]) => void;
  addUserMessage: (content: string) => void;
  setIsStreaming: (v: boolean) => void;
  addStreamDelta: (delta: string) => void;
  addStreamToolCall: (tc: StreamingToolCall) => void;
  updateToolCallResult: (name: string, result: string) => void;
  resetStreaming: () => void;
  setError: (msg: string, fallback?: FallbackOption[]) => void;
  clearError: () => void;
  setSystemMessage: (msg: string) => void;
  updateSessionInList: (session: Session) => void;
  removeSession: (id: string) => void;
  prependSession: (session: Session) => void;
}

export const useStore = create<ForgeState>((set) => ({
  sessions: [],
  activeSessionId: null,
  messages: [],
  isStreaming: false,
  streamingText: '',
  streamingToolCalls: [],
  errorMessage: '',
  showModelPicker: false,
  fallbackOptions: [],
  systemMessage: '',

  setSessions: (sessions) => set({ sessions }),
  setActiveSessionId: (id) => set({ activeSessionId: id }),
  setMessages: (messages) => set({ messages }),
  addUserMessage: (content) =>
    set((s) => ({ messages: [...s.messages, { role: 'user', content }] })),
  setIsStreaming: (v) => set({ isStreaming: v }),
  addStreamDelta: (delta) =>
    set((s) => ({ streamingText: s.streamingText + delta })),
  addStreamToolCall: (tc) =>
    set((s) => ({ streamingToolCalls: [...s.streamingToolCalls, tc] })),
  updateToolCallResult: (name, result) =>
    set((s) => {
      const calls = [...s.streamingToolCalls];
      const idx = calls.findLastIndex((tc) => tc.name === name && !tc.result);
      if (idx >= 0) {
        calls[idx] = { ...calls[idx], result };
      }
      return { streamingToolCalls: calls };
    }),
  resetStreaming: () =>
    set({ isStreaming: false, streamingText: '', streamingToolCalls: [] }),
  setError: (msg, fallback) =>
    set({
      errorMessage: msg,
      showModelPicker: true,
      fallbackOptions: fallback ?? [],
    }),
  clearError: () =>
    set({ errorMessage: '', showModelPicker: false, fallbackOptions: [] }),
  setSystemMessage: (msg) => set({ systemMessage: msg }),
  updateSessionInList: (session) =>
    set((s) => ({
      sessions: s.sessions.map((x) => (x.id === session.id ? session : x)),
    })),
  removeSession: (id) =>
    set((s) => ({
      sessions: s.sessions.filter((x) => x.id !== id),
      activeSessionId: s.activeSessionId === id ? null : s.activeSessionId,
    })),
  prependSession: (session) =>
    set((s) => ({ sessions: [session, ...s.sessions] })),
}));

// Derived helper: get active session object
export function getActiveSession(): Session | null {
  const { sessions, activeSessionId } = useStore.getState();
  if (!activeSessionId) return null;
  return sessions.find((s) => s.id === activeSessionId) ?? null;
}
