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
}
