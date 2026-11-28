import { Link } from 'react-router-dom';
import { Card } from '../components/ui';

export function NotFound({ message }: { message?: string }) {
  return (
    <div className="max-w-xl mx-auto py-10">
      <Card tone="paper" className="p-6 sm:p-8 space-y-4">
        <p className="m-0 font-[var(--font-mono)] text-xs uppercase tracking-widest text-[var(--accent-text)]">
          404 · Record Not Found
        </p>
        <h1 className="m-0 text-2xl sm:text-3xl font-bold text-[var(--ink)]">
          This record does not exist
        </h1>
        <p className="m-0 text-sm sm:text-base text-[var(--ink-secondary)] leading-relaxed">
          {message ??
            'The page or dossier you requested could not be found in the published archive.'}
        </p>
        <div className="pt-2">
          <Link
            to="/"
            className="inline-flex items-center justify-center min-h-[44px] px-4 py-2 text-sm font-medium bg-[var(--accent)] text-[var(--ink-on-dark)] rounded-[var(--radius-md)] no-underline hover:bg-[var(--accent-hover)]"
          >
            Return to Published Cases
          </Link>
        </div>
      </Card>
    </div>
  );
}
