use std::env;
use std::path::PathBuf;

use anyhow::{bail, Context, Result};
use url::Url;

pub const DEFAULT_REDIS_URL: &str = "redis://127.0.0.1:6379/0";
pub const DEFAULT_STORAGE_ROOT: &str = "./data/evidence";
pub const DEFAULT_LOG_LEVEL: &str = "info";
pub const DEFAULT_WORKER_COUNT: usize = 2;
pub const DEFAULT_HEALTH_ADDR: &str = "127.0.0.1:9091";
pub const REQUIRED_LOOPBACK_HOST: &str = "127.0.0.1";

#[derive(Debug, Clone)]
pub struct Config {
    pub redis_url: String,
    pub database_url: String,
    pub storage_root: PathBuf,
    pub log_level: String,
    pub worker_count: usize,
    pub health_addr: String,
    pub meili_url: Option<String>,
    pub meili_api_key: Option<String>,
    pub meili_index: Option<String>,
}

impl Config {
    /// Load and validate runtime configuration from process environment variables.
    pub fn load() -> Result<Self> {
        Self::load_from(|k| env::var(k).ok())
    }

    /// Load and validate configuration from a custom lookup closure (unit-testable, no global mutation).
    pub fn load_from<F>(lookup: F) -> Result<Self>
    where
        F: Fn(&str) -> Option<String>,
    {
        let database_url = lookup("DATABASE_URL")
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
            .ok_or_else(|| anyhow::anyhow!("DATABASE_URL is required"))?;
        validate_loopback_url("DATABASE_URL", &database_url, &["postgres", "postgresql"])?;

        let redis_url = lookup("REDIS_URL")
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
            .unwrap_or_else(|| DEFAULT_REDIS_URL.to_string());
        validate_loopback_url("REDIS_URL", &redis_url, &["redis", "rediss"])?;

        let storage_root = lookup("STORAGE_ROOT")
            .or_else(|| lookup("LOCAL_STORAGE_ROOT"))
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
            .unwrap_or_else(|| DEFAULT_STORAGE_ROOT.to_string());

        let log_level = lookup("LOG_LEVEL")
            .map(|s| s.trim().to_lowercase())
            .filter(|s| !s.is_empty())
            .unwrap_or_else(|| DEFAULT_LOG_LEVEL.to_string());

        let worker_count = match lookup("WORKER_COUNT")
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
        {
            Some(raw) => {
                let parsed: usize = raw.parse().with_context(|| {
                    format!("WORKER_COUNT {:?} must be a positive integer", raw)
                })?;
                if parsed == 0 || parsed > 16 {
                    bail!("WORKER_COUNT must be between 1 and 16 (got {})", parsed);
                }
                parsed
            }
            None => DEFAULT_WORKER_COUNT,
        };

        let health_addr = lookup("HEALTH_ADDR")
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
            .unwrap_or_else(|| DEFAULT_HEALTH_ADDR.to_string());

        let meili_url = lookup("MEILI_URL")
            .or_else(|| lookup("MEILISEARCH_URL"))
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty());
        if let Some(ref url) = meili_url {
            validate_loopback_url("MEILI_URL", url, &["http", "https"])?;
        }

        let meili_api_key = lookup("MEILI_API_KEY")
            .or_else(|| lookup("MEILI_MASTER_KEY"))
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty());

        let meili_index = lookup("MEILI_INDEX")
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty());

        Ok(Self {
            redis_url,
            database_url,
            storage_root: PathBuf::from(storage_root),
            log_level,
            worker_count,
            health_addr,
            meili_url,
            meili_api_key,
            meili_index,
        })
    }
}

pub fn validate_loopback_url(name: &str, raw: &str, allowed_schemes: &[&str]) -> Result<()> {
    let parsed = Url::parse(raw).with_context(|| format!("{} is not a valid URL", name))?;
    if !allowed_schemes.contains(&parsed.scheme()) {
        bail!(
            "{} scheme {:?} must be one of {:?}",
            name,
            parsed.scheme(),
            allowed_schemes
        );
    }
    let host = parsed.host_str().unwrap_or_default();
    if host != REQUIRED_LOOPBACK_HOST {
        bail!(
            "refusing to start: {} host {:?} must be {} (local loopback guard)",
            name,
            host,
            REQUIRED_LOOPBACK_HOST
        );
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashMap;

    #[test]
    fn loads_defaults_and_enforces_loopback_guard() {
        let mut env_map = HashMap::new();
        env_map.insert(
            "DATABASE_URL",
            "postgres://postgres@127.0.0.1:5432/truecrime_dev?sslmode=disable",
        );

        let cfg = Config::load_from(|k| env_map.get(k).map(|v| (*v).to_string())).unwrap();
        assert_eq!(cfg.redis_url, DEFAULT_REDIS_URL);
        assert_eq!(cfg.storage_root, PathBuf::from(DEFAULT_STORAGE_ROOT));
        assert_eq!(cfg.worker_count, 2);
        assert_eq!(cfg.health_addr, DEFAULT_HEALTH_ADDR);

        // Non-loopback REDIS_URL must be refused.
        env_map.insert("REDIS_URL", "redis://redis.prod.internal:6379/0");
        let err = Config::load_from(|k| env_map.get(k).map(|v| (*v).to_string())).unwrap_err();
        assert!(err.to_string().contains("127.0.0.1"));
    }
}
