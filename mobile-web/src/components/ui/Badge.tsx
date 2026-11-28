import type { ReactNode } from 'react';

export type BadgeVariant =
  | 't1'
  | 't2'
  | 't3'
  | 't4'
  | 'public_domain'
  | 'licensed'
  | 'recreated'
  | 'category'
  | 'neutral';

export interface BadgeProps {
  variant?: BadgeVariant;
  children: ReactNode;
  icon?: ReactNode;
  className?: string;
  title?: string;
}

const VARIANT_STYLES: Record<BadgeVariant, string> = {
  t1: 'bg-[var(--t1-bg)] text-[var(--t1-text)] border-[var(--t1-border)]',
  t2: 'bg-[var(--t2-bg)] text-[var(--t2-text)] border-[var(--t2-border)]',
  t3: 'bg-[var(--t3-bg)] text-[var(--t3-text)] border-[var(--t3-border)]',
  t4: 'bg-[var(--t4-bg)] text-[var(--t4-text)] border-[var(--t4-border)]',
  public_domain: 'bg-[var(--t1-bg)] text-[var(--t1-text)] border-[var(--t1-border)]',
  licensed: 'bg-[var(--t2-bg)] text-[var(--t2-text)] border-[var(--t2-border)]',
  recreated: 'bg-[var(--t3-bg)] text-[var(--t3-text)] border-[var(--t3-border)]',
  category: 'bg-[var(--paper)] text-[var(--ink)] border-[var(--surface-border-strong)]',
  neutral: 'bg-[var(--t4-bg)] text-[var(--t4-text)] border-[var(--surface-border-subtle)]',
};

export function Badge({
  variant = 'neutral',
  children,
  icon,
  className = '',
  title,
}: BadgeProps) {
  return (
    <span
      title={title}
      className={`inline-flex items-center gap-1.5 px-2 py-0.5 text-xs font-semibold tracking-wide border rounded-[var(--radius-sm)] leading-tight whitespace-nowrap ${VARIANT_STYLES[variant]} ${className}`}
    >
      {icon && (
        <span aria-hidden="true" className="inline-flex shrink-0 items-center">
          {icon}
        </span>
      )}
      <span>{children}</span>
    </span>
  );
}

export const TIER_DEFINITIONS: Record<number, { short: string; label: string; definition: string }> =
  {
    1: {
      short: 'T1',
      label: 'T1 · Primary Official',
      definition: 'Primary government, court, or statutory public record verified directly from the issuing authority.',
    },
    2: {
      short: 'T2',
      label: 'T2 · Corroborated Secondary',
      definition: 'Corroborated record, licensed third-party exhibit, or sourced recreation backed by primary citations.',
    },
    3: {
      short: 'T3',
      label: 'T3 · Single-Source Reported',
      definition: 'Reported by a single named outlet or witness without independent official corroboration.',
    },
    4: {
      short: 'T4',
      label: 'T4 · Unverified / Disputed',
      definition: 'Contested or unverified claim included solely for completeness; treat with caution.',
    },
  };

export function VerificationTierBadge({
  tier,
  verbose = false,
  className = '',
}: {
  tier: number;
  verbose?: boolean;
  className?: string;
}) {
  const clamped = tier >= 1 && tier <= 4 ? tier : 4;
  const variant = `t${clamped}` as 't1' | 't2' | 't3' | 't4';
  const info = TIER_DEFINITIONS[clamped] ?? TIER_DEFINITIONS[4]!;
  return (
    <Badge
      variant={variant}
      title={info.definition}
      className={`font-[var(--font-mono)] ${className}`}
    >
      {verbose ? info.label : info.short}
    </Badge>
  );
}

export const LICENSE_EXPLANATIONS: Record<
  string,
  { variant: BadgeVariant; label: string; explanation: string; isWarning: boolean }
> = {
  PUBLIC_DOMAIN: {
    variant: 'public_domain',
    label: 'PUBLIC DOMAIN',
    explanation: 'Public-domain government or historical record free of copyright restrictions.',
    isWarning: false,
  },
  LICENSED: {
    variant: 'licensed',
    label: 'LICENSED',
    explanation: 'Third-party exhibit published under documented license or explicit rights clearance.',
    isWarning: false,
  },
  RECREATED: {
    variant: 'recreated',
    label: 'RECREATED',
    explanation: 'In-house editorial recreation derived from underlying sources — not an original artifact.',
    isWarning: true,
  },
};

export function WarningTriangleIcon({ className = 'w-3.5 h-3.5' }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 20 20"
      fill="currentColor"
      className={className}
      aria-hidden="true"
      focusable="false"
    >
      <path
        fillRule="evenodd"
        d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 6a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 6zm0 9a1 1 0 100-2 1 1 0 000 2z"
        clipRule="evenodd"
      />
    </svg>
  );
}

export function LicenseStatusBadge({
  status,
  className = '',
}: {
  status: string;
  className?: string;
}) {
  const normalized = status.toUpperCase();
  const meta = LICENSE_EXPLANATIONS[normalized] ?? {
    variant: 'neutral' as BadgeVariant,
    label: normalized || 'UNSPECIFIED',
    explanation: 'Licensing status recorded in the evidence metadata.',
    isWarning: false,
  };
  return (
    <Badge
      variant={meta.variant}
      icon={meta.isWarning ? <WarningTriangleIcon /> : undefined}
      title={meta.explanation}
      className={className}
    >
      {meta.label}
    </Badge>
  );
}
