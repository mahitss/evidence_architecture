import { useEffect, useState, type FormEvent } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { Button, Callout, Card, Input, Select, Textarea } from '../components/ui';
import { RateLimitError } from '../lib/api';
import { useCases, useSubmitCorrection } from '../lib/queries';
import { SubmitCorrectionFormSchema } from '../lib/schemas';

interface FormErrors {
  case_slug?: string;
  description?: string;
  contact_email?: string;
}

export function CorrectionForm() {
  const [searchParams] = useSearchParams();
  const prefilledCase = searchParams.get('case') ?? '';

  const { data: casesPage } = useCases();
  const mutation = useSubmitCorrection();

  const [caseSlug, setCaseSlug] = useState(prefilledCase);
  const [description, setDescription] = useState('');
  const [contactEmail, setContactEmail] = useState('');
  const [honeypot, setHoneypot] = useState('');
  const [fieldErrors, setFieldErrors] = useState<FormErrors>({});
  const [rateLimitCountdown, setRateLimitCountdown] = useState<number>(0);

  useEffect(() => {
    if (prefilledCase) {
      setCaseSlug(prefilledCase);
    }
  }, [prefilledCase]);

  useEffect(() => {
    if (rateLimitCountdown <= 0) return;
    const timer = window.setInterval(() => {
      setRateLimitCountdown((prev) => (prev <= 1 ? 0 : prev - 1));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [rateLimitCountdown]);

  const caseOptions = (casesPage?.data ?? []).map((item) => ({
    value: item.slug,
    label: `${item.title} (${item.slug})`,
  }));

  const hasPrefilledOption =
    caseSlug !== '' && caseOptions.some((opt) => opt.value === caseSlug);
  const mergedOptions =
    caseSlug && !hasPrefilledOption
      ? [{ value: caseSlug, label: caseSlug }, ...caseOptions]
      : caseOptions;

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFieldErrors({});

    const parsed = SubmitCorrectionFormSchema.safeParse({
      case_slug: caseSlug,
      description,
      contact_email: contactEmail,
      honeypot_field: honeypot,
    });

    if (!parsed.success) {
      const nextErrors: FormErrors = {};
      for (const issue of parsed.error.issues) {
        const key = issue.path[0];
        if (key === 'case_slug' && !nextErrors.case_slug) {
          nextErrors.case_slug = issue.message;
        } else if (key === 'description' && !nextErrors.description) {
          nextErrors.description = issue.message;
        } else if (key === 'contact_email' && !nextErrors.contact_email) {
          nextErrors.contact_email = issue.message;
        }
      }
      setFieldErrors(nextErrors);
      return;
    }

    mutation.mutate(parsed.data, {
      onError: (err) => {
        if (err instanceof RateLimitError) {
          setRateLimitCountdown(err.retryAfter);
        }
      },
    });
  };

  if (mutation.isSuccess && mutation.data) {
    return (
      <div className="max-w-2xl mx-auto space-y-6">
        <Card tone="paper" className="p-6 space-y-4">
          <h1 className="m-0 text-2xl sm:text-3xl font-bold text-[var(--ink)]">
            Thank you — correction recorded
          </h1>
          <p className="m-0 text-sm text-[var(--ink-secondary)] leading-relaxed">
            Your submission has been queued for editorial and legal review. No account was created
            and no tracking profile was stored.
          </p>
          <div className="p-3 bg-[var(--surface-light)] border border-[var(--surface-border-subtle)] rounded-[var(--radius-sm)] space-y-1">
            <p className="m-0 text-xs uppercase tracking-wider font-semibold text-[var(--ink-secondary)]">
              Correction Reference ID
            </p>
            <p
              data-testid="correction-id"
              className="m-0 font-[var(--font-mono)] text-sm font-semibold text-[var(--ink)] break-all"
            >
              {mutation.data.correction_id}
            </p>
            {mutation.data.ticket_id && (
              <p className="m-0 font-[var(--font-mono)] text-xs text-[var(--ink-secondary)]">
                Ticket: {mutation.data.ticket_id}
              </p>
            )}
          </div>
          <div className="flex flex-wrap gap-3 pt-2">
            {caseSlug && (
              <Link
                to={`/case/${encodeURIComponent(caseSlug)}`}
                className="inline-flex items-center justify-center min-h-[44px] px-4 py-2 text-sm font-medium bg-[var(--accent)] text-[var(--ink-on-dark)] rounded-[var(--radius-md)] no-underline hover:bg-[var(--accent-hover)]"
              >
                Return to case
              </Link>
            )}
            <Link
              to="/"
              className="inline-flex items-center justify-center min-h-[44px] px-4 py-2 text-sm font-medium bg-[var(--surface-light)] text-[var(--ink)] border border-[var(--surface-border-strong)] rounded-[var(--radius-md)] no-underline hover:bg-[var(--paper)]"
            >
              Browse all cases
            </Link>
          </div>
        </Card>
      </div>
    );
  }

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <header className="space-y-2 pb-4 border-b border-[var(--surface-border-subtle)]">
        <h1 className="m-0 text-3xl font-bold text-[var(--ink)]">Report a Correction</h1>
        <p className="m-0 text-sm text-[var(--ink-secondary)] leading-relaxed">
          Flag a factual error, broken primary source, licensing concern, or privacy issue.
          Submissions are anonymous unless you choose to provide a contact email.
        </p>
      </header>

      {mutation.isError && (
        <Callout variant="danger" title="Submission could not be accepted">
          {mutation.error instanceof RateLimitError ? (
            <p className="m-0">
              Too many correction requests from this network. Please wait{' '}
              <strong className="font-[var(--font-mono)]">
                {rateLimitCountdown > 0 ? `${rateLimitCountdown}s` : 'a moment'}
              </strong>{' '}
              before submitting again.
            </p>
          ) : (
            <p className="m-0">
              {mutation.error instanceof Error
                ? mutation.error.message
                : 'Unable to submit your correction right now.'}
            </p>
          )}
        </Callout>
      )}

      <Card tone="paper" className="p-5 sm:p-6">
        <form onSubmit={handleSubmit} noValidate className="space-y-5">
          {mergedOptions.length > 0 ? (
            <Select
              label="Case dossier"
              name="case_slug"
              required
              value={caseSlug}
              onChange={(event) => setCaseSlug(event.target.value)}
              options={mergedOptions}
              placeholder="Select a published case..."
              error={fieldErrors.case_slug}
            />
          ) : (
            <Input
              label="Case slug"
              name="case_slug"
              required
              value={caseSlug}
              onChange={(event) => setCaseSlug(event.target.value)}
              placeholder="e.g. us-sec-demo-2026"
              error={fieldErrors.case_slug}
            />
          )}

          <Textarea
            label="Correction details"
            name="description"
            required
            minLength={20}
            maxLength={2000}
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            placeholder="Describe the specific claim, date, or exhibit citation that requires review and include any public record citation if available."
            helperText={`${description.trim().length} / 2000 characters (minimum 20)`}
            error={fieldErrors.description}
          />

          <Input
            label="Contact email (optional)"
            name="contact_email"
            type="email"
            value={contactEmail}
            onChange={(event) => setContactEmail(event.target.value)}
            placeholder="reader@example.org"
            helperText="Optional. Only used if editors need clarification on your citation."
            error={fieldErrors.contact_email}
          />

          {/* Visually hidden bot honeypot field — never remove */}
          <div className="cf-hp-field" aria-hidden="true">
            <label htmlFor="honeypot_field">Leave this field blank</label>
            <input
              id="honeypot_field"
              name="honeypot_field"
              type="text"
              tabIndex={-1}
              autoComplete="off"
              value={honeypot}
              onChange={(event) => setHoneypot(event.target.value)}
            />
          </div>

          <div className="flex flex-wrap items-center justify-between gap-3 pt-2">
            <p className="m-0 text-xs text-[var(--ink-on-paper-muted)]">
              Protected by idempotent submission and rate limiting (3 requests/hour).
            </p>
            <Button
              type="submit"
              variant="primary"
              disabled={mutation.isPending || rateLimitCountdown > 0}
            >
              {mutation.isPending ? 'Submitting...' : 'Submit correction'}
            </Button>
          </div>
        </form>
      </Card>
    </div>
  );
}
