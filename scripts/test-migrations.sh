#!/usr/bin/env bash
# =============================================================================
# scripts/test-migrations.sh — End-to-End PostgreSQL Migration & Constraint Tests
#
# Executes tests T1–T16 proving:
#   - Clean forward / backward / forward migration execution
#   - All 8 core v1.0 tables created and dropped cleanly
#   - NOT NULL & CHECK constraints on evidence_items (including Q5 & 4x3 matrix)
#   - Legal-review publication trigger (bidirectional pass & block)
#   - Append-only trigger on audit_log (blocks UPDATE and DELETE)
#   - Immutability trigger on legal_review_records (blocks UPDATE)
#   - Dev seed migration (000002_seed_dev) applies and rolls back cleanly
#
# Exits non-zero on ANY test failure. Never weakens constraints.
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
cd "${ROOT_DIR}"

export PATH="/usr/local/go/bin:${HOME}/go/bin:${PATH}"
export PGHOST="127.0.0.1"
export PG_HOST="127.0.0.1"
export DATABASE_URL="${DATABASE_URL:-postgres://postgres@127.0.0.1:5432/truecrime_dev?sslmode=disable}"

psql_exec() {
  psql -v ON_ERROR_STOP=1 -X -q -t -A "${DATABASE_URL}" -c "$1"
}

assert_sql_fails() {
  local test_id="$1"
  local expected_pattern="$2"
  local sql="$3"
  local err_out=""

  set +e
  err_out="$(psql -v ON_ERROR_STOP=1 -X "${DATABASE_URL}" -c "${sql}" 2>&1)"
  local rc=$?
  set -e

  if [[ "${rc}" -eq 0 ]]; then
    echo "FAIL [${test_id}]: Expected SQL to FAIL (matching '${expected_pattern}'), but it SUCCEEDED!" >&2
    exit 1
  fi

  if ! printf '%s' "${err_out}" | grep -qiE "${expected_pattern}"; then
    echo "FAIL [${test_id}]: SQL failed, but error did not match expected pattern '${expected_pattern}'." >&2
    echo "Actual error: ${err_out}" >&2
    exit 1
  fi

  echo "PASS [${test_id}]: Correctly rejected -> $(printf '%s' "${err_out}" | tr '\n' ' ' | sed 's/  */ /g')"
}

echo "============================================================================="
echo "Running Phase 09 Migration & Database Constraint Verification Suite (T1–T16)"
echo "============================================================================="

# T1. Start local backing services
echo ""
echo "--- [T1] make services-up ---"
make services-up
echo "PASS [T1]: Local PostgreSQL 15 and Redis 7 started."

# Ensure clean schema start if a prior local DB already had migrations applied
echo ""
echo "--- [T2] bash scripts/migrate.sh up 1 (apply 000001_init.up.sql) ---"
bash scripts/migrate.sh up 1
bash scripts/migrate.sh status
echo "PASS [T2]: 000001_init.up.sql applied cleanly."

# T3. Verify all 8 tables exist
echo ""
echo "--- [T3] Verify all 8 core tables exist ---"
TABLE_COUNT="$(psql_exec "SELECT COUNT(*) FROM pg_tables WHERE schemaname = 'public' AND tablename IN ('cases','evidence_items','case_timeline_events','licensing_records','legal_review_records','audit_log','admin_users','correction_requests');")"
echo "Found core table count: ${TABLE_COUNT}"
if [[ "${TABLE_COUNT}" != "8" ]]; then
  echo "FAIL [T3]: Expected 8 core tables, got ${TABLE_COUNT}." >&2
  exit 1
fi
echo "PASS [T3]: All 8 core tables exist."

# Seed temporary parent admin + draft case for negative tests T4–T7 (cleaned up before T8)
psql_exec "
INSERT INTO admin_users (id, email, password_hash, role)
VALUES ('90000000-0000-0000-0000-000000000001', 'negtest@truecrime.local', '\$argon2id\$devnegtest', 'ADMIN');
INSERT INTO cases (id, slug, title, category, jurisdiction, status, official_briefing, genuine_gaps, created_by, updated_by)
VALUES (
  '91000000-0000-0000-0000-000000000001',
  'neg-test-case',
  'Negative Constraint Test Case',
  'financial_crime',
  'US-FEDERAL',
  'DRAFT',
  'Draft briefing',
  '[{\"claim\":\"Gap\",\"why_unverified\":\"Sealed\",\"highest_available_tier\":\"None\"}]'::jsonb,
  '90000000-0000-0000-0000-000000000001',
  '90000000-0000-0000-0000-000000000001'
);
"

