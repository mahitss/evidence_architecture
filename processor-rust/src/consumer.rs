use std::collections::HashMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;
use std::time::{Duration, Instant};

use anyhow::{Context, Result};
use redis::aio::MultiplexedConnection;
use redis::streams::StreamReadReply;
use redis::Value;
use serde::{Deserialize, Serialize};
use tokio::sync::watch;
use tracing::{error, info, warn};
use uuid::Uuid;

use crate::db::Db;
use crate::dlq::{publish_dead_letter, DeadLetterEntry};
use crate::error::ProcessorError;
use crate::extract::ExtractedDocument;
use crate::meili::MeiliUpdater;
use crate::processor::DocumentProcessor;

pub const EVIDENCE_PROCESS_STREAM: &str = "evidence.process";
pub const CONSUMER_GROUP_NAME: &str = "processor-workers";
pub const READ_BLOCK_MS: usize = 5000;
pub const MAX_TRANSIENT_RETRIES: u64 = 3;
pub const IN_FLIGHT_LOCK_TTL_SECS: u64 = 300;
pub const RETRY_COUNTER_TTL_SECS: u64 = 86_400;

/// Job payload schema matching `backend-go/internal/queue/queue.go` (`EvidenceProcessingJob`).
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct EvidenceJobPayload {
    pub job_id: String,
    pub evidence_id: Uuid,
    pub storage_key: String,
    pub mime_type: String,
    pub case_id: Uuid,
}

impl EvidenceJobPayload {
    /// Parses either a JSON `"payload"` stream field (Go producer) or top-level stream fields (`redis-cli XADD`).
    pub fn from_stream_fields(fields: &HashMap<String, String>) -> Result<Self, String> {
        if let Some(raw_json) = fields.get("payload") {
            if let Ok(parsed) = serde_json::from_str::<EvidenceJobPayload>(raw_json) {
                return Ok(parsed);
            }
        }

        let job_id = fields
            .get("job_id")
            .cloned()
            .filter(|s| !s.trim().is_empty())
            .ok_or_else(|| "missing job_id".to_string())?;
        let evidence_id_str = fields
            .get("evidence_id")
            .ok_or_else(|| "missing evidence_id".to_string())?;
        let evidence_id = Uuid::parse_str(evidence_id_str.trim())
            .map_err(|e| format!("invalid evidence_id {:?}: {}", evidence_id_str, e))?;
        let storage_key = fields
            .get("storage_key")
            .cloned()
            .filter(|s| !s.trim().is_empty())
            .ok_or_else(|| "missing storage_key".to_string())?;
        let mime_type = fields
            .get("mime_type")
            .cloned()
            .filter(|s| !s.trim().is_empty())
            .ok_or_else(|| "missing mime_type".to_string())?;
        let case_id_str = fields
            .get("case_id")
            .ok_or_else(|| "missing case_id".to_string())?;
        let case_id = Uuid::parse_str(case_id_str.trim())
            .map_err(|e| format!("invalid case_id {:?}: {}", case_id_str, e))?;

        Ok(Self {
            job_id,
            evidence_id,
            storage_key,
            mime_type,
            case_id,
        })
    }

    pub fn job_uuid(&self) -> Uuid {
        Uuid::parse_str(self.job_id.trim()).unwrap_or_else(|_| Uuid::new_v4())
    }
}

/// Outcome returned by processing a single Redis Streams entry.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum JobOutcome {
    Success,
    SkippedDuplicate,
    FailedTemporary,
    FailedPermanent,
}

impl JobOutcome {
    pub fn as_str(&self) -> &'static str {
        match self {
            Self::Success => "SUCCESS",
            Self::SkippedDuplicate => "SKIPPED_DUPLICATE",
            Self::FailedTemporary => "FAILED_TEMPORARY",
            Self::FailedPermanent => "FAILED_PERMANENT",
        }
    }
}

