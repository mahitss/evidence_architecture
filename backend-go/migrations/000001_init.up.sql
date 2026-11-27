-- =============================================================================
-- Migration: 000001_init.up.sql
-- Phase 09 Foundation — Core v1.0 PostgreSQL 15 Schema
-- Source of Truth: docs/08-database/DATABASE_DESIGN.md, ADR-007, ADR-009, ADR-016
-- =============================================================================

BEGIN;

-- -----------------------------------------------------------------------------
-- 1. admin_users (Internal operational accounts ONLY; no public consumer users)
-- -----------------------------------------------------------------------------
CREATE TABLE admin_users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'ADMIN',
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMPTZ NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_admin_users_role CHECK (role IN ('ADMIN', 'RESEARCHER'))
);

CREATE UNIQUE INDEX idx_admin_users_email_lower ON admin_users (LOWER(email));

-- -----------------------------------------------------------------------------
-- 2. cases (Core investigative dossier; published_legal_review_id FK added after
--    legal_review_records creation to resolve circular reference cleanly)
-- -----------------------------------------------------------------------------
CREATE TABLE cases (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                      TEXT NOT NULL,
    title                     TEXT NOT NULL,
    category                  TEXT NOT NULL,
    jurisdiction              TEXT NOT NULL DEFAULT 'US',
    eu_origin                 BOOLEAN NOT NULL DEFAULT FALSE,
    gdpr_verified             BOOLEAN NOT NULL DEFAULT FALSE,
    status                    TEXT NOT NULL DEFAULT 'DRAFT',
    official_briefing         TEXT NOT NULL DEFAULT '',
    genuine_gaps              JSONB NOT NULL DEFAULT '[]'::jsonb,
    published_corrections_log JSONB NOT NULL DEFAULT '[]'::jsonb,
    published_legal_review_id UUID NULL,
    search_sync_pending       BOOLEAN NOT NULL DEFAULT FALSE,
    published_at              TIMESTAMPTZ NULL,
    retracted_at              TIMESTAMPTZ NULL,
    retraction_reason         TEXT NULL,
    created_by                UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    updated_by                UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_cases_slug CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    CONSTRAINT chk_cases_title CHECK (length(trim(title)) BETWEEN 3 AND 200),
    CONSTRAINT chk_cases_category CHECK (category IN ('financial_crime', 'historical_public_record')),
    CONSTRAINT chk_cases_status CHECK (status IN ('DRAFT', 'IN_REVIEW', 'APPROVED', 'PUBLISHED', 'RETRACTED')),
    CONSTRAINT chk_eu_origin_requires_gdpr CHECK (
        status NOT IN ('IN_REVIEW', 'APPROVED', 'PUBLISHED')
        OR NOT (eu_origin = TRUE AND gdpr_verified = FALSE)
    ),
    CONSTRAINT chk_published_requires_legal_review CHECK (
        status <> 'PUBLISHED'
        OR (
            published_legal_review_id IS NOT NULL
            AND published_at IS NOT NULL
            AND jsonb_typeof(genuine_gaps) = 'array'
            AND jsonb_array_length(genuine_gaps) >= 1
        )
    )
);

CREATE UNIQUE INDEX idx_cases_slug ON cases (slug);
CREATE INDEX idx_cases_published_catalog
    ON cases (category, published_at DESC, id DESC)
    WHERE status = 'PUBLISHED';
CREATE INDEX idx_cases_search_sync
    ON cases (id)
    WHERE search_sync_pending = TRUE;