# T4. Insert evidence WITHOUT license_status -> must FAIL with NOT NULL violation
echo ""
echo "--- [T4] Insert evidence WITHOUT license_status (expect NOT NULL violation) ---"
assert_sql_fails "T4" "null value in column \"license_status\".*not-null" "
INSERT INTO evidence_items (
  case_id, title, source_type, verification_tier, license_status,
  source_citation, issuing_authority, license_basis_note,
  mime_type, byte_size, sha256_hash, object_key, retrieved_at
) VALUES (
  '91000000-0000-0000-0000-000000000001',
  'Missing License Status Exhibit',
  'gov_primary_record',
  1,
  NULL,
  'Docket 00-cr-0000',
  'DOJ',
  'Test',
  'application/pdf',
  1024,
  '1111111111111111111111111111111111111111111111111111111111111111',
  'cases/neg/t4.pdf',
  NOW()
);
"

# T5. Insert evidence with license_status='INVALID' -> must FAIL with CHECK constraint
echo ""
echo "--- [T5] Insert evidence with license_status='INVALID' (expect CHECK violation) ---"
assert_sql_fails "T5" "chk_license_status" "
INSERT INTO evidence_items (
  case_id, title, source_type, verification_tier, license_status,
  source_citation, issuing_authority, license_basis_note,
  mime_type, byte_size, sha256_hash, object_key, retrieved_at
) VALUES (
  '91000000-0000-0000-0000-000000000001',
  'Invalid License Status Exhibit',
  'gov_primary_record',
  1,
  'INVALID',
  'Docket 00-cr-0000',
  'DOJ',
  'Test',
  'application/pdf',
  1024,
  '2222222222222222222222222222222222222222222222222222222222222222',
  'cases/neg/t5.pdf',
  NOW()
);
"

# T6. CRITICAL Q5 CHECK: third_party_exhibit + PUBLIC_DOMAIN -> MUST FAIL
echo ""
echo "--- [T6] Insert third_party_exhibit + PUBLIC_DOMAIN (expect Q5 CHECK violation) ---"
assert_sql_fails "T6" "chk_no_third_party_public_domain" "
INSERT INTO evidence_items (
  case_id, title, source_type, verification_tier, license_status,
  source_citation, issuing_authority, license_basis_note,
  mime_type, byte_size, sha256_hash, object_key, retrieved_at
) VALUES (
  '91000000-0000-0000-0000-000000000001',
  'Illegal Public Domain Third-Party PACER Exhibit',
  'third_party_exhibit',
  1,
  'PUBLIC_DOMAIN',
  'PACER Exhibit 14-A',
  'Private Corp',
  'Attempted auto-tag',
  'application/pdf',
  2048,
  '3333333333333333333333333333333333333333333333333333333333333333',
  'cases/neg/t6.pdf',
  NOW()
);
"

# Bonus verification for R1: Tier 3 + RECREATED must also fail chk_no_recreated_tier3_tier4
assert_sql_fails "T6-R1-matrix" "chk_no_recreated_tier3_tier4" "
INSERT INTO evidence_items (
  case_id, title, source_type, verification_tier, license_status,
  recreation_source_ids, recreation_author_note,
  source_citation, issuing_authority, license_basis_note,
  mime_type, byte_size, sha256_hash, object_key, retrieved_at
) VALUES (
  '91000000-0000-0000-0000-000000000001',
  'Blocked Tier 3 Recreated Exhibit',
  'in_house_recreation',
  3,
  'RECREATED',
  '[\"src-1\"]'::jsonb,
  'Should be rejected',
  'News article paraphrase',
  'Internal',
  'Blocked cell',
  'image/png',
  2048,
  '4444444444444444444444444444444444444444444444444444444444444444',
  'cases/neg/t6r1.png',
  NOW()
);
"

# T7. Transition DRAFT case to PUBLISHED without legal_review_record -> MUST FAIL with trigger exception
echo ""
echo "--- [T7] Publish DRAFT case without legal_review_record (expect trigger exception) ---"
assert_sql_fails "T7" "published_legal_review_id is required|legal review gate" "
UPDATE cases
SET status = 'PUBLISHED'
WHERE id = '91000000-0000-0000-0000-000000000001';
"