pub type BoxFuture<'a, T> = Pin<Box<dyn Future<Output = T> + Send + 'a>>;

/// Abstraction over Redis + PostgreSQL operations used by the consumer state machine,
/// enabling deterministic unit testing of retry and DLQ behavior with a mock backend.
pub trait ConsumerBackend: Send + Sync {
    fn get_processing_status(&self, evidence_id: Uuid) -> BoxFuture<'_, Result<Option<String>>>;
    fn claim_in_flight(
        &self,
        evidence_id: Uuid,
        consumer_name: &str,
    ) -> BoxFuture<'_, Result<bool>>;
    fn release_in_flight(&self, evidence_id: Uuid) -> BoxFuture<'_, Result<()>>;
    fn mark_completed(
        &self,
        evidence_id: Uuid,
        job_id: Uuid,
        doc: ExtractedDocument,
    ) -> BoxFuture<'_, Result<()>>;
    fn mark_failed_permanent(
        &self,
        evidence_id: Uuid,
        job_id: Uuid,
        reason_code: String,
        error_reason: String,
    ) -> BoxFuture<'_, Result<()>>;
    fn increment_retry(&self, job_id: &str) -> BoxFuture<'_, Result<u64>>;
    fn push_dlq(&self, entry: DeadLetterEntry) -> BoxFuture<'_, Result<()>>;
    fn xack(&self, msg_id: &str) -> BoxFuture<'_, Result<()>>;
}

/// Production backend backed by real local PostgreSQL (`sqlx`), Redis 7 (`redis`),
/// and optional Meilisearch updater for in-place refreshes of published cases.
#[derive(Clone)]
pub struct RedisPostgresBackend {
    db: Db,
    redis_conn: MultiplexedConnection,
    meili: Option<MeiliUpdater>,
}

impl RedisPostgresBackend {
    pub fn new(db: Db, redis_conn: MultiplexedConnection) -> Self {
        Self {
            db,
            redis_conn,
            meili: None,
        }
    }

    pub fn with_meili(mut self, meili: Option<MeiliUpdater>) -> Self {
        self.meili = meili;
        self
    }
}

