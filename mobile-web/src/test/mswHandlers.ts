import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';

export const API_BASE = 'http://127.0.0.1:8080';

export const MOCK_CASES = [
  {
    id: '11111111-1111-1111-1111-111111111111',
    slug: 'us-sec-clearinghouse-2024',
    title: 'SEC v. Meridian Clearinghouse Trust',
    summary:
      'Federal civil enforcement action alleging undisclosed off-ledger collateral rehypothecation across municipal bond accounts.',
    category: 'financial_crime',
    jurisdiction: 'US-SDNY',
    published_at: '2025-11-14T10:00:00Z',
    updated_at: '2025-11-15T08:00:00Z',
    evidence_count: 2,
  },
  {
    id: '22222222-2222-2222-2222-222222222222',
    slug: 'harbor-rail-charter-1928',
    title: 'Port Authority Harbor Rail Charter Inquiry (1928)',
    summary:
      'State legislative commission transcript and engineering ledgers auditing municipal bond disbursements for pier rail spurs.',
    category: 'historical_public_record',
    jurisdiction: 'US-NY',
    published_at: '2025-10-03T12:00:00Z',
    updated_at: '2025-10-04T09:00:00Z',
    evidence_count: 1,
  },
];

export const MOCK_CASE_DETAIL = {
  id: '11111111-1111-1111-1111-111111111111',
  slug: 'us-sec-clearinghouse-2024',
  title: 'SEC v. Meridian Clearinghouse Trust',
  summary:
    'Federal civil enforcement action alleging undisclosed off-ledger collateral rehypothecation across municipal bond accounts.',
  official_briefing:
    'In November 2024, the U.S. Securities and Exchange Commission filed a civil complaint in the Southern District of New York detailing discrepancies between Meridian Clearinghouse quarterly filings and custodial settlement statements.',
  category: 'financial_crime',
  jurisdiction: 'US-SDNY',
  published_at: '2025-11-14T10:00:00Z',
  updated_at: '2025-11-15T08:00:00Z',
  evidence_count: 2,
  genuine_gaps: [
    {
      claim: 'Beneficial ownership of the Cayman sub-custodian entity',
      why_unverified: 'Registry filings remain sealed under offshore banking confidentiality rules',
    },
  ],
  genuine_gaps_note:
    'Beneficial ownership of the Cayman sub-custodian entity (Registry filings remain sealed)',
  evidence_items: [
    {
      id: 'ev-primary-001',
      title: 'SEC Civil Complaint 24-CV-08812',
      description: 'Filed in U.S. District Court for the Southern District of New York.',
      type: 'gov_primary_record',
      source_type: 'gov_primary_record',
      verification_tier: 1,
      license_status: 'PUBLIC_DOMAIN',
      source_citation: 'US District Court SDNY Docket 24-CV-08812, Doc 1',
      sha256_hash: 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',
      is_redacted: false,
      warning_level: 'NONE',
    },
    {
      id: 'ev-recreated-002',
      title: 'Collateral Flow Diagram (Recreated)',
      description: 'Visual diagram summarizing Schedule B transfers from the SEC complaint.',
      type: 'in_house_recreation',
      source_type: 'in_house_recreation',
      verification_tier: 2,
      license_status: 'RECREATED',
      source_citation: 'Derived from SEC Complaint 24-CV-08812 Tables 4–7',
      sha256_hash: '9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08',
      is_redacted: false,
      warning_level: 'DOUBLE_WARNING',
    },
  ],
  timeline: [
    {
      id: 'tl-001',
      case_id: '11111111-1111-1111-1111-111111111111',
      event_start_date: '2024-06-15',
      event_end_date: null,
      sort_order: 1,
      event_type: 'filing',
      title: 'Custodial audit discrepancy flagged',
      summary: 'External auditor issued a qualified opinion noting unreconciled settlement balances.',
      verification_tier: 1,
      has_conflict: false,
    },
    {
      id: 'tl-002',
      case_id: '11111111-1111-1111-1111-111111111111',
      event_start_date: '2024-11-04',
      event_end_date: null,
      sort_order: 2,
      event_type: 'indictment',
      title: 'SEC files federal civil complaint',
      summary: 'Complaint docketed in SDNY seeking disgorgement and permanent injunction.',
      verification_tier: 1,
      has_conflict: false,
    },
  ],
  timeline_events: [
    {
      id: 'tl-001',
      case_id: '11111111-1111-1111-1111-111111111111',
      event_start_date: '2024-06-15',
      event_end_date: null,
      sort_order: 1,
      event_type: 'filing',
      title: 'Custodial audit discrepancy flagged',
      summary: 'External auditor issued a qualified opinion noting unreconciled settlement balances.',
      verification_tier: 1,
      has_conflict: false,
    },
    {
      id: 'tl-002',
      case_id: '11111111-1111-1111-1111-111111111111',
      event_start_date: '2024-11-04',
      event_end_date: null,
      sort_order: 2,
      event_type: 'indictment',
      title: 'SEC files federal civil complaint',
      summary: 'Complaint docketed in SDNY seeking disgorgement and permanent injunction.',
      verification_tier: 1,
      has_conflict: false,
    },
  ],
};

