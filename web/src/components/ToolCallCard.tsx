import { useState } from 'react';

interface Props {
  name: string;
  args: Record<string, unknown>;
  result?: string;
}

export default function ToolCallCard({ name, args, result }: Props) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="tool-card">
      <button className="tool-header" onClick={() => setExpanded(!expanded)}>
        <span className="tool-icon">&#9889;</span>
        <span className="tool-name">{name}</span>
        <span className="tool-toggle">{expanded ? '\u25BC' : '\u25B6'}</span>
      </button>

      {expanded && (
        <div className="tool-body">
          <div className="tool-section">
            <div className="tool-label">Arguments</div>
            <pre className="tool-pre">{JSON.stringify(args, null, 2)}</pre>
          </div>
          {result !== undefined && (
            <div className="tool-section">
              <div className="tool-label">Result</div>
              <pre className="tool-pre">{result}</pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