impl ConsumerBackend for RedisPostgresBackend {
    fn get_processing_status(&self, evidence_id: Uuid) -> BoxFuture<'_, Result<Option<String>>> {
        Box::pin(async move { self.db.get_processing_status(evidence_id).await })
    }

    fn claim_in_flight(
        &self,
        evidence_id: Uuid,
        consumer_name: &str,
    ) -> BoxFuture<'_, Result<bool>> {
        let mut conn = self.redis_conn.clone();
        let owner = consumer_name.to_string();
        Box::pin(async move {
            let key = format!("processing:{}", evidence_id);
            let res: Option<String> = redis::cmd("SET")
                .arg(&key)
                .arg(&owner)
                .arg("NX")
                .arg("EX")
                .arg(IN_FLIGHT_LOCK_TTL_SECS)
                .query_async(&mut conn)
                .await
                .context("SET processing claim key")?;
            Ok(res.is_some())
        })
    }

    fn release_in_flight(&self, evidence_id: Uuid) -> BoxFuture<'_, Result<()>> {
        let mut conn = self.redis_conn.clone();
        Box::pin(async move {
            let key = format!("processing:{}", evidence_id);
            let _: i64 = redis::cmd("DEL")
                .arg(&key)
                .query_async(&mut conn)
                .await
                .context("DEL processing claim key")?;
            Ok(())
        })
    }

    fn mark_completed(
        &self,
        evidence_id: Uuid,
        job_id: Uuid,
        doc: ExtractedDocument,
    ) -> BoxFuture<'_, Result<()>> {
        Box::pin(async move {
            self.db.mark_completed(evidence_id, job_id, &doc).await?;
            if let Some(ref meili) = self.meili {
                match self
                    .db
                    .fetch_published_case_search_document_for_evidence(evidence_id)
                    .await
                {
                    Ok(Some(case_doc)) => {
                        if let Err(err) = meili.upsert_case_document(&case_doc).await {
                            warn!(
                                evidence_id = %evidence_id,
                                case_id = %case_doc.id,
                                error = %err,
                                "failed to refresh published case document in Meilisearch after evidence extraction"
                            );
                        } else {
                            info!(
                                evidence_id = %evidence_id,
                                case_id = %case_doc.id,
                                "refreshed published case document in Meilisearch in place"
                            );
                        }
                    }
                    Ok(None) => {}
                    Err(err) => {
                        warn!(
                            evidence_id = %evidence_id,
                            error = %err,
                            "failed to check parent case publish state for Meilisearch refresh"
                        );
                    }
                }
            }
            Ok(())
        })
    }

    fn mark_failed_permanent(
        &self,
        evidence_id: Uuid,
        job_id: Uuid,
        reason_code: String,
        error_reason: String,
    ) -> BoxFuture<'_, Result<()>> {
        Box::pin(async move {
            self.db
                .mark_failed_permanent(evidence_id, job_id, &reason_code, &error_reason)
                .await
        })
    }

    fn increment_retry(&self, job_id: &str) -> BoxFuture<'_, Result<u64>> {
        let mut conn = self.redis_conn.clone();
        let key = format!("retry:{}", job_id);
        Box::pin(async move {
            let count: u64 = redis::cmd("INCR")
                .arg(&key)
                .query_async(&mut conn)
                .await
                .context("INCR retry counter")?;
            let _: bool = redis::cmd("EXPIRE")
                .arg(&key)
                .arg(RETRY_COUNTER_TTL_SECS)
                .query_async(&mut conn)
                .await
                .context("EXPIRE retry counter")?;
            Ok(count)
        })
    }

    fn push_dlq(&self, entry: DeadLetterEntry) -> BoxFuture<'_, Result<()>> {
        let mut conn = self.redis_conn.clone();
        Box::pin(async move {
            publish_dead_letter(&mut conn, &entry).await?;
            Ok(())
        })
    }

    fn xack(&self, msg_id: &str) -> BoxFuture<'_, Result<()>> {
        let mut conn = self.redis_conn.clone();
        let id = msg_id.to_string();
        Box::pin(async move {
            let _: i64 = redis::cmd("XACK")
                .arg(EVIDENCE_PROCESS_STREAM)
                .arg(CONSUMER_GROUP_NAME)
                .arg(&id)
                .query_async(&mut conn)
                .await
                .context("XACK message")?;
            Ok(())
        })
    }
}

/// Ensures consumer group `processor-workers` exists on stream `evidence.process` (with `MKSTREAM`).
pub async fn ensure_consumer_group(conn: &mut MultiplexedConnection) -> Result<()> {
    let res: redis::RedisResult<String> = redis::cmd("XGROUP")
        .arg("CREATE")
        .arg(EVIDENCE_PROCESS_STREAM)
        .arg(CONSUMER_GROUP_NAME)
        .arg("0")
        .arg("MKSTREAM")
        .query_async(conn)
        .await;

    match res {
        Ok(_) => {
            info!(
                stream = EVIDENCE_PROCESS_STREAM,
                group = CONSUMER_GROUP_NAME,
                "created redis streams consumer group"
            );
            Ok(())
        }
        Err(err) if err.to_string().contains("BUSYGROUP") => Ok(()),
        Err(err) => Err(err).context("XGROUP CREATE processor-workers"),
    }
}

