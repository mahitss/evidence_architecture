import { Link } from 'react-router-dom';
import { Callout, Card } from '../components/ui';

export function About() {
  return (
    <article className="max-w-3xl mx-auto space-y-6">
      <header className="space-y-2 pb-4 border-b border-[var(--surface-border-subtle)]">
        <h1 className="m-0 text-3xl sm:text-4xl font-bold text-[var(--ink)]">About Case Files</h1>
        <p className="m-0 text-sm font-semibold text-[var(--accent-text)]">
          All evidence is sourced with licensing status and verification tier.
        </p>
      </header>

      <section className="space-y-4 text-base leading-relaxed text-[var(--ink)]">
        <p className="m-0">
          Case Files is a public-record investigative archive built for readers who want to inspect
          primary documents rather than unsupported speculation. Every published dossier links
          claims to underlying court filings, regulatory releases, statutory records, or clearly
          labeled licensed and recreated exhibits—alongside explicit statements of what remains
          unverified.
        </p>
        <p className="m-0">
          Reading is completely anonymous: there are no user accounts, no tracking cookies, no
          behavioral analytics, and no advertising. Every exhibit carries an explicit verification
          tier (T1 through T4), a licensing classification (Public Domain, Licensed, or Recreated),
          and a SHA-256 cryptographic checksum so readers can independently verify downloaded files.
        </p>
      </section>

      <Callout variant="gaps" title="Legal & Editorial Disclaimer">
        <p className="m-0 mb-2">
          All dossiers are published strictly for informational and public-record archival purposes
          in accordance with our compliance policy (
          <code className="text-xs">docs/19-compliance/COMPLIANCE.md</code>). Nothing in this
          archive constitutes legal advice, and any individual accused or referenced in official
          filings is presumed innocent unless proven guilty in a court of law.
        </p>
        <p className="m-0">
          If you discover a factual inaccuracy, sealed record, licensing discrepancy, or privacy
          concern, please notify our editorial review team immediately.
        </p>
      </Callout>

      <Card tone="paper" className="p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div className="space-y-1">
          <h2 className="m-0 text-lg font-bold text-[var(--ink)]">Found an error or gap?</h2>
          <p className="m-0 text-sm text-[var(--ink-secondary)]">
            Submit an anonymous factual correction or takedown notice for legal review.
          </p>
        </div>
        <Link
          to="/correction"
          className="inline-flex items-center justify-center min-h-[44px] px-4 py-2 text-sm font-medium bg-[var(--accent)] text-[var(--ink-on-dark)] rounded-[var(--radius-md)] no-underline hover:bg-[var(--accent-hover)] shrink-0"
        >
          Report a correction
        </Link>
      </Card>
    </article>
  );
}
