import { useMemo } from 'react';
import { marked } from 'marked';

marked.setOptions({ breaks: true, gfm: true });

export default function Markdown({ content }: { content: string }) {
  const html = useMemo(() => marked.parse(content || '') as string, [content]);

  return (
    <div
      className="markdown"
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}