/// Processes a single stream message through idempotency check, claim, extraction, DB write-back,
/// retry tracking, and DLQ routing.
pub async fn process_message<B, F, Fut>(
    backend: &B,
    consumer_name: &str,
    msg_id: &str,
    fields: &HashMap<String, String>,
    extract_fn: F,
) -> Result<JobOutcome>
where
    B: ConsumerBackend,
    F: FnOnce(EvidenceJobPayload) -> Fut,
    Fut: Future<Output = Result<ExtractedDocument, ProcessorError>>,
{
    let started = Instant::now();
    let raw_payload = serde_json::to_string(fields).unwrap_or_else(|_| "{}".to_string());

    let job = match EvidenceJobPayload::from_stream_fields(fields) {
        Ok(j) => j,
        Err(parse_err) => {
            let entry = DeadLetterEntry::new(
                msg_id,
                fields.get("job_id").cloned().unwrap_or_default(),
                fields.get("evidence_id").cloned().unwrap_or_default(),
                fields.get("case_id").cloned().unwrap_or_default(),
                fields.get("storage_key").cloned().unwrap_or_default(),
                fields.get("mime_type").cloned().unwrap_or_default(),
                raw_payload,
                "MALFORMED_PAYLOAD",
                parse_err.clone(),
            );
            backend.push_dlq(entry).await?;
            backend.xack(msg_id).await?;
            warn!(
                msg_id = msg_id,
                duration_ms = started.elapsed().as_millis() as u64,
                result = JobOutcome::FailedPermanent.as_str(),
                error = %parse_err,
                "routed malformed payload to DLQ"
            );
            return Ok(JobOutcome::FailedPermanent);
        }
    };

    let job_uuid = job.job_uuid();

    // Step 2: Idempotency check against evidence_items.processing_status
    match backend.get_processing_status(job.evidence_id).await? {
        None => {
            let entry = DeadLetterEntry::new(
                msg_id,
                &job.job_id,
                job.evidence_id.to_string(),
                job.case_id.to_string(),
                &job.storage_key,
                &job.mime_type,
                raw_payload,
                "EVIDENCE_NOT_FOUND",
                "evidence_items row does not exist",
            );
            backend.push_dlq(entry).await?;
            backend.xack(msg_id).await?;
            warn!(
                job_id = %job.job_id,
                evidence_id = %job.evidence_id,
                duration_ms = started.elapsed().as_millis() as u64,
                result = JobOutcome::FailedPermanent.as_str(),
                "evidence_items row missing; routed to DLQ"
            );
            return Ok(JobOutcome::FailedPermanent);
        }
        Some(status) if status == "COMPLETED" || status == "FAILED" => {
            backend.xack(msg_id).await?;
            info!(
                job_id = %job.job_id,
                evidence_id = %job.evidence_id,
                existing_status = %status,
                duration_ms = started.elapsed().as_millis() as u64,
                result = JobOutcome::SkippedDuplicate.as_str(),
                "skipping already finalized evidence item"
            );
            return Ok(JobOutcome::SkippedDuplicate);
        }
        Some(_) => {}
    }

    // Step 3: Claim in-flight lock so concurrent duplicate deliveries skip cleanly.
    if !backend
        .claim_in_flight(job.evidence_id, consumer_name)
        .await?
    {
        backend.xack(msg_id).await?;
        info!(
            job_id = %job.job_id,
            evidence_id = %job.evidence_id,
            duration_ms = started.elapsed().as_millis() as u64,
            result = JobOutcome::SkippedDuplicate.as_str(),
            "evidence item already claimed by another worker"
        );
        return Ok(JobOutcome::SkippedDuplicate);
    }

    // Step 4 & 5: Fetch file and run extraction / OCR
    match extract_fn(job.clone()).await {
        Ok(doc) => {
            let page_count = doc.page_count;
            let char_count = doc.char_count;
            let ocr_used = doc.ocr_used;
            if let Err(db_err) = backend.mark_completed(job.evidence_id, job_uuid, doc).await {
                let _ = backend.release_in_flight(job.evidence_id).await;
                return handle_transient_failure(
                    backend,
                    msg_id,
                    &job,
                    job_uuid,
                    raw_payload,
                    "DB_WRITE_ERROR",
                    &db_err.to_string(),
                    started,
                )
                .await;
            }
            let _ = backend.release_in_flight(job.evidence_id).await;
            backend.xack(msg_id).await?;
            info!(
                job_id = %job.job_id,
                evidence_id = %job.evidence_id,
                page_count = page_count,
                char_count = char_count,
                ocr_used = ocr_used,
                duration_ms = started.elapsed().as_millis() as u64,
                result = JobOutcome::Success.as_str(),
                "evidence processing completed"
            );
            Ok(JobOutcome::Success)
        }
        Err(proc_err) if proc_err.is_permanent() => {
            let _ = backend.release_in_flight(job.evidence_id).await;
            route_to_dlq(
                backend,
                msg_id,
                &job,
                job_uuid,
                raw_payload,
                proc_err.code(),
                &proc_err.to_string(),
                started,
            )
            .await
        }
        Err(transient_err) => {
            let _ = backend.release_in_flight(job.evidence_id).await;
            handle_transient_failure(
                backend,
                msg_id,
                &job,
                job_uuid,
                raw_payload,
                transient_err.code(),
                &transient_err.to_string(),
                started,
            )
            .await
        }
    }
}

