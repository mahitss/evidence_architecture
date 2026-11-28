import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { Badge, Button, Callout, Card, Input, Skeleton } from '../components/ui';
import { HighlightedText } from '../components/HighlightedText';
import { formatArchivalDate, formatCategory } from '../lib/format';
import { useSearch } from '../lib/queries';
import type { SearchHit } from '../lib/schemas';

const SEARCH_CATEGORIES = ['financial_crime', 'historical_public_record'];

export function SearchResults() {
  const [searchParams, setSearchParams] = useSearchParams();
  const initialQuery = searchParams.get('q') ?? '';
  const initialCategory = searchParams.get('category') ?? '';

  const [queryInput, setQueryInput] = useState(initialQuery);
  const [category, setCategory] = useState(initialCategory);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [accumulated, setAccumulated] = useState<SearchHit[]>([]);

  useEffect(() => {
    const nextParams = new URLSearchParams();
    if (queryInput.trim()) nextParams.set('q', queryInput.trim());
    if (category) nextParams.set('category', category);
    setSearchParams(nextParams, { replace: true });
    setCursor(undefined);
    setAccumulated([]);
  }, [queryInput, category, setSearchParams]);

  const { data, isLoading, isFetching, isError, error, refetch } = useSearch(
    queryInput,
    category || undefined,
    cursor,
    300,
  );

  useEffect(() => {
    if (!data) return;
    if (!cursor) {
      setAccumulated(data.data);
    } else {
      setAccumulated((prev) => {
        const seen = new Set(prev.map((item) => item.id));
        const fresh = data.data.filter((item) => !seen.has(item.id));
        return [...prev, ...fresh];
      });
    }
  }, [data, cursor]);

  const trimmedQuery = queryInput.trim();
  const hits = cursor ? accumulated : (data?.data ?? accumulated);
  const queryTooShort = trimmedQuery.length < 2;

  return (
    <div className="space-y-6">
      <header className="space-y-2 pb-4 border-b border-[var(--surface-border-subtle)]">
        <h1 className="m-0 text-3xl font-bold text-[var(--ink)]">Search Archive</h1>
        <p className="m-0 text-sm text-[var(--ink-secondary)]">
          Full-text search across published case briefings and extracted PDF / OCR exhibit text.
        </p>
      </header>

      <section aria-label="Search controls" className="space-y-4">
        <Input
          type="search"
          autoFocus
          value={queryInput}
          onChange={(event) => setQueryInput(event.target.value)}
          placeholder="Type at least 2 characters (e.g. clearinghouse, SEC, indictment)..."
          label="Search query"
        />

        <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Filter search by category">
          <button
            type="button"
            aria-pressed={category === ''}
            onClick={() => setCategory('')}
            className={`min-h-[44px] px-3.5 py-2 text-sm font-medium border rounded-[var(--radius-sm)] cursor-pointer ${
              category === ''
                ? 'bg-[var(--surface-dark)] text-[var(--ink-on-dark)] border-[var(--surface-dark)]'
                : 'bg-[var(--surface-light)] text-[var(--ink)] border-[var(--surface-border-strong)] hover:bg-[var(--paper)]'
            }`}
          >
            All
          </button>
          {SEARCH_CATEGORIES.map((cat) => {
            const active = category === cat;
            return (
              <button
                key={cat}
                type="button"
                aria-pressed={active}
                onClick={() => setCategory(cat)}
                className={`min-h-[44px] px-3.5 py-2 text-sm font-medium border rounded-[var(--radius-sm)] cursor-pointer ${
                  active
                    ? 'bg-[var(--surface-dark)] text-[var(--ink-on-dark)] border-[var(--surface-dark)]'
                    : 'bg-[var(--surface-light)] text-[var(--ink)] border-[var(--surface-border-strong)] hover:bg-[var(--paper)]'
                }`}
              >
                {formatCategory(cat)}
              </button>
            );
          })}
        </div>
      </section>

      {queryTooShort && (
        <Card tone="light" className="p-6 text-sm text-[var(--ink-secondary)]">
          Enter at least 2 characters to search published dossiers and indexed exhibit transcripts.
        </Card>
      )}

      {!queryTooShort && isError && (
        <Callout variant="danger" title="Search unavailable">
          <p className="m-0 mb-3">
            {error instanceof Error ? error.message : 'Unable to execute archive search right now.'}
          </p>
          <Button variant="secondary" size="sm" onClick={() => void refetch()}>
            Retry search
          </Button>
        </Callout>
      )}

      {!queryTooShort && isLoading && hits.length === 0 && (
        <div aria-busy="true" aria-label="Searching archive" className="space-y-3">
          {Array.from({ length: 3 }).map((_, idx) => (
            <Card key={idx} className="p-5 space-y-2.5">
              <Skeleton className="h-4 w-28" />
              <Skeleton className="h-6 w-2/3" />
              <Skeleton className="h-4 w-full" />
            </Card>
          ))}
        </div>
      )}

      {!queryTooShort && !isLoading && !isFetching && !isError && hits.length === 0 && (
        <Card tone="light" className="p-8 text-center space-y-2">
          <p className="m-0 font-[var(--font-serif)] text-xl font-semibold text-[var(--ink)]">
            No results for &lsquo;{trimmedQuery}&rsquo;
          </p>
          <p className="m-0 text-sm text-[var(--ink-secondary)]">
            Try a broader term, remove a category filter, or browse all published case files.
          </p>
        </Card>
      )}

      {!queryTooShort && hits.length > 0 && (
        <section aria-label="Search results" className="space-y-4">
          <p className="m-0 text-xs font-[var(--font-mono)] text-[var(--ink-secondary)]">
            Showing {hits.length} matching {hits.length === 1 ? 'case' : 'cases'}
          </p>
          <ul className="list-none m-0 p-0 space-y-3">
            {hits.map((hit) => (
              <li key={hit.id}>
                <Link
                  to={`/case/${encodeURIComponent(hit.slug)}`}
                  className="block no-underline text-inherit"
                >
                  <Card interactive className="p-5 space-y-2.5">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <Badge variant="category">{formatCategory(hit.category)}</Badge>
                      <div className="flex items-center gap-3 text-xs text-[var(--ink-on-paper-muted)]">
                        <span>
                          {hit.evidence_count}{' '}
                          {hit.evidence_count === 1 ? 'exhibit' : 'exhibits'}
                        </span>
                        <time className="font-[var(--font-mono)]">
                          {formatArchivalDate(hit.published_at)}
                        </time>
                      </div>
                    </div>
                    <h2 className="m-0 text-lg font-bold text-[var(--ink)]">{hit.title}</h2>
                    {hit.highlight ? (
                      <p className="m-0 text-sm text-[var(--ink-secondary)] leading-relaxed">
                        <HighlightedText text={hit.highlight} />
                      </p>
                    ) : (
                      <p className="m-0 text-sm text-[var(--ink-secondary)] leading-relaxed">
                        {hit.summary}
                      </p>
                    )}
                  </Card>
                </Link>
              </li>
            ))}
          </ul>

          {data?.has_more && data.next_cursor && (
            <div className="flex justify-center pt-2">
              <Button
                variant="secondary"
                disabled={isFetching}
                onClick={() => {
                  if (data.next_cursor) {
                    setCursor(data.next_cursor);
                  }
                }}
              >
                {isFetching ? 'Loading more...' : 'Load more results'}
              </Button>
            </div>
          )}
        </section>
      )}
    </div>
  );
}
