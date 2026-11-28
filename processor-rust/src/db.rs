use anyhow::{Context, Result};
use serde_json::json;
use sqlx::postgres::PgPoolOptions;
use sqlx::{PgPool, Row};
use uuid::Uuid;

use crate::extract::ExtractedDocument;

/// PostgreSQL persistence layer for `processor-rust`.
///
/// Strictly adheres to the frozen `000001_init.up.sql` schema:
/// - `evidence_items.processing_status` CHECK constraint permits `'PENDING'`, `'COMPLETED'`, `'FAILED'`.
/// - `fn_enforce_legal_review_before_publish()` requires `processing_status = 'COMPLETED'`.
/// - Permanent failure metadata & error reasons are recorded in `audit_log` (append-only) and `evidence.process.dead`.
#[derive(Clone)]
pub struct Db {
    pool: PgPool,
}

impl Db {
    pub async fn connect(database_url: &str) -> Result<Self> {
        let pool = PgPoolOptions::new()
            .max_connections(8)
            .connect(database_url)
            .await
            .context("connect to PostgreSQL")?;
        Ok(Self { pool })
    }

    pub fn from_pool(pool: PgPool) -> Self {
        Self { pool }
    }

    pub fn pool(&self) -> &PgPool {
        &self.pool
    }

    pub async fn ping(&self) -> Result<()> {
        sqlx::query("SELECT 1")
            .execute(&self.pool)
            .await
            .context("postgres health check")?;
        Ok(())
    }

    /// Returns current `processing_status` for an evidence item, or `None` if row does not exist.
    pub async fn get_processing_status(&self, evidence_id: Uuid) -> Result<Option<String>> {
        let row = sqlx::query("SELECT processing_status FROM evidence_items WHERE id = $1")
            .bind(evidence_id)
            .fetch_optional(&self.pool)
            .await
            .context("query evidence processing_status")?;
        Ok(row.map(|r| r.get::<String, _>("processing_status")))
    }

    /// Persists extracted text, page_count, ocr_used, last_processed_job_id, and marks
    /// `processing_status = 'COMPLETED'` (required by the DB publish trigger).
    pub async fn mark_completed(
        &self,
        evidence_id: Uuid,
        job_id: Uuid,
        doc: &ExtractedDocument,
    ) -> Result<()> {
        sqlx::query(
            r#"
            UPDATE evidence_items
            SET
                processing_status = 'COMPLETED',
                last_processed_job_id = $2,
                extracted_text = $3,
                page_count = $4,
                ocr_used = $5,
                updated_at = NOW()
            WHERE id = $1
            "#,
        )
        .bind(evidence_id)
        .bind(job_id)
        .bind(&doc.text)
        .bind(doc.page_count)
        .bind(doc.ocr_used)
        .execute(&self.pool)
        .await
        .context("update evidence_items to COMPLETED")?;
        Ok(())
    }

    /// Marks `evidence_items.processing_status = 'FAILED'` and appends an append-only
    /// `audit_log` row (`action = 'evidence.processing_failed'`) inside a single transaction.
    pub async fn mark_failed_permanent(
        &self,
        evidence_id: Uuid,
        job_id: Uuid,
        reason_code: &str,
        error_reason: &str,
    ) -> Result<()> {
        let mut tx = self
            .pool
            .begin()
            .await
            .context("begin permanent failure transaction")?;

        sqlx::query(
            r#"
            UPDATE evidence_items
            SET
                processing_status = 'FAILED',
                last_processed_job_id = $2,
                updated_at = NOW()
            WHERE id = $1
            "#,
        )
        .bind(evidence_id)
        .bind(job_id)
        .execute(&mut *tx)
        .await
        .context("update evidence_items to FAILED")?;

        let metadata = json!({
            "job_id": job_id.to_string(),
            "status": "FAILED_PERMANENT",
            "error_code": reason_code,
            "error_reason": error_reason,
        });

        sqlx::query(
            r#"
            INSERT INTO audit_log (
                actor_admin_id,
                actor_role,
                action,
                entity_type,
                entity_id,
                reason_code,
                metadata
            ) VALUES (NULL, 'SYSTEM', 'evidence.processing_failed', 'evidence', $1, $2, $3)
            "#,
        )
        .bind(evidence_id)
        .bind(reason_code)
        .bind(metadata)
        .execute(&mut *tx)
        .await
        .context("insert evidence.processing_failed audit_log")?;

        tx.commit()
            .await
            .context("commit permanent failure transaction")?;
        Ok(())
    }

    /// Loads an ADR-019 Meilisearch case document for the parent case of `evidence_id`
    /// ONLY when that parent case has `status = 'PUBLISHED'`. Returns `Ok(None)` for drafts/in-review/retracted cases.
    pub async fn fetch_published_case_search_document_for_evidence(
        &self,
        evidence_id: Uuid,
    ) -> Result<Option<crate::meili::CaseSearchDocument>> {
        let case_row = sqlx::query(
            r#"
            SELECT
                c.id,
                c.slug,
                c.title,
                c.official_briefing,
                c.status,
                c.category,
                COALESCE(to_char(c.published_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '') AS published_at_iso
            FROM cases c
            JOIN evidence_items e ON e.case_id = c.id
            WHERE e.id = $1 AND c.status = 'PUBLISHED'
            LIMIT 1
            "#,
        )
        .bind(evidence_id)
        .fetch_optional(&self.pool)
        .await
        .context("query parent published case for search refresh")?;

        let Some(c) = case_row else {
            return Ok(None);
        };

        let case_id: Uuid = c.get("id");
        let ev_rows = sqlx::query(
            r#"
            SELECT title, extracted_text, source_type, is_redacted
            FROM evidence_items
            WHERE case_id = $1
            ORDER BY created_at ASC, id ASC
            "#,
        )
        .bind(case_id)
        .fetch_all(&self.pool)
        .await
        .context("query case evidence items for search refresh")?;

        let evidence_count = ev_rows.len();
        let mut titles = Vec::new();
        let mut texts = Vec::new();
        let mut source_types = Vec::new();
        for row in ev_rows {
            let is_redacted: bool = row.get("is_redacted");
            if is_redacted {
                continue;
            }
            let title: String = row.get("title");
            if !title.trim().is_empty() {
                titles.push(title);
            }
            let extracted: Option<String> = row.get("extracted_text");
            if let Some(txt) = extracted {
                if !txt.trim().is_empty() {
                    texts.push(txt);
                }
            }
            let st: String = row.get("source_type");
            if !st.is_empty() && !source_types.contains(&st) {
                source_types.push(st);
            }
        }

        Ok(Some(crate::meili::CaseSearchDocument {
            id: case_id.to_string(),
            case_slug: c.get("slug"),
            case_title: c.get("title"),
            case_summary: c.get("official_briefing"),
            status: c.get("status"),
            category: c.get("category"),
            published_at: c.get("published_at_iso"),
            evidence_count,
            evidence_titles: titles,
            evidence_extracted_text: texts,
            evidence_source_type: source_types,
        }))
    }
}