#[allow(clippy::too_many_arguments)]
async fn handle_transient_failure<B: ConsumerBackend>(
    backend: &B,
    msg_id: &str,
    job: &EvidenceJobPayload,
    job_uuid: Uuid,
    raw_payload: String,
    code: &str,
    message: &str,
    started: Instant,
) -> Result<JobOutcome> {
    let retries = backend.increment_retry(&job.job_id).await?;
    if retries > MAX_TRANSIENT_RETRIES {
        return route_to_dlq(
            backend,
            msg_id,
            job,
            job_uuid,
            raw_payload,
            "MAX_RETRIES_EXCEEDED",
            &format!(
                "exceeded {} retries; last error ({}): {}",
                MAX_TRANSIENT_RETRIES, code, message
            ),
            started,
        )
        .await;
    }

    // Do NOT XACK — leave pending in Redis PEL for retry / XAUTOCLAIM.
    warn!(
        job_id = %job.job_id,
        evidence_id = %job.evidence_id,
        retry_count = retries,
        duration_ms = started.elapsed().as_millis() as u64,
        result = JobOutcome::FailedTemporary.as_str(),
        error = %message,
        "transient processing error; leaving unacknowledged for retry"
    );
    Ok(JobOutcome::FailedTemporary)
}

#[allow(clippy::too_many_arguments)]
async fn route_to_dlq<B: ConsumerBackend>(
    backend: &B,
    msg_id: &str,
    job: &EvidenceJobPayload,
    job_uuid: Uuid,
    raw_payload: String,
    code: &str,
    message: &str,
    started: Instant,
) -> Result<JobOutcome> {
    backend
        .mark_failed_permanent(
            job.evidence_id,
            job_uuid,
            code.to_string(),
            message.to_string(),
        )
        .await?;
    let entry = DeadLetterEntry::new(
        msg_id,
        &job.job_id,
        job.evidence_id.to_string(),
        job.case_id.to_string(),
        &job.storage_key,
        &job.mime_type,
        raw_payload,
        code,
        message,
    );
    backend.push_dlq(entry).await?;
    backend.xack(msg_id).await?;
    error!(
        job_id = %job.job_id,
        evidence_id = %job.evidence_id,
        error_code = code,
        duration_ms = started.elapsed().as_millis() as u64,
        result = JobOutcome::FailedPermanent.as_str(),
        error = %message,
        "permanent processing failure; marked FAILED, written to audit_log, and routed to DLQ"
    );
    Ok(JobOutcome::FailedPermanent)
}

