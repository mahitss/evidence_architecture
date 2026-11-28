import type { ReactNode } from 'react';
import { WarningTriangleIcon } from './Badge';

export type CalloutVariant = 'gaps' | 'warning' | 'danger' | 'info';

export interface CalloutProps {
  variant?: CalloutVariant;
  title: string;
  children: ReactNode;
  className?: string;
  role?: 'region' | 'alert' | 'note' | 'status';
  headingLevel?: 2 | 3;
}

const VARIANT_CLASSES: Record<CalloutVariant, { box: string; heading: string; iconColor: string }> =
  {
    gaps: {
      box: 'bg-[var(--paper)] border-[var(--accent)] text-[var(--ink)]',
      heading: 'text-[var(--ink)]',
      iconColor: 'text-[var(--accent-text)]',
    },
    warning: {
      box: 'bg-[var(--t3-bg)] border-[var(--t3-border)] text-[var(--ink)]',
      heading: 'text-[var(--t3-text)]',
      iconColor: 'text-[var(--t3-text)]',
    },
    danger: {
      box: 'bg-[var(--danger-bg)] border-[var(--danger-border)] text-[var(--ink)]',
      heading: 'text-[var(--danger-text)]',
      iconColor: 'text-[var(--danger-text)]',
    },
    info: {
      box: 'bg-[var(--t2-bg)] border-[var(--t2-border)] text-[var(--ink)]',
      heading: 'text-[var(--t2-text)]',
      iconColor: 'text-[var(--t2-text)]',
    },
  };

export function Callout({
  variant = 'info',
  title,
  children,
  className = '',
  role,
  headingLevel = 2,
}: CalloutProps) {
  const styles = VARIANT_CLASSES[variant];
  const resolvedRole = role ?? (variant === 'danger' || variant === 'warning' ? 'alert' : 'note');
  const HeadingTag = headingLevel === 3 ? 'h3' : 'h2';

  return (
    <section
      role={resolvedRole}
      aria-label={title}
      className={`border rounded-[var(--radius-md)] p-4 ${styles.box} ${className}`}
    >
      <div className="flex items-start gap-2.5">
        <span className={`mt-0.5 shrink-0 ${styles.iconColor}`} aria-hidden="true">
          {variant === 'warning' || variant === 'danger' ? (
            <WarningTriangleIcon className="w-4 h-4" />
          ) : (
            <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
              <path
                fillRule="evenodd"
                d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a.75.75 0 000 1.5h.253a.25.25 0 01.244.304l-.459 2.066A1.75 1.75 0 0010.747 15H11a.75.75 0 000-1.5h-.253a.25.25 0 01-.244-.304l.459-2.066A1.75 1.75 0 009.253 9H9z"
                clipRule="evenodd"
              />
            </svg>
          )}
        </span>
        <div className="min-w-0 flex-1 space-y-1.5">
          <HeadingTag className={`m-0 text-sm font-semibold font-[var(--font-sans)] ${styles.heading}`}>
            {title}
          </HeadingTag>
          <div className="text-sm leading-relaxed text-[var(--ink)]">{children}</div>
        </div>
      </div>
    </section>
  );
}
