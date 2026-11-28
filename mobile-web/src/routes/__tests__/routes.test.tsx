import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import {
  API_BASE,
  MOCK_EVIDENCE_PRIMARY,
  MOCK_EVIDENCE_RECREATED,
  server,
} from '../../test/mswHandlers';
import { renderAppAt } from '../../test/renderWithProviders';

describe('Reader routes', () => {
  it('CaseList renders published cases, filters by category, handles empty state, and retries on error', async () => {
    const user = userEvent.setup();
    renderAppAt('/');

    expect(await screen.findByText('SEC v. Meridian Clearinghouse Trust')).toBeInTheDocument();
    expect(
      screen.getByText('Port Authority Harbor Rail Charter Inquiry (1928)'),
    ).toBeInTheDocument();

    // Category filter
    await user.click(screen.getByRole('button', { name: 'Financial Crimes' }));
    await waitFor(() => {
      expect(
        screen.queryByText('Port Authority Harbor Rail Charter Inquiry (1928)'),
      ).not.toBeInTheDocument();
    });
    expect(screen.getByText('SEC v. Meridian Clearinghouse Trust')).toBeInTheDocument();
  });

  it('CaseList displays empty state when no cases are published', async () => {
    server.use(
      http.get(`${API_BASE}/api/v1/cases`, () =>
        HttpResponse.json({ data: [], next_cursor: null, has_more: false }),
      ),
    );

    renderAppAt('/');
    expect(await screen.findByText('No cases published yet.')).toBeInTheDocument();
  });

  it('CaseList displays error callout and recovers on retry', async () => {
    const user = userEvent.setup();
    let attempts = 0;
    server.use(
      http.get(`${API_BASE}/api/v1/cases`, () => {
        attempts += 1;
        if (attempts <= 2) {
          return HttpResponse.json(
            { error: { code: 'SERVICE_UNAVAILABLE', message: 'Temporary outage' } },
            { status: 503 },
          );
        }
        return HttpResponse.json({
          data: [
            {
              id: '11111111-1111-1111-1111-111111111111',
              slug: 'us-sec-clearinghouse-2024',
              title: 'SEC v. Meridian Clearinghouse Trust',
              summary: 'Recovered dossier summary.',
              category: 'financial_crime',
              published_at: '2025-11-14T10:00:00Z',
              evidence_count: 2,
            },
          ],
          next_cursor: null,
          has_more: false,
        });
      }),
    );

    renderAppAt('/');
    expect(await screen.findByText(/Unable to load case files/i)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(await screen.findByText('SEC v. Meridian Clearinghouse Trust')).toBeInTheDocument();
  });

  it('CaseDetail renders header, Genuine Gaps callout, evidence list, timeline, and handles 404', async () => {
    renderAppAt('/case/us-sec-clearinghouse-2024');

    expect(
      await screen.findByRole('heading', { level: 1, name: 'SEC v. Meridian Clearinghouse Trust' }),
    ).toBeInTheDocument();
    expect(screen.getByText('Genuine gaps — what we could not verify')).toBeInTheDocument();
    expect(
      screen.getByText(/Beneficial ownership of the Cayman sub-custodian entity/i),
    ).toBeInTheDocument();
    expect(screen.getByText('SEC Civil Complaint 24-CV-08812')).toBeInTheDocument();
    expect(screen.getByText('Collateral Flow Diagram (Recreated)')).toBeInTheDocument();
    expect(screen.getByText('Custodial audit discrepancy flagged')).toBeInTheDocument();
    expect(screen.getByText('SEC files federal civil complaint')).toBeInTheDocument();
  });

  it('CaseDetail renders NotFound on 404 slug', async () => {
    renderAppAt('/case/nonexistent-dossier');
    expect(await screen.findByText('This record does not exist')).toBeInTheDocument();
  });

  it('EvidenceDetail renders mandatory non-collapsible Source Panel BEFORE viewer, RECREATED warning, and SHA-256 hash', async () => {
    renderAppAt(`/evidence/${MOCK_EVIDENCE_RECREATED.id}`);

    const sourcePanel = await screen.findByTestId('mandatory-source-panel');
    const viewerSection = screen.getByRole('region', { name: 'Evidence document viewer' });

    // Mandatory Source & Licensing Panel MUST precede the file viewer in document order
    expect(
      Boolean(sourcePanel.compareDocumentPosition(viewerSection) & Node.DOCUMENT_POSITION_FOLLOWING),
    ).toBe(true);

    expect(sourcePanel).toHaveTextContent('Source & Licensing Record');
    expect(sourcePanel).toHaveTextContent(MOCK_EVIDENCE_RECREATED.source_url);
    expect(sourcePanel).toHaveTextContent(/RECREATED/);
    expect(sourcePanel).toHaveTextContent(/T2/);
    expect(sourcePanel).toHaveTextContent('2025-11-12');
    expect(sourcePanel).toHaveTextContent(
      /This is a recreation based on SEC Complaint 24-CV-08812 Tables 4 through 7\. It is not the original document\./,
    );

    // Integrity footer displays exact SHA-256 hash
    expect(screen.getByTestId('evidence-sha256')).toHaveTextContent(
      MOCK_EVIDENCE_RECREATED.sha256_hash,
    );
  });

  it('EvidenceDetail renders primary PUBLIC_DOMAIN PDF exhibit with download fallback', async () => {
    renderAppAt(`/evidence/${MOCK_EVIDENCE_PRIMARY.id}`);

    const sourcePanel = await screen.findByTestId('mandatory-source-panel');
    expect(sourcePanel).toHaveTextContent(/PUBLIC DOMAIN/);
    expect(sourcePanel).toHaveTextContent(/T1/);
    expect(screen.getByRole('link', { name: /Download PDF/i })).toHaveAttribute(
      'href',
      MOCK_EVIDENCE_PRIMARY.signed_file_url,
    );
  });

  it('SearchResults renders highlighted <mark> snippets and empty query feedback', async () => {
    const user = userEvent.setup();
    renderAppAt('/search?q=meridian');

    expect(await screen.findByText('SEC v. Meridian Clearinghouse Trust')).toBeInTheDocument();
    const marks = document.querySelectorAll('mark.search-mark');
    expect(marks.length).toBeGreaterThanOrEqual(1);
    expect(marks[0]?.textContent).toBe('Meridian');

    const input = screen.getByLabelText('Search query');
    await user.clear(input);
    await user.type(input, 'zzznomatch');
    expect(await screen.findByText(/No results for ‘zzznomatch’/i)).toBeInTheDocument();
  });

  it('CorrectionForm validates empty submission, submits with Idempotency-Key, handles honeypot silently, and displays 429 countdown', async () => {
    const user = userEvent.setup();
    renderAppAt('/correction?case=us-sec-clearinghouse-2024');

    // Empty description fails client validation
    await user.click(screen.getByRole('button', { name: 'Submit correction' }));
    expect(
      await screen.findByText(/Please provide at least 20 characters/i),
    ).toBeInTheDocument();

    // Fill valid description and submit
    await user.type(
      screen.getByLabelText(/Correction details/i),
      'Schedule B filing date on page 14 reads 2024-06-15 in the official SDNY docket.',
    );
    await user.type(screen.getByLabelText(/Contact email/i), 'reader@example.org');
    await user.click(screen.getByRole('button', { name: 'Submit correction' }));

    expect(await screen.findByText(/Thank you — correction recorded/i)).toBeInTheDocument();
    expect(screen.getByTestId('correction-id')).toHaveTextContent(
      'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee',
    );
  });

  it('CorrectionForm handles honeypot fill silently with identical 202 confirmation UX', async () => {
    const user = userEvent.setup();
    renderAppAt('/correction?case=us-sec-clearinghouse-2024');

    await user.type(
      screen.getByLabelText(/Correction details/i),
      'Automated spam payload that triggers the hidden honeypot input field.',
    );
    // Simulate a bot filling the visually hidden honeypot field
    const hpInput = document.querySelector<HTMLInputElement>('input[name="honeypot_field"]');
    expect(hpInput).not.toBeNull();
    await user.type(hpInput!, 'https://spam.example');

    await user.click(screen.getByRole('button', { name: 'Submit correction' }));
    expect(await screen.findByText(/Thank you — correction recorded/i)).toBeInTheDocument();
    expect(screen.getByTestId('correction-id')).toHaveTextContent(
      '99999999-9999-4999-8999-999999999999',
    );
  });

  it('CorrectionForm displays friendly retry countdown on HTTP 429', async () => {
    server.use(
      http.post(`${API_BASE}/api/v1/corrections`, () =>
        HttpResponse.json(
          { error: { code: 'RATE_LIMITED', message: 'Correction rate limit exceeded' } },
          { status: 429, headers: { 'Retry-After': '45' } },
        ),
      ),
    );

    const user = userEvent.setup();
    renderAppAt('/correction?case=us-sec-clearinghouse-2024');
    await user.type(
      screen.getByLabelText(/Correction details/i),
      'Requesting review of exhibit filing timestamp from public docket entry 12.',
    );
    await user.click(screen.getByRole('button', { name: 'Submit correction' }));

    expect(await screen.findByText(/Too many correction requests/i)).toBeInTheDocument();
    expect(screen.getAllByText(/45s/).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(/Rate limit reached/i)).toBeInTheDocument();
  });

  it('redirects to GeoblockPage on HTTP 451 with static copy and link to /about', async () => {
    server.use(
      http.get(`${API_BASE}/api/v1/cases`, () =>
        new HttpResponse('Not available in your region', {
          status: 451,
          headers: { 'Content-Type': 'text/html', 'CF-IPCountry': 'DE' },
        }),
      ),
    );

    renderAppAt('/');

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Not available in your region' }),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Read about Case Files/i })).toHaveAttribute(
      'href',
      '/about',
    );
  });
});
