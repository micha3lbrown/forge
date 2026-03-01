import { useStore } from '../lib/store';
import { createSession, deleteSession, listSessions } from '../lib/api';
import { useEffect } from 'react';

export default function Sidebar() {
  const sessions = useStore((s) => s.sessions);
  const activeSessionId = useStore((s) => s.activeSessionId);
  const setSessions = useStore((s) => s.setSessions);
  const setActiveSessionId = useStore((s) => s.setActiveSessionId);
  const removeSession = useStore((s) => s.removeSession);
  const prependSession = useStore((s) => s.prependSession);

  useEffect(() => {
    listSessions().then(setSessions).catch(console.error);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  async function handleNewChat() {
    try {
      const sess = await createSession({});
      prependSession(sess);
      setActiveSessionId(sess.id);
    } catch (err) {
      console.error('Failed to create session:', err);
    }
  }

  async function handleDelete(e: React.MouseEvent, id: string) {
    e.stopPropagation();
    try {
      await deleteSession(id);
      removeSession(id);
    } catch (err) {
      console.error('Failed to delete session:', err);
    }
  }

  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <h2>Forge</h2>
        <button className="new-chat" onClick={handleNewChat}>+ New Chat</button>
      </div>

      <div className="session-list">
        {sessions.map((session) => (
          <div
            key={session.id}
            className={`session-item ${activeSessionId === session.id ? 'active' : ''}`}
            onClick={() => setActiveSessionId(session.id)}
            onKeyDown={(e) => e.key === 'Enter' && setActiveSessionId(session.id)}
            role="button"
            tabIndex={0}
          >
            <span className="session-title">{session.title || 'New Chat'}</span>
            <span className="session-meta">{session.provider}/{session.model}</span>
            <button
              className="delete-btn"
              onClick={(e) => handleDelete(e, session.id)}
              title="Delete"
            >
              x
            </button>
          </div>
        ))}

        {sessions.length === 0 && (
          <p className="empty">No sessions yet</p>
        )}
      </div>
    </aside>
  );
}
