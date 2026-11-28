import { useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { Button, Callout, Card, Skeleton } from '../components/ui';
import { EvidenceSourcePanel } from '../components/EvidenceSourcePanel';
import { EvidenceViewer } from '../components/EvidenceViewer';
import { ShareButton } from '../components/ShareButton';
import { ApiError } from '../lib/api';
import { useEvidence } from '../lib/queries';
import { NotFound } from './NotFound';

export function EvidenceDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data: evidence, isLoading, isError, error, refetch } = useEvidence(id);
  const [showHashHelp, setShowHashHelp] = useState(false);

  if (isLoading) {
    return (
      <div aria-busy="true" aria-label="Loading evidence record" className="space-y-6">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="h-9 w-2/3" />
        <Skeleton className="h-44 w-full" />
        <Skeleton className="h-72 w-full" />
      </div>
    );
  }

  if (isError) {
    if (error instanceof ApiError && error.status === 404) {
      return <NotFound message="The requested evidence exhibit was not found or is not published." />;
    }
    return (
      <Callout variant="danger" title="Unable to load evidence exhibit">
        <p className="m-0 mb-3">
          {error instanceof Error
            ? error.message
            : 'An unexpected error occurred while fetching this exhibit.'}
        </p>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" size="sm" onClick={() => void refetch()}>
            Retry
          </Button>
          <Link
            to="/"
            className="inline-flex items-center min-h-[38px] px-3 py-1.5 text-sm font-medium underline text-[var(--ink)]"
          >
            Return to Cases
          </Link>
        </div>
      </Callout>
    );
  }

  if (!evidence) {
    return <NotFound message="The requested evidence exhibit was not found." />;
  }

  return (
    <article className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <button
          type="button"
          onClick={() => navigate(-1)}
          className="inline-flex items-center gap-1 min-h-[44px] px-2 text-sm font-medium text-[var(--accent-text)] hover:text-[var(--ink)] underline cursor-pointer bg-transparent border-0"
        >
          &larr; Back
        </button>
        <ShareButton title={evidence.title} text={evidence.source_citation || evidence.description} />
      </div>

      {/* Exhibit Header */}
      <header className="space-y-2">
        <h1 className="m-0 text-2xl sm:text-3xl font-bold leading-tight text-[var(--ink)]">
          {evidence.title}
        </h1>
        {evidence.description && (
          <p className="m-0 text-sm sm:text-base text-[var(--ink-secondary)] leading-relaxed max-w-3xl">
            {evidence.description}
          </p>
        )}
      </header>

      {/* MANDATORY NON-COLLAPSIBLE SOURCE & LICENSING PANEL (Always first before file viewer) */}
      <EvidenceSourcePanel evidence={evidence} />

      {/* File Viewer */}
      <section aria-label="Evidence document viewer" className="space-y-2">
        <EvidenceViewer evidence={evidence} />
      </section>

      {/* Cryptographic Integrity Footer */}
      <Card
        tone="paper"
        className="p-4 space-y-2"
        aria-label="Document cryptographic integrity"
      >
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h2 className="m-0 text-xs font-semibold font-[var(--font-sans)] uppercase tracking-wider text-[var(--ink-on-paper-muted)]">
            Integrity Checksum (SHA-256)
          </h2>
          <button
            type="button"
            aria-expanded={showHashHelp}
            onClick={() => setShowHashHelp((prev) => !prev)}
            className="text-xs underline font-medium text-[var(--accent-text)] hover:text-[var(--ink)] bg-transparent border-0 cursor-pointer min-h-[32px] px-1"
          >
            {showHashHelp ? 'Hide verification help' : 'How to verify'}
          </button>
        </div>
        <p
          data-testid="evidence-sha256"
          className="m-0 font-[var(--font-mono)] text-xs break-all text-[var(--ink)] bg-[var(--surface-light)] px-3 py-2 border border-[var(--surface-border-subtle)] rounded-[var(--radius-sm)]"
        >
          {evidence.sha256_hash || 'Checksum pending'}
        </p>
        {showHashHelp && (
          <p className="m-0 text-xs text-[var(--ink-secondary)] leading-relaxed">
            After downloading the original file, run{' '}
            <code className="px-1 py-0.5 bg-[var(--surface-light)] rounded-[var(--radius-sm)]">
              shasum -a 256 &lt;filename&gt;
            </code>{' '}
            (macOS/Linux) or{' '}
            <code className="px-1 py-0.5 bg-[var(--surface-light)] rounded-[var(--radius-sm)]">
              certutil -hashfile &lt;filename&gt; SHA256
            </code>{' '}
            (Windows) and confirm the output matches this exact 64-character hex digest.
          </p>
        )}
      </Card>
    </article>
  );
}