-- -----------------------------------------------------------------------------
-- 3. licensing_records (Proof of written rights-holder permission for LICENSED)
-- -----------------------------------------------------------------------------
CREATE TABLE licensing_records (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rights_holder    TEXT NOT NULL,
    license_type     TEXT NOT NULL,
    scope_territory  TEXT NOT NULL DEFAULT 'WORLDWIDE',
    credit_line      TEXT NOT NULL,
    proof_object_key TEXT NOT NULL,
    expires_on       DATE NULL,
    recorded_by      UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_licensing_records_expires
    ON licensing_records (expires_on)
    WHERE expires_on IS NOT NULL;

-- -----------------------------------------------------------------------------
-- 4. evidence_items (Primary & contextual exhibits with Q5 & 4x3 CHECKs)
-- -----------------------------------------------------------------------------
CREATE TABLE evidence_items (
    id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id                    UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    title                      TEXT NOT NULL,
    source_type                TEXT NOT NULL,
    verification_tier          SMALLINT NOT NULL,
    license_status             TEXT NOT NULL,
    licensing_record_id        UUID NULL REFERENCES licensing_records(id) ON DELETE RESTRICT,
    recreation_source_ids      JSONB NULL,
    recreation_author_note     TEXT NULL,
    source_citation            TEXT NOT NULL,
    source_url                 TEXT NULL,
    issuing_authority          TEXT NOT NULL,
    license_basis_note         TEXT NOT NULL,
    mime_type                  TEXT NOT NULL,
    byte_size                  BIGINT NOT NULL,
    sha256_hash                CHAR(64) NOT NULL,
    object_key                 TEXT NOT NULL,
    processing_status          TEXT NOT NULL DEFAULT 'PENDING',
    last_processed_job_id      UUID NULL,
    extracted_text             TEXT NULL,
    page_count                 INTEGER NULL,
    ocr_used                   BOOLEAN NOT NULL DEFAULT FALSE,
    prohibited_content_cleared BOOLEAN NOT NULL DEFAULT FALSE,
    is_redacted                BOOLEAN NOT NULL DEFAULT FALSE,
    redaction_reason           TEXT NULL,
    retrieved_at               TIMESTAMPTZ NOT NULL,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_source_type CHECK (
        source_type IN (
            'gov_primary_record',
            'court_opinion_or_order',
            'third_party_exhibit',
            'historical_public_domain',
            'licensed_media',
            'in_house_recreation'
        )
    ),
    CONSTRAINT chk_license_status CHECK (license_status IN ('PUBLIC_DOMAIN', 'LICENSED', 'RECREATED')),
    CONSTRAINT chk_verification_tier CHECK (verification_tier IN (1, 2, 3, 4)),
    -- MANDATORY Q5 CHECK CONSTRAINT (PO Decision Q5 / EVIDENCE_INTEGRITY_SPEC.md §2):
    CONSTRAINT chk_no_third_party_public_domain CHECK (
        NOT (source_type = 'third_party_exhibit' AND license_status = 'PUBLIC_DOMAIN')
    ),
    -- 4x3 Evidence Integrity Matrix Hard Blocks (EVIDENCE_INTEGRITY_SPEC.md §3):
    CONSTRAINT chk_no_recreated_tier3_tier4 CHECK (
        NOT (verification_tier IN (3, 4) AND license_status = 'RECREATED')
    ),
    CONSTRAINT chk_licensed_requires_record CHECK (
        license_status <> 'LICENSED' OR licensing_record_id IS NOT NULL
    ),
    CONSTRAINT chk_recreated_requires_sources CHECK (
        license_status <> 'RECREATED'
        OR (
            recreation_source_ids IS NOT NULL
            AND jsonb_typeof(recreation_source_ids) = 'array'
            AND jsonb_array_length(recreation_source_ids) >= 1
            AND recreation_author_note IS NOT NULL
            AND length(trim(recreation_author_note)) > 0
        )
    ),
    CONSTRAINT chk_source_citation CHECK (length(trim(source_citation)) > 0),
    CONSTRAINT chk_mime_type CHECK (mime_type IN ('application/pdf', 'image/png', 'image/jpeg', 'text/plain')),
    CONSTRAINT chk_byte_size CHECK (byte_size > 0 AND byte_size <= 26214400),
    CONSTRAINT chk_sha256_hash CHECK (sha256_hash ~ '^[a-f0-9]{64}$'),
    CONSTRAINT chk_processing_status CHECK (processing_status IN ('PENDING', 'COMPLETED', 'FAILED')),
    CONSTRAINT chk_page_count CHECK (page_count IS NULL OR page_count > 0)
);

CREATE UNIQUE INDEX idx_evidence_object_key ON evidence_items (object_key);
CREATE INDEX idx_evidence_case_id ON evidence_items (case_id, created_at ASC);
CREATE INDEX idx_evidence_case_sha256 ON evidence_items (case_id, sha256_hash);
CREATE INDEX idx_evidence_processing_pending
    ON evidence_items (processing_status)
    WHERE processing_status = 'PENDING';

-- -----------------------------------------------------------------------------
-- 5. case_timeline_events
-- -----------------------------------------------------------------------------
CREATE TABLE case_timeline_events (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id                 UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    event_start_date        DATE NOT NULL,
    event_end_date          DATE NULL,
    sort_order              INTEGER NOT NULL DEFAULT 0,
    event_type              TEXT NOT NULL,
    title                   TEXT NOT NULL,
    summary                 TEXT NOT NULL,
    verification_tier       SMALLINT NOT NULL,
    linked_evidence_ids     JSONB NOT NULL DEFAULT '[]'::jsonb,
    has_conflict            BOOLEAN NOT NULL DEFAULT FALSE,
    conflicting_sources     JSONB NOT NULL DEFAULT '[]'::jsonb,
    controlling_status_note TEXT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_timeline_dates CHECK (event_end_date IS NULL OR event_end_date >= event_start_date),
    CONSTRAINT chk_timeline_event_type CHECK (
        event_type IN ('filing', 'transaction', 'arrest', 'hearing', 'verdict', 'release', 'other')
    ),
    CONSTRAINT chk_timeline_verification_tier CHECK (verification_tier IN (1, 2, 3, 4)),
    CONSTRAINT chk_timeline_conflict_sources CHECK (
        NOT has_conflict
        OR (jsonb_typeof(conflicting_sources) = 'array' AND jsonb_array_length(conflicting_sources) >= 2)
    )
);

CREATE INDEX idx_timeline_case_order
    ON case_timeline_events (case_id, event_start_date ASC, sort_order ASC, id ASC);
CREATE INDEX idx_timeline_case_tier_type
    ON case_timeline_events (case_id, verification_tier, event_type);

-- -----------------------------------------------------------------------------
-- 6. legal_review_records (Immutable legal counsel sign-off records)
-- -----------------------------------------------------------------------------
CREATE TABLE legal_review_records (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id                     UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    reviewer_id                 UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    external_counsel_ref        TEXT NOT NULL,
    decision                    TEXT NOT NULL,
    prohibited_checklist_passed BOOLEAN NOT NULL DEFAULT FALSE,
    genuine_gaps_verified       BOOLEAN NOT NULL DEFAULT FALSE,
    license_audit_passed        BOOLEAN NOT NULL DEFAULT FALSE,
    notes                       TEXT NOT NULL DEFAULT '',
    reviewed_at                 TIMESTAMPTZ NOT NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_legal_review_decision CHECK (
        decision IN ('APPROVED', 'CHANGES_REQUESTED', 'REJECTED')
    ),
    CONSTRAINT chk_legal_review_approved_complete CHECK (
        decision <> 'APPROVED'
        OR (
            prohibited_checklist_passed = TRUE
            AND genuine_gaps_verified = TRUE
            AND license_audit_passed = TRUE
            AND length(trim(external_counsel_ref)) > 0
        )
    )
);

CREATE INDEX idx_legal_reviews_case_time
    ON legal_review_records (case_id, reviewed_at DESC);

-- Complete circular FK on cases.published_legal_review_id
ALTER TABLE cases
    ADD CONSTRAINT fk_cases_published_legal_review
    FOREIGN KEY (published_legal_review_id)
    REFERENCES legal_review_records(id)
    ON DELETE RESTRICT;

-- -----------------------------------------------------------------------------
-- 7. audit_log (Append-only, strictly zero-PII legal defense ledger)
-- -----------------------------------------------------------------------------
CREATE TABLE audit_log (
    id             BIGSERIAL PRIMARY KEY,
    actor_admin_id UUID NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    actor_role     TEXT NOT NULL,
    action         TEXT NOT NULL,
    entity_type    TEXT NOT NULL,
    entity_id      UUID NULL,
    reason_code    TEXT NULL,
    metadata       JSONB NOT NULL DEFAULT '{}'::jsonb,
    payload_sha256 CHAR(64) NULL,
    occurred_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_audit_actor_role CHECK (actor_role IN ('ADMIN', 'RESEARCHER', 'SYSTEM', 'ANONYMOUS')),
    CONSTRAINT chk_audit_payload_sha256 CHECK (payload_sha256 IS NULL OR payload_sha256 ~ '^[a-f0-9]{64}$')
);

CREATE INDEX idx_audit_entity ON audit_log (entity_type, entity_id, occurred_at DESC);
CREATE INDEX idx_audit_occurred ON audit_log (occurred_at DESC);

-- -----------------------------------------------------------------------------
-- 8. correction_requests (Public correction / takedown / privacy grievances)
-- -----------------------------------------------------------------------------
CREATE TABLE correction_requests (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id               TEXT NOT NULL,
    case_id                 UUID NULL REFERENCES cases(id) ON DELETE SET NULL,
    evidence_id             UUID NULL REFERENCES evidence_items(id) ON DELETE SET NULL,
    request_type            TEXT NOT NULL,
    requester_contact_email TEXT NOT NULL,
    description             TEXT NOT NULL,
    supporting_citation     TEXT NULL,
    status                  TEXT NOT NULL DEFAULT 'OPEN',
    resolution_note         TEXT NULL,
    triaged_by              UUID NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    triaged_at              TIMESTAMPTZ NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_correction_request_type CHECK (
        request_type IN ('factual_correction', 'defamation_takedown', 'copyright_takedown', 'privacy_grievance')
    ),
    CONSTRAINT chk_correction_description CHECK (length(trim(description)) BETWEEN 10 AND 5000),
    CONSTRAINT chk_correction_status CHECK (
        status IN ('OPEN', 'UNDER_REVIEW', 'RESOLVED_UPDATED', 'RESOLVED_NO_CHANGE', 'SPAM_DISMISSED')
    )
);

CREATE UNIQUE INDEX idx_corrections_ticket ON correction_requests (ticket_id);
CREATE INDEX idx_corrections_status_created ON correction_requests (status, created_at ASC);

-- -----------------------------------------------------------------------------
-- 9. Triggers — Hard Legal & Integrity Enforcement inside PostgreSQL
-- -----------------------------------------------------------------------------

-- Trigger A: Legal review publication gate on cases
CREATE OR REPLACE FUNCTION fn_enforce_legal_review_before_publish()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'PUBLISHED' THEN
        IF NEW.published_legal_review_id IS NULL THEN
            RAISE EXCEPTION 'Cannot publish case %: published_legal_review_id is required (legal review gate)', NEW.id
                USING ERRCODE = 'P0001';
        END IF;

        IF NOT EXISTS (
            SELECT 1
            FROM legal_review_records lrr
            WHERE lrr.id = NEW.published_legal_review_id
              AND lrr.case_id = NEW.id
              AND lrr.reviewer_id IS NOT NULL
              AND lrr.reviewed_at IS NOT NULL
              AND lrr.decision = 'APPROVED'
              AND lrr.prohibited_checklist_passed = TRUE
        ) THEN
            RAISE EXCEPTION 'Cannot publish case %: no APPROVED legal_review_records row (id=%) with reviewer_id, reviewed_at, and prohibited_checklist_passed=TRUE',
                NEW.id, NEW.published_legal_review_id
                USING ERRCODE = 'P0001';
        END IF;

        IF jsonb_typeof(NEW.genuine_gaps) <> 'array' OR jsonb_array_length(NEW.genuine_gaps) < 1 THEN
            RAISE EXCEPTION 'Cannot publish case %: at least one Genuine Gaps disclosure entry is required', NEW.id
                USING ERRCODE = 'P0001';
        END IF;

        IF EXISTS (
            SELECT 1
            FROM evidence_items ei
            WHERE ei.case_id = NEW.id
              AND (ei.prohibited_content_cleared = FALSE OR ei.processing_status <> 'COMPLETED')
        ) THEN
            RAISE EXCEPTION 'Cannot publish case %: all evidence_items must have prohibited_content_cleared=TRUE and processing_status=COMPLETED', NEW.id
                USING ERRCODE = 'P0001';
        END IF;

        IF NEW.published_at IS NULL THEN
            NEW.published_at := NOW();
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER enforce_legal_review_before_publish
    BEFORE INSERT OR UPDATE ON cases
    FOR EACH ROW
    EXECUTE FUNCTION fn_enforce_legal_review_before_publish();

-- Trigger B: Append-only audit_log (blocks UPDATE and DELETE unconditionally)
CREATE OR REPLACE FUNCTION fn_enforce_audit_log_append_only()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only: % operations are strictly prohibited', TG_OP
        USING ERRCODE = 'P0001';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER enforce_audit_log_append_only
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW
    EXECUTE FUNCTION fn_enforce_audit_log_append_only();

-- Trigger C: Immutability of legal_review_records (blocks UPDATE of decision/reviewer_id/reviewed_at & core sign-off fields)
CREATE OR REPLACE FUNCTION fn_enforce_legal_review_immutability()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.decision IS DISTINCT FROM OLD.decision
       OR NEW.reviewer_id IS DISTINCT FROM OLD.reviewer_id
       OR NEW.reviewed_at IS DISTINCT FROM OLD.reviewed_at
       OR NEW.case_id IS DISTINCT FROM OLD.case_id
       OR NEW.external_counsel_ref IS DISTINCT FROM OLD.external_counsel_ref
       OR NEW.prohibited_checklist_passed IS DISTINCT FROM OLD.prohibited_checklist_passed THEN
        RAISE EXCEPTION 'legal_review_records is immutable: updating decision, reviewer_id, reviewed_at, or sign-off fields is prohibited'
            USING ERRCODE = 'P0001';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER enforce_legal_review_immutability
    BEFORE UPDATE ON legal_review_records
    FOR EACH ROW
    EXECUTE FUNCTION fn_enforce_legal_review_immutability();

COMMIT;
