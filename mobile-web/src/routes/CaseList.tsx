import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Badge, Button, Callout, Card, Input, Skeleton } from '../components/ui';
import { formatArchivalDate, formatCategory } from '../lib/format';
import { useCases } from '../lib/queries';
import type { CaseSummary } from '../lib/schemas';

const BASE_CATEGORIES = ['financial_crime', 'historical_public_record'];

export function CaseList() {
  const navigate = useNavigate();
  const [searchDraft, setSearchDraft] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('');
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [accumulated, setAccumulated] = useState<CaseSummary[]>([]);

  const { data, isLoading, isFetching, isError, error, refetch } = useCases(
    selectedCategory || undefined,
    cursor,
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

  const categories = useMemo(() => {
    const set = new Set<string>(BASE_CATEGORIES);
    for (const item of accumulated) {
      if (item.category) set.add(item.category);
    }
    return Array.from(set);
  }, [accumulated]);

  const handleCategoryChange = (nextCategory: string) => {
    if (nextCategory === selectedCategory) return;
    setSelectedCategory(nextCategory);
    setCursor(undefined);
    setAccumulated([]);
  };

  const handleSearchSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = searchDraft.trim();
    if (trimmed.length > 0) {
      navigate(`/search?q=${encodeURIComponent(trimmed)}`);
    } else {
      navigate('/search');
    }
  };

  const casesToRender = cursor ? accumulated : (data?.data ?? accumulated);
  const showInitialSkeleton = isLoading && casesToRender.length === 0;

  return (
    <div className="space-y-6">
      <header className="space-y-2 pb-4 border-b border-[var(--surface-border-subtle)]">
        <h1 className="text-3xl sm:text-4xl font-bold leading-tight m-0 text-[var(--ink)]">
          Published Case Files
        </h1>
        <p className="m-0 text-sm sm:text-base text-[var(--ink-secondary)] max-w-2xl">
          Public-record dossiers reviewed for primary-source provenance, explicit licensing, and
          documented evidentiary gaps.
        </p>
      </header>

      {/* Search + Category Filters */}
      <section aria-label="Filter and search cases" className="space-y-4">
        <form onSubmit={handleSearchSubmit} role="search" className="flex flex-col sm:flex-row gap-2">
          <div className="flex-1">
            <Input
              type="search"
              value={searchDraft}
              onChange={(event) => setSearchDraft(event.target.value)}
              placeholder="Search dossiers, docket numbers, or extracted exhibit text..."
              aria-label="Search published case files"
            />
          </div>
          <Button type="submit" variant="primary" className="sm:w-auto w-full shrink-0">
            Search Archive
          </Button>
        </form>

        <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Filter by category">
          <button
            type="button"
            aria-pressed={selectedCategory === ''}
            onClick={() => handleCategoryChange('')}
            className={`min-h-[44px] px-3.5 py-2 text-sm font-medium border rounded-[var(--radius-sm)] transition-colors duration-[var(--duration-fast)] cursor-pointer ${
              selectedCategory === ''
                ? 'bg-[var(--surface-dark)] text-[var(--ink-on-dark)] border-[var(--surface-dark)]'
                : 'bg-[var(--surface-light)] text-[var(--ink)] border-[var(--surface-border-strong)] hover:bg-[var(--paper)]'
            }`}
          >
            All
          </button>
          {categories.map((cat) => {
            const active = selectedCategory === cat;
            return (
              <button
                key={cat}
                type="button"
                aria-pressed={active}
                onClick={() => handleCategoryChange(cat)}
                className={`min-h-[44px] px-3.5 py-2 text-sm font-medium border rounded-[var(--radius-sm)] transition-colors duration-[var(--duration-fast)] cursor-pointer ${
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

      {/* Error State */}
      {isError && (
        <Callout variant="danger" title="Unable to load case files">
          <p className="m-0 mb-3">
            {error instanceof Error
              ? error.message
              : 'The archive server could not be reached right now.'}
          </p>
          <Button variant="secondary" size="sm" onClick={() => void refetch()}>
            Retry
          </Button>
        </Callout>
      )}

      {/* Loading Skeleton Grid */}
      {showInitialSkeleton && (
        <div
          aria-busy="true"
          aria-label="Loading published cases"
          className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4"
        >
          {Array.from({ length: 6 }).map((_, idx) => (
            <Card key={idx} className="p-5 space-y-3">
              <Skeleton className="h-5 w-28" />
              <Skeleton className="h-6 w-5/6" />
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-4/5" />
              <div className="pt-3 flex items-center justify-between border-t border-[var(--surface-border-subtle)]">
                <Skeleton className="h-4 w-20" />
                <Skeleton className="h-4 w-24" />
              </div>
            </Card>
          ))}
        </div>
      )}

      {/* Empty State */}
      {!showInitialSkeleton && !isError && casesToRender.length === 0 && (
        <Card tone="light" className="p-8 text-center space-y-2">
          <p className="m-0 font-[var(--font-serif)] text-xl font-semibold text-[var(--ink)]">
            No cases published yet.
          </p>
          <p className="m-0 text-sm text-[var(--ink-secondary)]">
            {selectedCategory
              ? 'No published dossiers match this category filter.'
              : 'Published public-record dossiers will appear here once legal review is complete.'}
          </p>
          {selectedCategory && (
            <div className="pt-2">
              <Button variant="secondary" size="sm" onClick={() => handleCategoryChange('')}>
                Show all categories
              </Button>
            </div>
          )}
        </Card>
      )}

      {/* Responsive 1 / 2 / 3 Column Case Card Grid */}
      {casesToRender.length > 0 && (
        <section aria-label="Case file dossiers" className="space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {casesToRender.map((item) => (
              <Link
                key={item.id}
                to={`/case/${encodeURIComponent(item.slug)}`}
                className="group block no-underline text-inherit focus-visible:outline-offset-2"
              >
                <Card interactive className="h-full p-5 flex flex-col justify-between gap-4">
                  <div className="space-y-2.5">
                    <div className="flex items-center justify-between gap-2">
                      <Badge variant="category">{formatCategory(item.category)}</Badge>
                      {item.jurisdiction && (
                        <span className="text-xs font-[var(--font-mono)] text-[var(--ink-on-paper-muted)]">
                          {item.jurisdiction}
                        </span>
                      )}
                    </div>
                    <h2 className="m-0 text-lg font-bold leading-snug line-clamp-2 text-[var(--ink)] group-hover:text-[var(--accent-text)] transition-colors duration-[var(--duration-fast)]">
                      {item.title}
                    </h2>
                    <p className="m-0 text-sm leading-relaxed text-[var(--ink-secondary)] line-clamp-3">
                      {item.summary}
                    </p>
                  </div>

                  <div className="pt-3 border-t border-[var(--surface-border-subtle)] flex items-center justify-between gap-2 text-xs text-[var(--ink-on-paper-muted)]">
                    <span>
                      {item.evidence_count}{' '}
                      {item.evidence_count === 1 ? 'evidence item' : 'evidence items'}
                    </span>
                    <time
                      dateTime={item.published_at ?? undefined}
                      className="font-[var(--font-mono)]"
                    >
                      {formatArchivalDate(item.published_at)}
                    </time>
                  </div>
                </Card>
              </Link>
            ))}
          </div>

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
                {isFetching ? 'Loading more cases...' : 'Load more'}
              </Button>
            </div>
          )}
        </section>
      )}
    </div>
  );
}
