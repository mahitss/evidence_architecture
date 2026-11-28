import { useEffect, useMemo } from 'react';
import { Link, useParams } from 'react-router-dom';
import {
  Badge,
  Button,
  Callout,
  Card,
  LicenseStatusBadge,
  Skeleton,
  VerificationTierBadge,
} from '../components/ui';
import { ShareButton } from '../components/ShareButton';
import { extractGenuineGapItems, formatArchivalDate, formatCategory } from '../lib/format';
import { recordCaseView } from '../lib/pwa';
import { useCase } from '../lib/queries';
import { NotFound } from './NotFound';

function EvidenceTypeIcon({ type }: { type: string }) {
  const lower = type.toLowerCase();
  if (lower.includes('image') || lower.includes('photo')) {
    return (
      <svg viewBox="0 0 20 20" fill="currentColor" className="w-5 h-5 shrink-0 text-[var(--accent-text)]" aria-hidden="true">
        <path
          fillRule="evenodd"
          d="M1 5.25A2.25 2.25 0 013.25 3h13.5A2.25 2.25 0 0119 5.25v9.5A2.25 2.25 0 0116.75 17H3.25A2.25 2.25 0 011 14.75v-9.5zm1.5 5.81v3.69c0 .414.336.75.75.75h13.5a.75.75 0 00.75-.75v-2.69l-2.22-2.219a.75.75 0 00-1.06 0l-1.91 1.909.47.47a.75.75 0 11-1.06 1.06L6.53 8.091a.75.75 0 00-1.06 0l-2.97 2.97zM12 7a1 1 0 11-2 0 1 1 0 012 0z"
          clipRule="evenodd"
        />
      </svg>
    );
  }
  return (
    <svg viewBox="0 0 20 20" fill="currentColor" className="w-5 h-5 shrink-0 text-[var(--accent-text)]" aria-hidden="true">
      <path
        fillRule="evenodd"
        d="M4.5 2A1.5 1.5 0 003 3.5v13A1.5 1.5 0 004.5 18h11a1.5 1.5 0 001.5-1.5V7.621a1.5 1.5 0 00-.44-1.06l-4.12-4.122A1.5 1.5 0 0011.378 2H4.5zm2.25 8.5a.75.75 0 000 1.5h6.5a.75.75 0 000-1.5h-6.5zm0 3a.75.75 0 000 1.5h6.5a.75.75 0 000-1.5h-6.5z"
        clipRule="evenodd"
      />
    </svg>
  );
}

