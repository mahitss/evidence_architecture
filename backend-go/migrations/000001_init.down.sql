-- =============================================================================
-- Migration: 000001_init.down.sql
-- Exact inverse of 000001_init.up.sql — drops triggers, functions, circular FK,
-- and all 8 tables in strict reverse FK dependency order.
-- =============================================================================

BEGIN;

-- Drop triggers and trigger functions first
DROP TRIGGER IF EXISTS enforce_legal_review_immutability ON legal_review_records;
DROP FUNCTION IF EXISTS fn_enforce_legal_review_immutability();

DROP TRIGGER IF EXISTS enforce_audit_log_append_only ON audit_log;
DROP FUNCTION IF EXISTS fn_enforce_audit_log_append_only();

DROP TRIGGER IF EXISTS enforce_legal_review_before_publish ON cases;
DROP FUNCTION IF EXISTS fn_enforce_legal_review_before_publish();

-- Break circular FK before dropping tables
ALTER TABLE IF EXISTS cases
    DROP CONSTRAINT IF EXISTS fk_cases_published_legal_review;

-- Drop tables in strict reverse foreign-key dependency order
DROP TABLE IF EXISTS correction_requests;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS legal_review_records;
DROP TABLE IF EXISTS case_timeline_events;
DROP TABLE IF EXISTS evidence_items;
DROP TABLE IF EXISTS licensing_records;
DROP TABLE IF EXISTS cases;
DROP TABLE IF EXISTS admin_users;

COMMIT;
