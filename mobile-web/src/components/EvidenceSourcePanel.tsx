import {
  Callout,
  LICENSE_EXPLANATIONS,
  LicenseStatusBadge,
  TIER_DEFINITIONS,
  VerificationTierBadge,
} from './ui';
import { formatArchivalDate, isSafeHttpUrl } from '../lib/format';
import type { EvidenceDetail } from '../lib/schemas';

export interface EvidenceSourcePanelProps {
  evidence: EvidenceDetail;
}

/**
 * MANDATORY NON-COLLAPSIBLE Source & Licensing Panel (Evidence Integrity Spec).
 * Must always render at the top of EvidenceDetail before the file viewer.
 * Never add toggle/collapse/hide controls to this component.
 */
export function EvidenceSourcePanel({ evidence }: EvidenceSourcePanelProps) {
  const normalizedLicense = evidence.license_status.toUpperCase();
  const licenseInfo = LICENSE_EXPLANATIONS[normalizedLicense];
  const tierInfo = TIER_DEFINITIONS[evidence.verification_tier] ?? TIER_DEFINITIONS[4]!;
  const isRecreated = normalizedLicense === 'RECREATED';
  const safeSourceUrl = isSafeHttpUrl(evidence.source_url) ? evidence.source_url : null;

  return (
    <section
      aria-label="Source and licensing provenance"
      data-testid="mandatory-source-panel"
      className="bg-[var(--paper)] border border-[var(--accent)] rounded-[var(--radius-md)] p-4 sm:p-5 space-y-4"
    >
      <div className="flex flex-wrap items-center justify-between gap-2 pb-3 border-b border-[var(--surface-border-strong)]">
        <h2 className="text-base font-semibold font-[var(--font-sans)] uppercase tracking-wider text-[var(--ink)]">
          Source &amp; Licensing Record
        </h2>
        <div className="flex flex-wrap items-center gap-2">
          <VerificationTierBadge tier={evidence.verification_tier} verbose />
          <LicenseStatusBadge status={evidence.license_status} />
        </div>
      </div>

      {isRecreated && (
        <Callout variant="warning" title="Recreated Document Notice" role="alert" headingLevel={3}>
          <p className="m-0 font-medium">
            This is a recreation based on{' '}
            {evidence.recreation_author_note?.trim() ||
              evidence.source_citation?.trim() ||
              'underlying cited public records'}
            . It is not the original document.
          </p>
        </Callout>
      )}

      <dl className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm m-0">
        <div className="space-y-1">
          <dt className="text-xs font-semibold uppercase tracking-wider text-[var(--ink-on-paper-muted)]">
            Source URL &amp; Citation
          </dt>
          <dd className="m-0 space-y-1 text-[var(--ink)]">
            {safeSourceUrl ? (
              <a
                href={safeSourceUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="block break-all underline decoration-[var(--accent)] underline-offset-2 text-[var(--accent-text)] hover:text-[var(--ink)] font-medium"
              >
                {safeSourceUrl}
              </a>
            ) : (
              <span className="text-[var(--ink-on-paper-muted)]">No public external URL recorded</span>
            )}
            {evidence.source_citation && (
              <p className="m-0 text-xs text-[var(--ink-secondary)]">
                Citation: {evidence.source_citation}
              </p>
            )}
            {evidence.issuing_authority && (
              <p className="m-0 text-xs text-[var(--ink-secondary)]">
                Issuing authority: {evidence.issuing_authority}
              </p>
            )}
          </dd>
        </div>

        <div className="space-y-1">
          <dt className="text-xs font-semibold uppercase tracking-wider text-[var(--ink-on-paper-muted)]">
            License Status
          </dt>
          <dd className="m-0 space-y-1 text-[var(--ink)]">
            <p className="m-0">
              {licenseInfo?.explanation ?? 'Licensing status documented in archive metadata.'}
            </p>
            {evidence.license_basis_note && (
              <p className="m-0 text-xs text-[var(--ink-secondary)]">
                Basis: {evidence.license_basis_note}
              </p>
            )}
          </dd>
        </div>

        <div className="space-y-1">
          <dt className="text-xs font-semibold uppercase tracking-wider text-[var(--ink-on-paper-muted)]">
            Verification Tier
          </dt>
          <dd className="m-0 text-[var(--ink)]">
            <span className="font-semibold font-[var(--font-mono)] mr-1.5">{tierInfo.short}</span>
            <span>{tierInfo.definition}</span>
          </dd>
        </div>

        <div className="space-y-1">
          <dt className="text-xs font-semibold uppercase tracking-wider text-[var(--ink-on-paper-muted)]">
            Reported as of
          </dt>
          <dd className="m-0 font-[var(--font-mono)] text-[var(--ink)]">
            {formatArchivalDate(evidence.retrieved_at)}
          </dd>
        </div>
      </dl>
    </section>
  );
}
