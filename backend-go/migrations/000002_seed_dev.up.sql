-- =============================================================================
-- Migration: 000002_seed_dev.up.sql
-- Inserts exactly ONE fully-tagged development test case ('test-case-001')
-- per PO Resolutions R1–R4 (ADR-016).
-- WARNING: Development/test seed ONLY — never apply to production.
-- =============================================================================

BEGIN;

-- 1. Fixed internal dev admin_users row (R3)
-- dev seed — cannot authenticate in production
INSERT INTO admin_users (
    id,
    email,
    password_hash,
    role,
    is_active
) VALUES (
    '00000000-0000-0000-0000-000000000001',
    'admin-test@truecrime.local',
    '$argon2id$v=19$m=65536,t=3,p=1$DEVSEEDINVALIDNONAUTH$DEVSEEDCANNOTAUTHENTICATEINPRODUCTION',
    'ADMIN',
    TRUE
);

-- 2. Licensing record governing the LICENSED third_party_exhibit item (R2)
INSERT INTO licensing_records (
    id,
    rights_holder,
    license_type,
    scope_territory,
    credit_line,
    proof_object_key,
    expires_on,
    recorded_by
) VALUES (
    '30000000-0000-0000-0000-000000000001',
    'Dev Archive Rights Holder LLC',
    'DIRECT_WRITTEN_GRANT',
    'US-IN-NON-EEA',
    'Courtesy of Dev Archive Rights Holder LLC (License #DEV-001)',
    'licenses/dev-seed-written-grant-001.pdf',
    NULL,
    '00000000-0000-0000-0000-000000000001'
);

-- 3. Draft test case ('test-case-001') with populated Genuine Gaps
INSERT INTO cases (
    id,
    slug,
    title,
    category,
    jurisdiction,
    eu_origin,
    gdpr_verified,
    status,
    official_briefing,
    genuine_gaps,
    published_legal_review_id,
    created_by,
    updated_by
) VALUES (
    '10000000-0000-0000-0000-000000000001',
    'test-case-001',
    'Test Case — Placeholder',
    'financial_crime',
    'US-FEDERAL',
    FALSE,
    FALSE,
    'DRAFT',
    'Placeholder Official Briefing grounded strictly in Tier 1 federal docket filings for local schema verification.',
    '[
        {
            "claim": "Exact clearing timestamp of offshore wire transfer #2",
            "why_unverified": "Corresponding bank attachment remains under federal protective order in test docket 00-cr-0001.",
            "highest_available_tier": "Tier 3",
            "citation_url": null
        }
    ]'::jsonb,
    NULL,
    '00000000-0000-0000-0000-000000000001',
    '00000000-0000-0000-0000-000000000001'
);

-- 4. Approved legal_review_records row for test-case-001 (R3/R4)
INSERT INTO legal_review_records (
    id,
    case_id,
    reviewer_id,
    external_counsel_ref,
    decision,
    prohibited_checklist_passed,
    genuine_gaps_verified,
    license_audit_passed,
    notes,
    reviewed_at
) VALUES (
    '20000000-0000-0000-0000-000000000001',
    '10000000-0000-0000-0000-000000000001',
    '00000000-0000-0000-0000-000000000001',
    'DEV-SEED',
    'APPROVED',
    TRUE,
    TRUE,
    TRUE,
    'Development seed legal sign-off record for automated publish-gate verification.',
    NOW()
);

-- Link legal review record to draft case (case remains DRAFT until explicitly published)
UPDATE cases
SET published_legal_review_id = '20000000-0000-0000-0000-000000000001'
WHERE id = '10000000-0000-0000-0000-000000000001';

-- 5. Three evidence_items demonstrating all 3 license statuses per R1 & R2
-- (a) PUBLIC_DOMAIN + gov_primary_record + Tier 1
INSERT INTO evidence_items (
    id,
    case_id,
    title,
    source_type,
    verification_tier,
    license_status,
    licensing_record_id,
    recreation_source_ids,
    recreation_author_note,
    source_citation,
    source_url,
    issuing_authority,
    license_basis_note,
    mime_type,
    byte_size,
    sha256_hash,
    object_key,
    processing_status,
    extracted_text,
    page_count,
    ocr_used,
    prohibited_content_cleared,
    retrieved_at
) VALUES (
    '40000000-0000-0000-0000-000000000001',
    '10000000-0000-0000-0000-000000000001',
    'Federal Criminal Indictment (Placeholder)',
    'gov_primary_record',
    1,
    'PUBLIC_DOMAIN',
    NULL,
    NULL,
    NULL,
    'U.S. v. Placeholder, S.D.N.Y. Docket 24-cr-00001, ECF No. 1',
    'https://www.justice.gov/example-placeholder-indictment',
    'U.S. District Court S.D.N.Y.',
    '17 U.S.C. § 105 federal government officer work',
    'application/pdf',
    102400,
    'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
    'cases/10000000-0000-0000-0000-000000000001/evidence/40000000-0000-0000-0000-000000000001/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.pdf',
    'COMPLETED',
    'UNITED STATES DISTRICT COURT SOUTHERN DISTRICT OF NEW YORK — PLACEHOLDER INDICTMENT TEXT.',
    12,
    FALSE,
    TRUE,
    NOW()
);

