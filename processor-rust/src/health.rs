use anyhow::{Context, Result};
use serde_json::json;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpListener;
use tokio::sync::watch;
use tracing::{info, warn};

use crate::db::Db;
use crate::ocr;

/// Lightweight HTTP/1.1 health probe server listening on `addr` (default `127.0.0.1:9091`).
///
/// Serves `GET /health` returning:
/// `{"status":"ok","redis":"ok"|"down","db":"ok"|"down","tesseract":"available"|"missing"}`
pub async fn serve_health(
    addr: String,
    redis_client: redis::Client,
    db: Db,
    mut shutdown_rx: watch::Receiver<bool>,
) -> Result<()> {
    let listener = TcpListener::bind(&addr)
        .await
        .with_context(|| format!("bind health server on {}", addr))?;
    info!(health_addr = %addr, "health HTTP server listening");

    loop {
        let accept_res = tokio::select! {
            _ = shutdown_rx.changed() => {
                if *shutdown_rx.borrow() {
                    info!("health HTTP server shutting down");
                    break;
                }
                continue;
            }
            res = listener.accept() => res,
        };

        let (mut socket, _) = match accept_res {
            Ok(pair) => pair,
            Err(err) => {
                warn!(error = %err, "health accept error");
                continue;
            }
        };

        let rc = redis_client.clone();
        let db_clone = db.clone();
        tokio::spawn(async move {
            let mut req_buf = [0u8; 1024];
            let n = match socket.read(&mut req_buf).await {
                Ok(n) if n > 0 => n,
                _ => return,
            };
            let req_line = String::from_utf8_lossy(&req_buf[..n]);
            let is_health = req_line.starts_with("GET /health ")
                || req_line.starts_with("GET /health?")
                || req_line.starts_with("GET /health\r\n");

            if !is_health {
                let body = r#"{"error":"not found"}"#;
                let resp = format!(
                    "HTTP/1.1 404 Not Found\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                    body.len(),
                    body
                );
                let _ = socket.write_all(resp.as_bytes()).await;
                return;
            }

            let redis_ok = check_redis(&rc).await;
            let db_ok = db_clone.ping().await.is_ok();
            let tesseract_state = if ocr::is_ocr_disabled() {
                "missing"
            } else {
                "available"
            };
            let overall_ok = redis_ok && db_ok;

            let body = json!({
                "status": if overall_ok { "ok" } else { "degraded" },
                "redis": if redis_ok { "ok" } else { "down" },
                "db": if db_ok { "ok" } else { "down" },
                "tesseract": tesseract_state,
            })
            .to_string();

            let status_line = if overall_ok {
                "HTTP/1.1 200 OK"
            } else {
                "HTTP/1.1 503 Service Unavailable"
            };
            let resp = format!(
                "{}\r\nContent-Type: application/json\r\nX-Content-Type-Options: nosniff\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                status_line,
                body.len(),
                body
            );
            let _ = socket.write_all(resp.as_bytes()).await;
        });
    }

    Ok(())
}

async fn check_redis(client: &redis::Client) -> bool {
    let Ok(mut conn) = client.get_multiplexed_async_connection().await else {
        return false;
    };
    let pong: redis::RedisResult<String> = redis::cmd("PING").query_async(&mut conn).await;
    matches!(pong.as_deref(), Ok("PONG"))
}