# Clean up temporary T4–T7 rows before T8 seed migration
psql_exec "
DELETE FROM cases WHERE id = '91000000-0000-0000-0000-000000000001';
DELETE FROM admin_users WHERE id = '90000000-0000-0000-0000-000000000001';
"

# T8. Apply 000002_seed_dev.up.sql via scripts/migrate.sh up
echo ""
echo "--- [T8] Apply 000002_seed_dev.up.sql via scripts/migrate.sh up ---"
bash scripts/migrate.sh up
bash scripts/migrate.sh status
SEED_CASE_COUNT="$(psql_exec "SELECT COUNT(*) FROM cases WHERE slug = 'test-case-001' AND status = 'DRAFT';")"
SEED_EVID_COUNT="$(psql_exec "SELECT COUNT(*) FROM evidence_items WHERE case_id = '10000000-0000-0000-0000-000000000001';")"
SEED_TIMELINE_COUNT="$(psql_exec "SELECT COUNT(*) FROM case_timeline_events WHERE case_id = '10000000-0000-0000-0000-000000000001';")"
SEED_CORR_COUNT="$(psql_exec "SELECT COUNT(*) FROM correction_requests WHERE ticket_id = 'CORR-DEV-SEED-001';")"
if [[ "${SEED_CASE_COUNT}" != "1" || "${SEED_EVID_COUNT}" != "3" || "${SEED_TIMELINE_COUNT}" != "2" || "${SEED_CORR_COUNT}" != "1" ]]; then
  echo "FAIL [T8]: Unexpected seed counts (case=${SEED_CASE_COUNT}, evidence=${SEED_EVID_COUNT}, timeline=${SEED_TIMELINE_COUNT}, corrections=${SEED_CORR_COUNT})." >&2
  exit 1
fi
echo "PASS [T8]: Seed migration 000002_seed_dev applied cleanly (1 case, 3 evidence items, 2 timeline events, 1 correction request)."

# T9 & T10. Append-only audit_log trigger (block UPDATE and DELETE)
echo ""
echo "--- [T9 & T10] audit_log append-only trigger (block UPDATE and DELETE) ---"
AUDIT_ID="$(psql_exec "
INSERT INTO audit_log (actor_admin_id, actor_role, action, entity_type, entity_id, metadata)
VALUES (NULL, 'SYSTEM', 'MIGRATION_TEST_ENTRY', 'case', '10000000-0000-0000-0000-000000000001', '{\"check\":\"append_only\"}'::jsonb)
RETURNING id;
")"
echo "Inserted test audit_log row id=${AUDIT_ID}"
assert_sql_fails "T9" "audit_log is append-only.*UPDATE" "
UPDATE audit_log SET action = 'TAMPERED' WHERE id = ${AUDIT_ID};
"
assert_sql_fails "T10" "audit_log is append-only.*DELETE" "
DELETE FROM audit_log WHERE id = ${AUDIT_ID};
"

# T15. Immutability trigger on legal_review_records
echo ""
echo "--- [T15] Attempt UPDATE legal_review_records SET decision='REJECTED' (expect immutability trigger) ---"
assert_sql_fails "T15" "legal_review_records is immutable" "
UPDATE legal_review_records
SET decision = 'REJECTED'
WHERE id = '20000000-0000-0000-0000-000000000001';
"

# T16. Bidirectional legal-review gate verification
echo ""
echo "--- [T16] Bidirectional legal-review publish gate verification ---"
# T16(a): Publish seeded test-case-001 (has APPROVED review record) -> MUST SUCCEED
psql_exec "UPDATE cases SET status = 'PUBLISHED' WHERE slug = 'test-case-001';"
PUB_STATUS="$(psql_exec "SELECT status || '|' || (published_at IS NOT NULL)::text FROM cases WHERE slug = 'test-case-001';")"
if [[ "${PUB_STATUS}" != "PUBLISHED|true" ]]; then
  echo "FAIL [T16a]: Expected PUBLISHED|true, got ${PUB_STATUS}" >&2
  exit 1
fi
echo "PASS [T16a]: Seeded case 'test-case-001' transitioned to PUBLISHED with APPROVED review record."

