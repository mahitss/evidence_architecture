use anyhow::{bail, Context, Result};
use serde::Serialize;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpStream;
use tokio::time::{timeout, Duration};
use url::Url;

pub const DEFAULT_MEILI_INDEX: &str = "cases";

/// ADR-019 Meilisearch document schema (kept byte-for-byte compatible with Go `search.CaseDocument`).
#[derive(Debug, Clone, Serialize)]
pub struct CaseSearchDocument {
    pub id: String,
    pub case_slug: String,
    pub case_title: String,
    pub case_summary: String,
    pub status: String,
    pub category: String,
    pub published_at: String,
    pub evidence_count: usize,
    pub evidence_titles: Vec<String>,
    pub evidence_extracted_text: Vec<String>,
    pub evidence_source_type: Vec<String>,
}

/// Lightweight loopback-only HTTP/1.1 client for refreshing published case documents in Meilisearch
/// after asynchronous evidence extraction completes (no extra runtime crate dependencies).
#[derive(Debug, Clone)]
pub struct MeiliUpdater {
    host_port: String,
    api_key: Option<String>,
    index_name: String,
}

impl MeiliUpdater {
    pub fn new(
        base_url: &str,
        api_key: Option<String>,
        index_name: Option<String>,
    ) -> Result<Self> {
        let parsed = Url::parse(base_url).context("parse MEILI_URL")?;
        let host = parsed.host_str().unwrap_or("127.0.0.1");
        let port = parsed.port().unwrap_or(7700);
        let idx = index_name
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
            .unwrap_or_else(|| DEFAULT_MEILI_INDEX.to_string());

        Ok(Self {
            host_port: format!("{}:{}", host, port),
            api_key: api_key
                .map(|s| s.trim().to_string())
                .filter(|s| !s.is_empty()),
            index_name: idx,
        })
    }

    /// Upserts a single published case document into Meilisearch (`POST /indexes/{index}/documents?primaryKey=id`).
    pub async fn upsert_case_document(&self, doc: &CaseSearchDocument) -> Result<()> {
        let body = serde_json::to_string(&[doc]).context("serialize meilisearch document")?;
        let path = format!("/indexes/{}/documents?primaryKey=id", self.index_name);

        let mut req = format!(
            "POST {} HTTP/1.1\r\nHost: {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n",
            path,
            self.host_port,
            body.len()
        );
        if let Some(ref key) = self.api_key {
            req.push_str(&format!("Authorization: Bearer {}\r\n", key));
        }
        req.push_str("\r\n");
        req.push_str(&body);

        let mut stream = timeout(Duration::from_secs(3), TcpStream::connect(&self.host_port))
            .await
            .context("timed out connecting to Meilisearch")?
            .context("connect to Meilisearch")?;

        timeout(Duration::from_secs(3), stream.write_all(req.as_bytes()))
            .await
            .context("timed out writing Meilisearch request")?
            .context("write Meilisearch request")?;

        let mut resp_bytes = Vec::with_capacity(512);
        timeout(Duration::from_secs(5), stream.read_to_end(&mut resp_bytes))
            .await
            .context("timed out reading Meilisearch response")?
            .context("read Meilisearch response")?;

        let resp_str = String::from_utf8_lossy(&resp_bytes);
        let status_line = resp_str.lines().next().unwrap_or_default();
        if !status_line.contains(" 200 ") && !status_line.contains(" 202 ") {
            bail!("unexpected Meilisearch HTTP response: {}", status_line);
        }
        Ok(())
    }
}
