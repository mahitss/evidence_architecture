use anyhow::{Context, Result};
use chrono::Utc;
use redis::aio::MultiplexedConnection;
use serde::{Deserialize, Serialize};

/// Dead-letter stream for permanently failed evidence processing jobs (LOW_LEVEL_DESIGN.md §3).
pub const EVIDENCE_DLQ_STREAM: &str = "evidence.process.dead";
pub const DLQ_MAX_APPROX_LEN: usize = 5000;

/// Payload pushed to `evidence.process.dead` preserving the full original job fields plus failure details.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct DeadLetterEntry {
    pub original_stream_id: String,
    pub job_id: String,
    pub evidence_id: String,
    pub case_id: String,
    pub storage_key: String,
    pub mime_type: String,
    pub raw_payload: String,
    pub error_code: String,
    pub error_reason: String,
    pub status: String,
    pub failed_at: String,
}

impl DeadLetterEntry {
    #[allow(clippy::too_many_arguments)]
    pub fn new(
        original_stream_id: impl Into<String>,
        job_id: impl Into<String>,
        evidence_id: impl Into<String>,
        case_id: impl Into<String>,
        storage_key: impl Into<String>,
        mime_type: impl Into<String>,
        raw_payload: impl Into<String>,
        error_code: impl Into<String>,
        error_reason: impl Into<String>,
    ) -> Self {
        Self {
            original_stream_id: original_stream_id.into(),
            job_id: job_id.into(),
            evidence_id: evidence_id.into(),
            case_id: case_id.into(),
            storage_key: storage_key.into(),
            mime_type: mime_type.into(),
            raw_payload: raw_payload.into(),
            error_code: error_code.into(),
            error_reason: error_reason.into(),
            status: "FAILED_PERMANENT".to_string(),
            failed_at: Utc::now().to_rfc3339(),
        }
    }
}

/// Appends a dead-letter entry to Redis Stream `evidence.process.dead` with approximate MAXLEN trimming.
pub async fn publish_dead_letter(
    conn: &mut MultiplexedConnection,
    entry: &DeadLetterEntry,
) -> Result<String> {
    let serialized = serde_json::to_string(entry).context("serialize DeadLetterEntry to JSON")?;
    let entry_id: String = redis::cmd("XADD")
        .arg(EVIDENCE_DLQ_STREAM)
        .arg("MAXLEN")
        .arg("~")
        .arg(DLQ_MAX_APPROX_LEN)
        .arg("*")
        .arg("job_id")
        .arg(&entry.job_id)
        .arg("evidence_id")
        .arg(&entry.evidence_id)
        .arg("case_id")
        .arg(&entry.case_id)
        .arg("storage_key")
        .arg(&entry.storage_key)
        .arg("mime_type")
        .arg(&entry.mime_type)
        .arg("error_code")
        .arg(&entry.error_code)
        .arg("error_reason")
        .arg(&entry.error_reason)
        .arg("status")
        .arg(&entry.status)
        .arg("payload")
        .arg(&serialized)
        .query_async(conn)
        .await
        .context("XADD to evidence.process.dead")?;
    Ok(entry_id)
}