export const MOCK_EVIDENCE_PRIMARY = {
  id: 'ev-primary-001',
  case_id: '11111111-1111-1111-1111-111111111111',
  title: 'SEC Civil Complaint 24-CV-08812',
  description: 'Primary federal civil complaint filed in SDNY.',
  type: 'gov_primary_record',
  source_type: 'gov_primary_record',
  verification_tier: 1,
  license_status: 'PUBLIC_DOMAIN',
  license_basis_note: '17 U.S.C. § 105 federal government work',
  source_citation: 'US District Court SDNY Docket 24-CV-08812, Doc 1',
  issuing_authority: 'U.S. Securities and Exchange Commission',
  source_url: 'https://www.sec.gov/litigation/complaints/2024/comp-meridian.pdf',
  storage_url: 'http://127.0.0.1:8080/files/comp-meridian.pdf',
  signed_file_url: 'http://127.0.0.1:8080/files/comp-meridian.pdf',
  signed_url_expires_at: '2026-12-01T00:00:00Z',
  recreation_author_note: null,
  mime_type: 'application/pdf',
  byte_size: 245760,
  page_count: 14,
  ocr_used: false,
  sha256_hash: 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',
  is_redacted: false,
  redaction_reason: null,
  warning_level: 'NONE',
  retrieved_at: '2025-11-10T15:30:00Z',
};

export const MOCK_EVIDENCE_RECREATED = {
  id: 'ev-recreated-002',
  case_id: '11111111-1111-1111-1111-111111111111',
  title: 'Collateral Flow Diagram (Recreated)',
  description: 'Editorial diagram mapping bond transfers cited in Schedule B.',
  type: 'in_house_recreation',
  source_type: 'in_house_recreation',
  verification_tier: 2,
  license_status: 'RECREATED',
  license_basis_note: 'In-house editorial synthesis from public court tables',
  source_citation: 'Derived from SEC Complaint 24-CV-08812 Tables 4–7',
  issuing_authority: 'Case Files Editorial Desk',
  source_url: 'https://www.sec.gov/litigation/complaints/2024/comp-meridian.pdf',
  storage_url: 'http://127.0.0.1:8080/files/collateral-flow.png',
  signed_file_url: 'http://127.0.0.1:8080/files/collateral-flow.png',
  signed_url_expires_at: '2026-12-01T00:00:00Z',
  recreation_author_note: 'SEC Complaint 24-CV-08812 Tables 4 through 7',
  mime_type: 'image/png',
  byte_size: 82400,
  page_count: null,
  ocr_used: false,
  sha256_hash: '9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08',
  is_redacted: false,
  redaction_reason: null,
  warning_level: 'DOUBLE_WARNING',
  retrieved_at: '2025-11-12T09:00:00Z',
};

export const defaultHandlers = [
  http.get(`${API_BASE}/api/v1/cases`, ({ request }) => {
    const url = new URL(request.url);
    const category = url.searchParams.get('category');
    const filtered = category
      ? MOCK_CASES.filter((item) => item.category === category)
      : MOCK_CASES;
    return HttpResponse.json({
      data: filtered,
      next_cursor: null,
      has_more: false,
    });
  }),

  http.get(`${API_BASE}/api/v1/cases/:slug`, ({ params }) => {
    if (params.slug === MOCK_CASE_DETAIL.slug) {
      return HttpResponse.json(MOCK_CASE_DETAIL);
    }
    return HttpResponse.json(
      { error: { code: 'NOT_FOUND', message: 'case not found', request_id: 'req-404' } },
      { status: 404 },
    );
  }),

  http.get(`${API_BASE}/api/v1/evidence/:id`, ({ params }) => {
    if (params.id === MOCK_EVIDENCE_PRIMARY.id) {
      return HttpResponse.json(MOCK_EVIDENCE_PRIMARY);
    }
    if (params.id === MOCK_EVIDENCE_RECREATED.id) {
      return HttpResponse.json(MOCK_EVIDENCE_RECREATED);
    }
    return HttpResponse.json(
      { error: { code: 'NOT_FOUND', message: 'evidence not found', request_id: 'req-ev-404' } },
      { status: 404 },
    );
  }),

  http.get(`${API_BASE}/api/v1/search`, ({ request }) => {
    const url = new URL(request.url);
    const q = (url.searchParams.get('q') ?? '').toLowerCase();
    if (q.includes('meridian') || q.includes('clearinghouse')) {
      return HttpResponse.json({
        data: [
          {
            id: MOCK_CASES[0]!.id,
            slug: MOCK_CASES[0]!.slug,
            title: MOCK_CASES[0]!.title,
            summary: MOCK_CASES[0]!.summary,
            category: MOCK_CASES[0]!.category,
            published_at: MOCK_CASES[0]!.published_at,
            evidence_count: 2,
            highlight:
              'Federal complaint detailing <mark>Meridian</mark> <mark>Clearinghouse</mark> collateral schedules.',
          },
        ],
        next_cursor: null,
        has_more: false,
      });
    }
    return HttpResponse.json({
      data: [],
      next_cursor: null,
      has_more: false,
    });
  }),

  http.post(`${API_BASE}/api/v1/corrections`, async ({ request }) => {
    const idempotencyKey = request.headers.get('Idempotency-Key');
    if (!idempotencyKey || idempotencyKey.length < 16) {
      return HttpResponse.json(
        { error: { code: 'BAD_REQUEST', message: 'Idempotency-Key header is required' } },
        { status: 400 },
      );
    }
    const body = (await request.json()) as { honeypot_field?: string };
    const isHoneypot = Boolean(body.honeypot_field && body.honeypot_field.trim().length > 0);
    return HttpResponse.json(
      {
        correction_id: isHoneypot
          ? '99999999-9999-4999-8999-999999999999'
          : 'aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee',
        ticket_id: isHoneypot ? 'COR-ACCEPTED' : 'COR-2026-0042',
        status: 'OPEN',
        created_at: '2026-01-10T12:00:00Z',
      },
      { status: 202 },
    );
  }),
];

export const server = setupServer(...defaultHandlers);
