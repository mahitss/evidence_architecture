import { Link } from 'react-router-dom';
import { Card } from '../components/ui';

/**
 * Static HTTP 451 geoblock page (ADR-008).
 * Makes zero API calls and offers no bypass/demo mechanism.
 */
export function GeoblockPage() {
  return (
    <div className="max-w-xl mx-auto py-8">
      <Card tone="paper" className="p-6 sm:p-8 space-y-4">
        <p className="m-0 font-[var(--font-mono)] text-xs uppercase tracking-widest text-[var(--accent-text)]">
          HTTP 451 · Regional Availability Notice
        </p>
        <h1 className="m-0 text-2xl sm:text-3xl font-bold text-[var(--ink)]">
          Not available in your region
        </h1>
        <p className="m-0 text-sm sm:text-base leading-relaxed text-[var(--ink-secondary)]">
          Case Files is currently unavailable in your region due to regional privacy, defamation,
          and public-record publication rules. No user data or account information has been
          collected.
        </p>
        <div className="pt-2">
          <Link
            to="/about"
            className="inline-flex items-center justify-center min-h-[44px] px-4 py-2 text-sm font-medium bg-[var(--surface-light)] text-[var(--ink)] border border-[var(--surface-border-strong)] rounded-[var(--radius-md)] no-underline hover:bg-[var(--surface-card-hover)]"
          >
            Read about Case Files
          </Link>
        </div>
      </Card>
    </div>
  );
}
