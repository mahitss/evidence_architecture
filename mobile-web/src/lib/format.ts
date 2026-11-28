import type { GenuineGapEntry } from './schemas';

export const CATEGORY_LABELS: Record<string, string> = {
  financial_crime: 'Financial Crimes',
  historical_public_record: 'Historical Records',
};

export function formatCategory(category: string): string {
  const known = CATEGORY_LABELS[category];
  if (known) return known;
  return category
    .replace(/[_-]+/g, ' ')
    .replace(/\b\w/g, (ch) => ch.toUpperCase());
}

export function formatArchivalDate(isoString: string | null | undefined): string {
  if (!isoString) return 'Unpublished';
  const parsed = new Date(isoString);
  if (Number.isNaN(parsed.getTime())) {
    return isoString;
  }
  return parsed.toISOString().slice(0, 10);
}

export function formatBytes(bytes: number | null | undefined): string {
  if (!bytes || bytes <= 0) return '—';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
}

export interface HighlightSegment {
  text: string;
  highlighted: boolean;
}

/**
 * Safely tokenizes `<mark>...</mark>` snippets returned by Meilisearch without
 * using dangerouslySetInnerHTML, preventing XSS from arbitrary extracted text.
 */
export function parseHighlightSegments(snippet: string): HighlightSegment[] {
  if (!snippet) return [];
  const regex = /<mark>(.*?)<\/mark>/gi;
  const segments: HighlightSegment[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null = regex.exec(snippet);

  while (match !== null) {
    if (match.index > lastIndex) {
      segments.push({
        text: snippet.slice(lastIndex, match.index),
        highlighted: false,
      });
    }
    const inner = match[1] ?? '';
    if (inner.length > 0) {
      segments.push({ text: inner, highlighted: true });
    }
    lastIndex = match.index + match[0].length;
    match = regex.exec(snippet);
  }

  if (lastIndex < snippet.length) {
    segments.push({
      text: snippet.slice(lastIndex),
      highlighted: false,
    });
  }

  return segments;
}

export function extractGenuineGapItems(
  entries: GenuineGapEntry[] | undefined,
  fallbackNote?: string,
): string[] {
  const results: string[] = [];
  if (entries && entries.length > 0) {
    for (const entry of entries) {
      if (typeof entry === 'string') {
        const trimmed = entry.trim();
        if (trimmed) results.push(trimmed);
      } else {
        const claim = entry.claim?.trim();
        const why = entry.why_unverified?.trim();
        const question = entry.question?.trim();
        const desc = entry.description?.trim();
        if (claim && why) {
          results.push(`${claim} — ${why}`);
        } else if (claim) {
          results.push(claim);
        } else if (question) {
          results.push(question);
        } else if (desc) {
          results.push(desc);
        }
      }
    }
  }
  if (results.length === 0 && fallbackNote && fallbackNote.trim().length > 0) {
    results.push(fallbackNote.trim());
  }
  return results;
}

export function isSafeHttpUrl(url: string | null | undefined): url is string {
  if (!url) return false;
  try {
    const parsed = new URL(url);
    return parsed.protocol === 'https:' || parsed.protocol === 'http:';
  } catch {
    return false;
  }
}