-- (b) LICENSED + third_party_exhibit + Tier 2 + licensing_record_id FK (proves legal path for third-party exhibits)
INSERT INTO evidence_items (
    id,
    case_id,
    title,
    source_type,
    verification_tier,
    license_status,
    licensing_record_id,
    recreation_source_ids,
    recreation_author_note,
    source_citation,
    source_url,
    issuing_authority,
    license_basis_note,
    mime_type,
    byte_size,
    sha256_hash,
    object_key,
    processing_status,
    extracted_text,
    page_count,
    ocr_used,
    prohibited_content_cleared,
    retrieved_at
) VALUES (
    '40000000-0000-0000-0000-000000000002',
    '10000000-0000-0000-0000-000000000001',
    'Recorded Subject Interview Transcript Excerpt (Licensed Third-Party Exhibit)',
    'third_party_exhibit',
    2,
    'LICENSED',
    '30000000-0000-0000-0000-000000000001',
    NULL,
    NULL,
    'Dev Archive Interview Transcript #01, attached as Defense Exhibit B in 24-cr-00001',
    NULL,
    'Dev Archive Rights Holder LLC',
    'Direct written permission on file via licensing_record_id 30000000-0000-0000-0000-000000000001',
    'text/plain',
    8192,
    'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
    'cases/10000000-0000-0000-0000-000000000001/evidence/40000000-0000-0000-0000-000000000002/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.txt',
    'COMPLETED',
    'Direct verbatim interview excerpt from licensed third-party archive.',
    3,
    FALSE,
    TRUE,
    NOW()
);

-- (c) RECREATED + in_house_recreation + Tier 2 + non-empty recreation_source_ids & note (R1 & R2)
INSERT INTO evidence_items (
    id,
    case_id,
    title,
    source_type,
    verification_tier,
    license_status,
    licensing_record_id,
    recreation_source_ids,
    recreation_author_note,
    source_citation,
    source_url,
    issuing_authority,
    license_basis_note,
    mime_type,
    byte_size,
    sha256_hash,
    object_key,
    processing_status,
    extracted_text,
    page_count,
    ocr_used,
    prohibited_content_cleared,
    retrieved_at
) VALUES (
    '40000000-0000-0000-0000-000000000003',
    '10000000-0000-0000-0000-000000000001',
    'Recreated Shell-Company Fund Flow Diagram',
    'in_house_recreation',
    2,
    'RECREATED',
    NULL,
    '["40000000-0000-0000-0000-000000000001", "40000000-0000-0000-0000-000000000002"]'::jsonb,
    'Original in-house diagram reconstructed strictly from Tier 1 indictment counts and Tier 2 subject transcript; omits sealed bank routing numbers.',
    'Derived from S.D.N.Y. 24-cr-00001 ECF No. 1 & Licensed Transcript #01',
    NULL,
    'Internal Editorial Graphics (Recreated)',
    'In-house factual recreation watermarked per EVIDENCE_INTEGRITY_SPEC.md §2',
    'image/png',
    65536,
    'cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc',
    'cases/10000000-0000-0000-0000-000000000001/evidence/40000000-0000-0000-0000-000000000003/cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc.png',
    'COMPLETED',
    'Recreated visual flow diagram: Entity A -> Offshore Account B -> Entity C.',
    1,
    TRUE,
    TRUE,
    NOW()
);

-- 6. Two case_timeline_events
INSERT INTO case_timeline_events (
    id,
    case_id,
    event_start_date,
    event_end_date,
    sort_order,
    event_type,
    title,
    summary,
    verification_tier,
    linked_evidence_ids,
    has_conflict,
    conflicting_sources
) VALUES
(
    '50000000-0000-0000-0000-000000000001',
    '10000000-0000-0000-0000-000000000001',
    '2023-03-15',
    NULL,
    1,
    'transaction',
    'Alleged Wire Transfer to Offshore Entity',
    'Indictment alleges transfer of funds across shell intermediaries prior to regulatory inquiry.',
    1,
    '["40000000-0000-0000-0000-000000000001", "40000000-0000-0000-0000-000000000003"]'::jsonb,
    FALSE,
    '[]'::jsonb
),
(
    '50000000-0000-0000-0000-000000000002',
    '10000000-0000-0000-0000-000000000001',
    '2024-01-10',
    NULL,
    2,
    'filing',
    'Federal Indictment Unsealed in S.D.N.Y.',
    'Grand jury indictment filed and unsealed; subject disputed timeline details in prior recorded interview.',
    1,
    '["40000000-0000-0000-0000-000000000001", "40000000-0000-0000-0000-000000000002"]'::jsonb,
    TRUE,
    '[
        {
            "source_name": "S.D.N.Y. Indictment ECF No. 1",
            "verification_tier": 1,
            "source_date": "2024-01-10",
            "excerpt": "Indictment states transfer occurred on March 15, 2023.",
            "evidence_id": "40000000-0000-0000-0000-000000000001"
        },
        {
            "source_name": "Dev Archive Subject Interview Transcript #01",
            "verification_tier": 2,
            "source_date": "2023-11-02",
            "excerpt": "Subject stated in interview that funds were not moved until April 2023.",
            "evidence_id": "40000000-0000-0000-0000-000000000002"
        }
    ]'::jsonb
);

-- 7. One correction_request row
INSERT INTO correction_requests (
    id,
    ticket_id,
    case_id,
    evidence_id,
    request_type,
    requester_contact_email,
    description,
    supporting_citation,
    status
) VALUES (
    '60000000-0000-0000-0000-000000000001',
    'CORR-DEV-SEED-001',
    '10000000-0000-0000-0000-000000000001',
    '40000000-0000-0000-0000-000000000001',
    'factual_correction',
    'factchecker-dev@example.invalid',
    'Development seed correction request verifying table constraints and admin triage workflow.',
    'S.D.N.Y. Docket 24-cr-00001 ECF No. 1 p. 4',
    'OPEN'
);

COMMIT;
