-- =============================================================================
-- Migration: 000002_seed_dev.down.sql
-- Exact inverse of 000002_seed_dev.up.sql — deletes all seeded rows in strict
-- reverse foreign-key dependency order.
-- =============================================================================

BEGIN;

-- 1. Delete seeded correction request
DELETE FROM correction_requests
WHERE id = '60000000-0000-0000-0000-000000000001';

-- 2. Delete seeded timeline events
DELETE FROM case_timeline_events
WHERE id IN (
    '50000000-0000-0000-0000-000000000001',
    '50000000-0000-0000-0000-000000000002'
);

-- 3. Delete seeded evidence items
DELETE FROM evidence_items
WHERE id IN (
    '40000000-0000-0000-0000-000000000001',
    '40000000-0000-0000-0000-000000000002',
    '40000000-0000-0000-0000-000000000003'
);

-- 4. Unlink circular FK on seeded case (ensuring status is DRAFT first)
UPDATE cases
SET status = 'DRAFT',
    published_legal_review_id = NULL,
    published_at = NULL
WHERE id = '10000000-0000-0000-0000-000000000001';

-- 5. Delete seeded legal review record
DELETE FROM legal_review_records
WHERE id = '20000000-0000-0000-0000-000000000001';

-- 6. Delete seeded case
DELETE FROM cases
WHERE id = '10000000-0000-0000-0000-000000000001';

-- 7. Delete seeded licensing record
DELETE FROM licensing_records
WHERE id = '30000000-0000-0000-0000-000000000001';

-- 8. Delete fixed dev admin user
DELETE FROM admin_users
WHERE id = '00000000-0000-0000-0000-000000000001';

COMMIT;