export function CaseDetail() {
  const { slug } = useParams<{ slug: string }>();
  const { data: caseData, isLoading, isError, error, refetch } = useCase(slug);

  useEffect(() => {
    if (caseData?.slug) {
      recordCaseView(caseData.slug);
    }
  }, [caseData?.slug]);

  const timelineEvents = useMemo(() => {
    if (!caseData) return [];
    const list =
      caseData.timeline_events.length > 0 ? caseData.timeline_events : caseData.timeline;
    return [...list].sort((a, b) => {
      if (a.event_start_date !== b.event_start_date) {
        return a.event_start_date.localeCompare(b.event_start_date);
      }
      return a.sort_order - b.sort_order;
    });
  }, [caseData]);

  const gapItems = useMemo(() => {
    if (!caseData) return [];
    return extractGenuineGapItems(caseData.genuine_gaps, caseData.genuine_gaps_note);
  }, [caseData]);

  if (isLoading) {
    return (
      <div aria-busy="true" aria-label="Loading case dossier" className="space-y-6">
        <Skeleton className="h-4 w-48" />
        <Skeleton className="h-10 w-3/4" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-48 w-full" />
      </div>
    );
  }

  if (isError) {
    return (
      <Callout variant="danger" title="Unable to load case dossier">
        <p className="m-0 mb-3">
          {error instanceof Error ? error.message : 'An unexpected error occurred loading this case.'}
        </p>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" size="sm" onClick={() => void refetch()}>
            Retry
          </Button>
          <Link
            to="/"
            className="inline-flex items-center min-h-[38px] px-3 py-1.5 text-sm font-medium underline text-[var(--ink)]"
          >
            Back to Cases
          </Link>
        </div>
      </Callout>
    );
  }

  if (caseData === null || caseData === undefined) {
    return <NotFound message="The requested case dossier was not found or is not published." />;
  }

  const briefing = caseData.official_briefing || caseData.summary;
  const evidenceCount = caseData.evidence_items.length || caseData.evidence_count;

  return (
    <article className="space-y-8">
      {/* Breadcrumb */}
      <nav aria-label="Breadcrumb" className="text-xs text-[var(--ink-secondary)]">
        <ol className="flex flex-wrap items-center gap-1.5 list-none m-0 p-0">
          <li>
            <Link to="/" className="underline text-[var(--accent-text)] hover:text-[var(--ink)]">
              Cases
            </Link>
          </li>
          <li aria-hidden="true">/</li>
          <li aria-current="page" className="text-[var(--ink)] font-medium">
            {caseData.title}
          </li>
        </ol>
      </nav>

      {/* Case Header */}
      <header className="space-y-4 pb-6 border-b border-[var(--surface-border-subtle)]">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="category">{formatCategory(caseData.category)}</Badge>
            {caseData.jurisdiction && (
              <Badge variant="neutral" className="font-[var(--font-mono)]">
                {caseData.jurisdiction}
              </Badge>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <ShareButton title={caseData.title} text={briefing.slice(0, 140)} />
            <Link
              to={`/correction?case=${encodeURIComponent(caseData.slug)}`}
              className="inline-flex items-center justify-center min-h-[38px] px-3 py-1.5 text-sm font-medium border border-[var(--surface-border-strong)] rounded-[var(--radius-md)] bg-[var(--surface-light)] text-[var(--ink)] hover:bg-[var(--paper)] no-underline"
            >
              Report correction
            </Link>
          </div>
        </div>

        <h1 className="m-0 text-3xl sm:text-4xl font-bold leading-tight text-[var(--ink)]">
          {caseData.title}
        </h1>

        <div className="flex flex-wrap items-center gap-4 text-xs text-[var(--ink-secondary)]">
          <span>
            Published:{' '}
            <time
              dateTime={caseData.published_at ?? undefined}
              className="font-[var(--font-mono)] text-[var(--ink)]"
            >
              {formatArchivalDate(caseData.published_at)}
            </time>
          </span>
          <span aria-hidden="true">·</span>
          <span>
            <strong className="font-semibold text-[var(--ink)]">{evidenceCount}</strong>{' '}
            {evidenceCount === 1 ? 'evidence item' : 'evidence items'}
          </span>
        </div>

        {briefing && (
          <p className="m-0 text-base leading-relaxed text-[var(--ink)] max-w-3xl">{briefing}</p>
        )}
      </header>

      {/* Genuine Gaps Callout */}
      <Callout variant="gaps" title="Genuine gaps — what we could not verify">
        {gapItems.length > 0 ? (
          <ul className="m-0 pl-5 space-y-1.5">
            {gapItems.map((gap, index) => (
              <li key={index}>{gap}</li>
            ))}
          </ul>
        ) : (
          <p className="m-0">
            No specific evidentiary gaps were annotated on this record. If you identify an omission
            or unverified claim,{' '}
            <Link
              to={`/correction?case=${encodeURIComponent(caseData.slug)}`}
              className="underline text-[var(--accent-text)] hover:text-[var(--ink)]"
            >
              submit a correction
            </Link>
            .
          </p>
        )}
      </Callout>

      {/* Evidence Items List */}
      <section aria-labelledby="evidence-heading" className="space-y-4">
        <div className="flex items-baseline justify-between gap-2">
          <h2 id="evidence-heading" className="m-0 text-2xl font-bold text-[var(--ink)]">
            Evidence &amp; Primary Records
          </h2>
          <span className="text-xs font-[var(--font-mono)] text-[var(--ink-secondary)]">
            {caseData.evidence_items.length} indexed
          </span>
        </div>

        {caseData.evidence_items.length === 0 ? (
          <Card tone="light" className="p-5 text-sm text-[var(--ink-secondary)]">
            No evidence exhibits are attached to this case yet.
          </Card>
        ) : (
          <ul className="list-none m-0 p-0 space-y-2.5">
            {caseData.evidence_items.map((item) => (
              <li key={item.id}>
                <Card interactive className="p-4">
                  <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                    <div className="flex items-start gap-3 min-w-0">
                      <div className="mt-0.5">
                        <EvidenceTypeIcon type={item.type || item.source_type} />
                      </div>
                      <div className="min-w-0 space-y-1.5">
                        <h3 className="m-0 text-base font-semibold font-[var(--font-sans)] text-[var(--ink)] leading-snug">
                          {item.title}
                        </h3>
                        {item.source_citation && (
                          <p className="m-0 text-xs text-[var(--ink-secondary)] line-clamp-2">
                            {item.source_citation}
                          </p>
                        )}
                        <div className="flex flex-wrap items-center gap-1.5 pt-0.5">
                          <VerificationTierBadge tier={item.verification_tier} />
                          <LicenseStatusBadge status={item.license_status} />
                        </div>
                      </div>
                    </div>

                    <Link
                      to={`/evidence/${encodeURIComponent(item.id)}`}
                      className="inline-flex items-center justify-center min-h-[44px] px-3.5 py-2 text-sm font-medium bg-[var(--surface-light)] text-[var(--accent-text)] border border-[var(--surface-border-strong)] rounded-[var(--radius-sm)] hover:bg-[var(--accent)] hover:text-[var(--ink-on-dark)] hover:border-[var(--accent)] transition-colors duration-[var(--duration-fast)] no-underline shrink-0"
                    >
                      View &rarr;
                    </Link>
                  </div>
                </Card>
              </li>
            ))}
          </ul>
        )}
      </section>

      {/* Chronological Timeline */}
      {timelineEvents.length > 0 && (
        <section aria-labelledby="timeline-heading" className="space-y-4">
          <h2 id="timeline-heading" className="m-0 text-2xl font-bold text-[var(--ink)]">
            Chronological Timeline
          </h2>
          <ol className="list-none m-0 p-0 space-y-3 border-l border-[var(--surface-border-strong)] pl-5">
            {timelineEvents.map((ev) => (
              <li key={ev.id} className="relative">
                <span
                  aria-hidden="true"
                  className="absolute -left-[25px] top-1.5 w-2.5 h-2.5 rounded-full bg-[var(--accent)] border-2 border-[var(--surface-light)]"
                />
                <Card tone="light" className="p-4 space-y-1.5">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <time className="text-xs font-[var(--font-mono)] font-semibold text-[var(--accent-text)]">
                      {formatArchivalDate(ev.event_start_date)}
                      {ev.event_end_date ? ` – ${formatArchivalDate(ev.event_end_date)}` : ''}
                    </time>
                    <div className="flex items-center gap-1.5">
                      <VerificationTierBadge tier={ev.verification_tier} />
                      {ev.has_conflict && <Badge variant="t3">Disputed date</Badge>}
                    </div>
                  </div>
                  <h3 className="m-0 text-base font-semibold font-[var(--font-sans)] text-[var(--ink)]">
                    {ev.title}
                  </h3>
                  {ev.summary && (
                    <p className="m-0 text-sm text-[var(--ink-secondary)] leading-relaxed">
                      {ev.summary}
                    </p>
                  )}
                </Card>
              </li>
            ))}
          </ol>
        </section>
      )}
    </article>
  );
}