# T16(b): Reset seeded case back to DRAFT
psql_exec "UPDATE cases SET status = 'DRAFT', published_at = NULL WHERE slug = 'test-case-001';"
DRAFT_STATUS="$(psql_exec "SELECT status FROM cases WHERE slug = 'test-case-001';")"
if [[ "${DRAFT_STATUS}" != "DRAFT" ]]; then
  echo "FAIL [T16b]: Expected DRAFT after reset, got ${DRAFT_STATUS}" >&2
  exit 1
fi
echo "PASS [T16b]: Seeded case 'test-case-001' reset cleanly to DRAFT."

# T16(c): Attempt to mutate decision on seeded legal_review_records -> MUST FAIL
assert_sql_fails "T16c" "legal_review_records is immutable" "
UPDATE legal_review_records
SET decision = 'REJECTED'
WHERE case_id = '10000000-0000-0000-0000-000000000001';
"

# T16(d): Create NEW case with no review record and attempt publish -> MUST FAIL
psql_exec "
INSERT INTO cases (id, slug, title, category, jurisdiction, status, official_briefing, genuine_gaps, created_by, updated_by)
VALUES (
  '92000000-0000-0000-0000-000000000001',
  'test-case-no-review',
  'Unreviewed Case — Must Block Publish',
  'historical_public_record',
  'US-FEDERAL',
  'DRAFT',
  'Draft without legal review',
  '[{\"claim\":\"Unverified date\",\"why_unverified\":\"Archive pending\",\"highest_available_tier\":\"None\"}]'::jsonb,
  '00000000-0000-0000-0000-000000000001',
  '00000000-0000-0000-0000-000000000001'
);
"
assert_sql_fails "T16d" "published_legal_review_id is required|legal review gate" "
UPDATE cases
SET status = 'PUBLISHED'
WHERE slug = 'test-case-no-review';
"
# Clean up temporary T16d case so down migration can delete admin_users cleanly
psql_exec "DELETE FROM cases WHERE id = '92000000-0000-0000-0000-000000000001';"

# T11. Roll back both migrations (down 2)
echo ""
echo "--- [T11] bash scripts/migrate.sh down 2 ---"
bash scripts/migrate.sh down 2
echo "PASS [T11]: Rolled back 2 migrations cleanly."

# T12. Verify all 8 tables are gone
echo ""
echo "--- [T12] Verify all 8 core tables are dropped (expect 0) ---"
REMAINING_TABLES="$(psql_exec "SELECT COUNT(*) FROM pg_tables WHERE schemaname = 'public' AND tablename IN ('cases','evidence_items','case_timeline_events','licensing_records','legal_review_records','audit_log','admin_users','correction_requests');")"
echo "Remaining core table count: ${REMAINING_TABLES}"
if [[ "${REMAINING_TABLES}" != "0" ]]; then
  echo "FAIL [T12]: Expected 0 core tables after down 2, got ${REMAINING_TABLES}." >&2
  exit 1
fi
echo "PASS [T12]: All 8 core tables cleanly removed."

# T13. Re-apply all migrations up (proves down -> up idempotency)
echo ""
echo "--- [T13] bash scripts/migrate.sh up (re-apply all after rollback) ---"
bash scripts/migrate.sh up
bash scripts/migrate.sh status
REAPPLIED_TABLES="$(psql_exec "SELECT COUNT(*) FROM pg_tables WHERE schemaname = 'public' AND tablename IN ('cases','evidence_items','case_timeline_events','licensing_records','legal_review_records','audit_log','admin_users','correction_requests');")"
REAPPLIED_SEED="$(psql_exec "SELECT COUNT(*) FROM cases WHERE slug = 'test-case-001';")"
if [[ "${REAPPLIED_TABLES}" != "8" || "${REAPPLIED_SEED}" != "1" ]]; then
  echo "FAIL [T13]: Expected 8 tables and 1 seed case after re-up, got tables=${REAPPLIED_TABLES}, seed=${REAPPLIED_SEED}." >&2
  exit 1
fi
echo "PASS [T13]: Forward -> Backward -> Forward cycle succeeded cleanly."

# T14. Stop local backing services
echo ""
echo "--- [T14] make services-down ---"
make services-down
echo "PASS [T14]: Local PostgreSQL and Redis stopped cleanly."

echo ""
echo "============================================================================="
echo "ALL MIGRATION & CONSTRAINT TESTS (T1–T16) PASSED!"
echo "============================================================================="
