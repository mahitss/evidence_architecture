import { parseHighlightSegments } from '../lib/format';

export interface HighlightedTextProps {
  text: string;
  className?: string;
}

/**
 * Renders Meilisearch `<mark>...</mark>` highlights safely as React nodes without raw HTML injection.
 */
export function HighlightedText({ text, className = '' }: HighlightedTextProps) {
  const segments = parseHighlightSegments(text);
  if (segments.length === 0) {
    return null;
  }

  return (
    <span className={className}>
      {segments.map((segment, index) =>
        segment.highlighted ? (
          <mark key={index} className="search-mark">
            {segment.text}
          </mark>
        ) : (
          <span key={index}>{segment.text}</span>
        ),
      )}
    </span>
  );
}