/// Runs a single worker consumer loop until `shutdown_rx` signals `true`.
pub async fn run_worker_loop(
    worker_index: usize,
    redis_client: redis::Client,
    db: Db,
    processor: Arc<DocumentProcessor>,
    shutdown_rx: watch::Receiver<bool>,
) -> Result<()> {
    run_worker_loop_with_meili(worker_index, redis_client, db, processor, None, shutdown_rx).await
}

/// Runs a single worker consumer loop with optional Meilisearch in-place updates on published cases.
pub async fn run_worker_loop_with_meili(
    worker_index: usize,
    redis_client: redis::Client,
    db: Db,
    processor: Arc<DocumentProcessor>,
    meili: Option<MeiliUpdater>,
    mut shutdown_rx: watch::Receiver<bool>,
) -> Result<()> {
    let hostname = std::env::var("HOSTNAME").unwrap_or_else(|_| "local".to_string());
    let consumer_name = format!("{}-{}-{}", hostname, std::process::id(), worker_index);

    let mut conn = redis_client
        .get_multiplexed_async_connection()
        .await
        .context("connect worker to Redis")?;
    ensure_consumer_group(&mut conn).await?;

    let backend = RedisPostgresBackend::new(db, conn.clone()).with_meili(meili);

    info!(
        consumer = %consumer_name,
        stream = EVIDENCE_PROCESS_STREAM,
        group = CONSUMER_GROUP_NAME,
        "worker loop started"
    );

    loop {
        if *shutdown_rx.borrow() {
            info!(consumer = %consumer_name, "worker loop shutting down");
            break;
        }

        let read_fut = async {
            let reply: redis::RedisResult<Option<StreamReadReply>> = redis::cmd("XREADGROUP")
                .arg("GROUP")
                .arg(CONSUMER_GROUP_NAME)
                .arg(&consumer_name)
                .arg("COUNT")
                .arg(1)
                .arg("BLOCK")
                .arg(READ_BLOCK_MS)
                .arg("STREAMS")
                .arg(EVIDENCE_PROCESS_STREAM)
                .arg(">")
                .query_async(&mut conn)
                .await;
            reply
        };

        let reply = tokio::select! {
            _ = shutdown_rx.changed() => {
                if *shutdown_rx.borrow() {
                    info!(consumer = %consumer_name, "worker loop received shutdown signal");
                    break;
                }
                continue;
            }
            res = read_fut => res,
        };

        match reply {
            Ok(Some(stream_reply)) => {
                for key in stream_reply.keys {
                    for entry in key.ids {
                        let fields = extract_string_map(&entry.map);
                        let proc_ref = Arc::clone(&processor);
                        if let Err(err) = process_message(
                            &backend,
                            &consumer_name,
                            &entry.id,
                            &fields,
                            |job| async move {
                                let (doc, _sha) = proc_ref
                                    .process_file(&job.storage_key, &job.mime_type)
                                    .await?;
                                Ok(doc)
                            },
                        )
                        .await
                        {
                            error!(
                                consumer = %consumer_name,
                                msg_id = %entry.id,
                                error = %err,
                                "unexpected backend error handling stream message"
                            );
                        }
                    }
                }
            }
            Ok(None) => {}
            Err(err) => {
                error!(
                    consumer = %consumer_name,
                    error = %err,
                    "XREADGROUP error; backing off 1s before retry"
                );
                tokio::time::sleep(Duration::from_secs(1)).await;
            }
        }
    }

    Ok(())
}

fn extract_string_map(map: &HashMap<String, Value>) -> HashMap<String, String> {
    let mut out = HashMap::with_capacity(map.len());
    for (k, v) in map {
        if let Some(s) = redis_value_to_string(v) {
            out.insert(k.clone(), s);
        }
    }
    out
}

