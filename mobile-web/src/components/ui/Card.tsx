import type { HTMLAttributes, ReactNode } from 'react';

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
  interactive?: boolean;
  tone?: 'paper' | 'light';
}

export function Card({
  children,
  interactive = false,
  tone = 'paper',
  className = '',
  ...rest
}: CardProps) {
  const surface =
    tone === 'paper'
      ? 'bg-[var(--surface-card)] border-[var(--surface-border-subtle)]'
      : 'bg-[var(--surface-light)] border-[var(--surface-border-subtle)]';

  const hover = interactive
    ? 'transition-colors duration-[var(--duration-fast)] hover:bg-[var(--surface-card-hover)] hover:border-[var(--accent)]'
    : '';

  return (
    <div
      className={`border rounded-[var(--radius-md)] ${surface} ${hover} ${className}`}
      {...rest}
    >
      {children}
    </div>
  );
}
