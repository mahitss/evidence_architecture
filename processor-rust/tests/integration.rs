use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::Arc;
use std::time::{Duration, Instant};

use chrono::Utc;
use redis::Value;
use serde_json::json;
use sha2::{Digest, Sha256};
use sqlx::Row;
use tempfile::TempDir;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpStream;
use tokio::sync::watch;
use uuid::Uuid;

use processor_rust::consumer::{
    ensure_consumer_group, run_worker_loop, EvidenceJobPayload, EVIDENCE_PROCESS_STREAM,
};
use processor_rust::db::Db;
use processor_rust::dlq::EVIDENCE_DLQ_STREAM;
use processor_rust::health::serve_health;
use processor_rust::ocr;
use processor_rust::processor::DocumentProcessor;

const SEED_ADMIN_ID: &str = "00000000-0000-0000-0000-000000000001";
const DEFAULT_DB_URL: &str = "postgres://postgres@127.0.0.1:5432/truecrime_dev?sslmode=disable";
const DEFAULT_REDIS_URL: &str = "redis://127.0.0.1:6379/0";

fn fixture_path(name: &str) -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("tests")
        .join("fixtures")
        .join(name)
}

fn sha256_hex(bytes: &[u8]) -> String {
    let mut h = Sha256::new();
    h.update(bytes);
    format!("{:x}", h.finalize())
}

async fn insert_test_case(db: &Db, slug: &str) -> Uuid {
    let admin_id = Uuid::parse_str(SEED_ADMIN_ID).expect("valid seed admin uuid");
    let case_id = Uuid::new_v4();
    sqlx::query(
        r#"
        INSERT INTO cases (
            id, slug, title, category, jurisdiction, status,
            official_briefing, genuine_gaps, created_by, updated_by
        )
        VALUES ($1, $2, $3, 'financial_crime', 'US-SDNY', 'DRAFT',
                'Rust integration test dossier', '[{"question":"Pending verification"}]'::jsonb,
                $4, $4)
        "#,
    )
    .bind(case_id)
    .bind(slug)
    .bind(format!("Rust Processor Test Case {}", slug))
    .bind(admin_id)
    .execute(db.pool())
    .await
    .expect("insert test case");
    case_id
}

async fn insert_pending_evidence(
    db: &Db,
    case_id: Uuid,
    title: &str,
    storage_key: &str,
    bytes: &[u8],
) -> Uuid {
    let evidence_id = Uuid::new_v4();
    let hash = sha256_hex(bytes);
    sqlx::query(
        r#"
        INSERT INTO evidence_items (
            id, case_id, title, source_type, verification_tier, license_status,
            source_citation, issuing_authority, license_basis_note,
            mime_type, byte_size, sha256_hash, object_key,
            processing_status, retrieved_at
        )
        VALUES (
            $1, $2, $3, 'gov_primary_record', 1, 'PUBLIC_DOMAIN',
            'DOJ SDNY Public Filing', 'US DOJ', '17 U.S.C. § 105',
            'application/pdf', $4, $5, $6,
            'PENDING', $7
        )
        "#,
    )
    .bind(evidence_id)
    .bind(case_id)
    .bind(title)
    .bind(bytes.len() as i64)
    .bind(hash)
    .bind(storage_key)
    .bind(Utc::now())
    .execute(db.pool())
    .await
    .expect("insert pending evidence_items row");
    evidence_id
}

async fn publish_job(
    conn: &mut redis::aio::MultiplexedConnection,
    job: &EvidenceJobPayload,
) -> String {
    let payload_json = serde_json::to_string(job).expect("serialize job");
    let stream_id: String = redis::cmd("XADD")
        .arg(EVIDENCE_PROCESS_STREAM)
        .arg("*")
        .arg("payload")
        .arg(&payload_json)
        .arg("job_id")
        .arg(&job.job_id)
        .arg("evidence_id")
        .arg(job.evidence_id.to_string())
        .arg("storage_key")
        .arg(&job.storage_key)
        .arg("mime_type")
        .arg(&job.mime_type)
        .arg("case_id")
        .arg(job.case_id.to_string())
        .query_async(conn)
        .await
        .expect("XADD evidence.process");
    stream_id
}

async fn wait_for_status(db: &Db, evidence_id: Uuid, target: &str, timeout: Duration) {
    let deadline = Instant::now() + timeout;
    loop {
        let status = db
            .get_processing_status(evidence_id)
            .await
            .expect("query status");
        if status.as_deref() == Some(target) {
            return;
        }
        assert!(
            Instant::now() < deadline,
            "timed out waiting for evidence {} to reach status {:?} (current: {:?})",
            evidence_id,
            target,
            status
        );
        tokio::time::sleep(Duration::from_millis(100)).await;
    }
}