fn redis_value_to_string(val: &Value) -> Option<String> {
    match val {
        Value::BulkString(bytes) => String::from_utf8(bytes.clone()).ok(),
        Value::SimpleString(s) => Some(s.clone()),
        Value::Int(i) => Some(i.to_string()),
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::sync::Mutex;

    #[derive(Default)]
    struct MockState {
        statuses: HashMap<Uuid, String>,
        in_flight: HashMap<Uuid, String>,
        retries: HashMap<String, u64>,
        dlq: Vec<DeadLetterEntry>,
        acked: Vec<String>,
        completed: Vec<(Uuid, ExtractedDocument)>,
        failed_permanent: Vec<(Uuid, String, String)>,
    }

    struct MockBackend {
        state: Mutex<MockState>,
    }

    impl MockBackend {
        fn with_pending(evidence_id: Uuid) -> Self {
            let mut st = MockState::default();
            st.statuses.insert(evidence_id, "PENDING".to_string());
            Self {
                state: Mutex::new(st),
            }
        }
    }

    impl ConsumerBackend for MockBackend {
        fn get_processing_status(
            &self,
            evidence_id: Uuid,
        ) -> BoxFuture<'_, Result<Option<String>>> {
            Box::pin(async move { Ok(self.state.lock().await.statuses.get(&evidence_id).cloned()) })
        }

        fn claim_in_flight(
            &self,
            evidence_id: Uuid,
            consumer_name: &str,
        ) -> BoxFuture<'_, Result<bool>> {
            let owner = consumer_name.to_string();
            Box::pin(async move {
                let mut st = self.state.lock().await;
                if st.in_flight.contains_key(&evidence_id) {
                    return Ok(false);
                }
                st.in_flight.insert(evidence_id, owner);
                Ok(true)
            })
        }

        fn release_in_flight(&self, evidence_id: Uuid) -> BoxFuture<'_, Result<()>> {
            Box::pin(async move {
                self.state.lock().await.in_flight.remove(&evidence_id);
                Ok(())
            })
        }

        fn mark_completed(
            &self,
            evidence_id: Uuid,
            _job_id: Uuid,
            doc: ExtractedDocument,
        ) -> BoxFuture<'_, Result<()>> {
            Box::pin(async move {
                let mut st = self.state.lock().await;
                st.statuses.insert(evidence_id, "COMPLETED".to_string());
                st.completed.push((evidence_id, doc));
                Ok(())
            })
        }

        fn mark_failed_permanent(
            &self,
            evidence_id: Uuid,
            _job_id: Uuid,
            reason_code: String,
            error_reason: String,
        ) -> BoxFuture<'_, Result<()>> {
            Box::pin(async move {
                let mut st = self.state.lock().await;
                st.statuses.insert(evidence_id, "FAILED".to_string());
                st.failed_permanent
                    .push((evidence_id, reason_code, error_reason));
                Ok(())
            })
        }

        fn increment_retry(&self, job_id: &str) -> BoxFuture<'_, Result<u64>> {
            let key = job_id.to_string();
            Box::pin(async move {
                let mut st = self.state.lock().await;
                let entry = st.retries.entry(key).or_insert(0);
                *entry += 1;
                Ok(*entry)
            })
        }

        fn push_dlq(&self, entry: DeadLetterEntry) -> BoxFuture<'_, Result<()>> {
            Box::pin(async move {
                self.state.lock().await.dlq.push(entry);
                Ok(())
            })
        }

        fn xack(&self, msg_id: &str) -> BoxFuture<'_, Result<()>> {
            let id = msg_id.to_string();
            Box::pin(async move {
                self.state.lock().await.acked.push(id);
                Ok(())
            })
        }
    }

    fn sample_fields(job_id: Uuid, evidence_id: Uuid, case_id: Uuid) -> HashMap<String, String> {
        let payload = EvidenceJobPayload {
            job_id: job_id.to_string(),
            evidence_id,
            storage_key: "cases/test/exhibit.pdf".to_string(),
            mime_type: "application/pdf".to_string(),
            case_id,
        };
        let mut map = HashMap::new();
        map.insert(
            "payload".to_string(),
            serde_json::to_string(&payload).unwrap(),
        );
        map
    }

    #[tokio::test]
    async fn transient_error_retries_three_times_then_routes_to_dlq() {
        let evidence_id = Uuid::new_v4();
        let job_id = Uuid::new_v4();
        let case_id = Uuid::new_v4();
        let backend = MockBackend::with_pending(evidence_id);
        let fields = sample_fields(job_id, evidence_id, case_id);

        // Attempts 1, 2, 3: transient failure -> NOT acked, NOT in DLQ, still PENDING
        for attempt in 1..=3 {
            let outcome = process_message(&backend, "worker-1", "1-0", &fields, |_| async {
                Err(ProcessorError::transient(
                    "STORAGE_IO_ERROR",
                    "temporary disk stall",
                ))
            })
            .await
            .unwrap();
            assert_eq!(outcome, JobOutcome::FailedTemporary, "attempt {}", attempt);
            let st = backend.state.lock().await;
            assert!(st.acked.is_empty(), "must not XACK transient failure");
            assert!(st.dlq.is_empty(), "must not DLQ before retry > 3");
            assert_eq!(st.statuses.get(&evidence_id).unwrap(), "PENDING");
        }

        // Attempt 4 (retry count = 4 > 3): moves to DLQ, marks FAILED, and XACKs
        let outcome = process_message(&backend, "worker-1", "1-0", &fields, |_| async {
            Err(ProcessorError::transient(
                "STORAGE_IO_ERROR",
                "temporary disk stall",
            ))
        })
        .await
        .unwrap();
        assert_eq!(outcome, JobOutcome::FailedPermanent);
        let st = backend.state.lock().await;
        assert_eq!(st.acked, vec!["1-0"]);
        assert_eq!(st.dlq.len(), 1);
        assert_eq!(st.dlq[0].error_code, "MAX_RETRIES_EXCEEDED");
        assert_eq!(st.statuses.get(&evidence_id).unwrap(), "FAILED");
    }

    #[tokio::test]
    async fn permanent_error_immediately_routes_to_dlq_and_xacks() {
        let evidence_id = Uuid::new_v4();
        let job_id = Uuid::new_v4();
        let case_id = Uuid::new_v4();
        let backend = MockBackend::with_pending(evidence_id);
        let fields = sample_fields(job_id, evidence_id, case_id);

        let outcome = process_message(&backend, "worker-1", "2-0", &fields, |_| async {
            Err(ProcessorError::permanent("CORRUPT_PDF", "invalid header"))
        })
        .await
        .unwrap();

        assert_eq!(outcome, JobOutcome::FailedPermanent);
        let st = backend.state.lock().await;
        assert_eq!(st.acked, vec!["2-0"]);
        assert_eq!(st.dlq.len(), 1);
        assert_eq!(st.dlq[0].error_code, "CORRUPT_PDF");
        assert_eq!(st.statuses.get(&evidence_id).unwrap(), "FAILED");
        assert_eq!(st.failed_permanent.len(), 1);
    }

    #[tokio::test]
    async fn already_completed_evidence_is_deduped_and_xacked() {
        let evidence_id = Uuid::new_v4();
        let job_id = Uuid::new_v4();
        let case_id = Uuid::new_v4();
        let backend = MockBackend::with_pending(evidence_id);
        backend
            .state
            .lock()
            .await
            .statuses
            .insert(evidence_id, "COMPLETED".to_string());
        let fields = sample_fields(job_id, evidence_id, case_id);

        let outcome = process_message(&backend, "worker-1", "3-0", &fields, |_| async {
            panic!("extract_fn must not run for already COMPLETED evidence");
        })
        .await
        .unwrap();

        assert_eq!(outcome, JobOutcome::SkippedDuplicate);
        assert_eq!(backend.state.lock().await.acked, vec!["3-0"]);
    }
}