fn parse_dlq_entries(val: Value) -> Vec<HashMap<String, String>> {
    let mut results = Vec::new();
    if let Value::Array(entries) = val {
        for entry in entries {
            if let Value::Array(parts) = entry {
                if parts.len() == 2 {
                    if let Value::Array(kvs) = &parts[1] {
                        let mut map = HashMap::new();
                        let mut iter = kvs.iter();
                        while let (Some(k), Some(v)) = (iter.next(), iter.next()) {
                            if let (Value::BulkString(kb), Value::BulkString(vb)) = (k, v) {
                                if let (Ok(ks), Ok(vs)) =
                                    (String::from_utf8(kb.clone()), String::from_utf8(vb.clone()))
                                {
                                    map.insert(ks, vs);
                                }
                            }
                        }
                        results.push(map);
                    }
                }
            }
        }
    }
    results
}

#[tokio::test]
async fn end_to_end_digital_pdf_corrupt_pdf_and_health_against_real_postgres_and_redis() {
    let db_url = std::env::var("DATABASE_URL").unwrap_or_else(|_| DEFAULT_DB_URL.to_string());
    let redis_url = std::env::var("REDIS_URL").unwrap_or_else(|_| DEFAULT_REDIS_URL.to_string());

    let db = Db::connect(&db_url)
        .await
        .expect("connect to local PostgreSQL (start via `make services-up`)");
    let redis_client = redis::Client::open(redis_url.as_str()).expect("open Redis client");
    let mut redis_conn = redis_client
        .get_multiplexed_async_connection()
        .await
        .expect("connect to local Redis (start via `make services-up`)");

    // Ensure consumer group exists before publishing test jobs so `>` receives our new messages.
    ensure_consumer_group(&mut redis_conn)
        .await
        .expect("ensure consumer group");

    let storage_dir = TempDir::new().expect("create temp storage dir");
    let processor = Arc::new(DocumentProcessor::new(
        storage_dir.path().to_path_buf(),
        !ocr::is_ocr_disabled(),
    ));

    let (shutdown_tx, shutdown_rx) = watch::channel(false);

    // Start worker loop in-process
    let worker_handle = tokio::spawn(run_worker_loop(
        0,
        redis_client.clone(),
        db.clone(),
        Arc::clone(&processor),
        shutdown_rx.clone(),
    ));

    // Start health endpoint on dedicated local test port
    let health_addr = "127.0.0.1:19091".to_string();
    let health_handle = tokio::spawn(serve_health(
        health_addr.clone(),
        redis_client.clone(),
        db.clone(),
        shutdown_rx.clone(),
    ));

    // Allow listener and XREADGROUP loop to initialize
    tokio::time::sleep(Duration::from_millis(250)).await;

    // -------------------------------------------------------------------------
    // 1. Health endpoint probe
    // -------------------------------------------------------------------------
    {
        let mut stream = TcpStream::connect(&health_addr)
            .await
            .expect("connect to health server");
        stream
            .write_all(b"GET /health HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: close\r\n\r\n")
            .await
            .expect("write health request");
        let mut buf = Vec::new();
        stream
            .read_to_end(&mut buf)
            .await
            .expect("read health response");
        let resp = String::from_utf8_lossy(&buf);
        assert!(
            resp.starts_with("HTTP/1.1 200 OK"),
            "expected HTTP 200 from /health, got: {}",
            resp
        );
        let body = resp.split("\r\n\r\n").nth(1).unwrap_or("");
        let parsed: serde_json::Value = serde_json::from_str(body).expect("parse health JSON");
        let expected_tess = if ocr::is_ocr_disabled() {
            "missing"
        } else {
            "available"
        };
        assert_eq!(
            parsed,
            json!({
                "status": "ok",
                "redis": "ok",
                "db": "ok",
                "tesseract": expected_tess
            })
        );
    }

    // -------------------------------------------------------------------------
    // 2. Digital PDF end-to-end (PENDING -> COMPLETED with extracted_text & page_count == 2)
    // -------------------------------------------------------------------------
    let run_suffix = Uuid::new_v4().simple().to_string();
    let slug = format!("rust-proc-it-{}", &run_suffix[..8]);
    let case_id = insert_test_case(&db, &slug).await;

    let digital_bytes = std::fs::read(fixture_path("digital-text.pdf")).expect("read digital PDF");
    let digital_key = format!("cases/{}/digital-exhibit-{}.pdf", case_id, &run_suffix[..8]);
    let digital_dst = storage_dir.path().join(&digital_key);
    std::fs::create_dir_all(digital_dst.parent().unwrap()).expect("mkdir case storage");
    std::fs::write(&digital_dst, &digital_bytes).expect("write digital PDF to storage");

    let digital_evidence_id = insert_pending_evidence(
        &db,
        case_id,
        "DOJ Indictment Digital Exhibit",
        &digital_key,
        &digital_bytes,
    )
    .await;

    let digital_job_id = Uuid::new_v4();
    let digital_job = EvidenceJobPayload {
        job_id: digital_job_id.to_string(),
        evidence_id: digital_evidence_id,
        storage_key: digital_key.clone(),
        mime_type: "application/pdf".to_string(),
        case_id,
    };
    publish_job(&mut redis_conn, &digital_job).await;

    wait_for_status(
        &db,
        digital_evidence_id,
        "COMPLETED",
        Duration::from_secs(10),
    )
    .await;

    let row = sqlx::query(
        r#"
        SELECT processing_status, last_processed_job_id, extracted_text, page_count, ocr_used
        FROM evidence_items
        WHERE id = $1
        "#,
    )
    .bind(digital_evidence_id)
    .fetch_one(db.pool())
    .await
    .expect("fetch completed digital evidence row");

    let extracted_text: String = row.get("extracted_text");
    let page_count: i32 = row.get("page_count");
    let ocr_used: bool = row.get("ocr_used");
    let last_job: Uuid = row.get("last_processed_job_id");

    assert_eq!(last_job, digital_job_id);
    assert_eq!(page_count, 2);
    assert!(!ocr_used, "digital PDF must not require OCR");
    assert!(
        extracted_text.contains("UNITED STATES DISTRICT COURT"),
        "expected page 1 text, got: {:?}",
        extracted_text
    );
    assert!(
        extracted_text.contains("Wire Transfer Ledger"),
        "expected page 2 text, got: {:?}",
        extracted_text
    );

    // -------------------------------------------------------------------------
    // 3. Corrupt PDF (.txt renamed to .pdf) -> FAILED + DLQ + audit_log
    // -------------------------------------------------------------------------
    let corrupt_bytes = b"This is a plain text document masquerading as a PDF file.\n".to_vec();
    let corrupt_key = format!("cases/{}/corrupt-exhibit-{}.pdf", case_id, &run_suffix[..8]);
    let corrupt_dst = storage_dir.path().join(&corrupt_key);
    std::fs::write(&corrupt_dst, &corrupt_bytes).expect("write corrupt PDF to storage");

    let corrupt_evidence_id = insert_pending_evidence(
        &db,
        case_id,
        "Corrupt PDF Exhibit",
        &corrupt_key,
        &corrupt_bytes,
    )
    .await;

    let corrupt_job_id = Uuid::new_v4();
    let corrupt_job = EvidenceJobPayload {
        job_id: corrupt_job_id.to_string(),
        evidence_id: corrupt_evidence_id,
        storage_key: corrupt_key.clone(),
        mime_type: "application/pdf".to_string(),
        case_id,
    };
    publish_job(&mut redis_conn, &corrupt_job).await;

    wait_for_status(&db, corrupt_evidence_id, "FAILED", Duration::from_secs(10)).await;

    // Verify DLQ entry in evidence.process.dead
    let dlq_raw: Value = redis::cmd("XRANGE")
        .arg(EVIDENCE_DLQ_STREAM)
        .arg("-")
        .arg("+")
        .query_async(&mut redis_conn)
        .await
        .expect("XRANGE evidence.process.dead");
    let dlq_entries = parse_dlq_entries(dlq_raw);
    let matching_dlq = dlq_entries.iter().find(|m| {
        m.get("job_id").map(String::as_str) == Some(corrupt_job_id.to_string().as_str())
            && m.get("evidence_id").map(String::as_str)
                == Some(corrupt_evidence_id.to_string().as_str())
    });
    let dlq_entry = matching_dlq.expect("expected matching DLQ entry in evidence.process.dead");
    assert_eq!(
        dlq_entry.get("status").map(String::as_str),
        Some("FAILED_PERMANENT")
    );
    assert_eq!(
        dlq_entry.get("error_code").map(String::as_str),
        Some("CORRUPT_PDF")
    );

    // Verify append-only audit_log row recorded
    let audit_row = sqlx::query(
        r#"
        SELECT actor_role, action, entity_type, reason_code
        FROM audit_log
        WHERE entity_id = $1 AND action = 'evidence.processing_failed'
        ORDER BY occurred_at DESC
        LIMIT 1
        "#,
    )
    .bind(corrupt_evidence_id)
    .fetch_one(db.pool())
    .await
    .expect("expected evidence.processing_failed row in audit_log");

    assert_eq!(audit_row.get::<String, _>("actor_role"), "SYSTEM");
    assert_eq!(audit_row.get::<String, _>("entity_type"), "evidence");
    assert_eq!(
        audit_row.get::<Option<String>, _>("reason_code").as_deref(),
        Some("CORRUPT_PDF")
    );

    // -------------------------------------------------------------------------
    // Cleanup: graceful shutdown + remove test evidence_items and cases rows
    // (audit_log is append-only by DB trigger and intentionally retained)
    // -------------------------------------------------------------------------
    let _ = shutdown_tx.send(true);
    let _ = tokio::time::timeout(Duration::from_secs(6), worker_handle).await;
    let _ = tokio::time::timeout(Duration::from_secs(3), health_handle).await;

    sqlx::query("DELETE FROM evidence_items WHERE case_id = $1")
        .bind(case_id)
        .execute(db.pool())
        .await
        .expect("cleanup test evidence_items");
    sqlx::query("DELETE FROM cases WHERE id = $1")
        .bind(case_id)
        .execute(db.pool())
        .await
        .expect("cleanup test case");
}
