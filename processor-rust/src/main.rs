use std::sync::Arc;

use anyhow::{Context, Result};
use tokio::sync::watch;
use tracing::{error, info};
use tracing_subscriber::EnvFilter;

use processor_rust::config::Config;
use processor_rust::consumer;
use processor_rust::db::Db;
use processor_rust::health;
use processor_rust::ocr;
use processor_rust::processor::DocumentProcessor;

#[tokio::main]
async fn main() -> Result<()> {
    let cfg = Config::load().context("load processor-rust configuration")?;

    let env_filter = EnvFilter::try_new(&cfg.log_level).unwrap_or_else(|_| EnvFilter::new("info"));
    tracing_subscriber::fmt()
        .json()
        .with_env_filter(env_filter)
        .init();

    let ocr_available = ocr::probe_tesseract_at_startup();

    let db = Db::connect(&cfg.database_url).await?;
    db.ping().await.context("initial PostgreSQL ping failed")?;

    let redis_client =
        redis::Client::open(cfg.redis_url.as_str()).context("parse REDIS_URL for redis client")?;
    let mut boot_conn = redis_client
        .get_multiplexed_async_connection()
        .await
        .context("connect to local Redis")?;
    consumer::ensure_consumer_group(&mut boot_conn).await?;

    let processor = Arc::new(DocumentProcessor::new(
        cfg.storage_root.clone(),
        ocr_available,
    ));

    let meili_updater = match cfg.meili_url {
        Some(ref url) => Some(
            processor_rust::meili::MeiliUpdater::new(
                url,
                cfg.meili_api_key.clone(),
                cfg.meili_index.clone(),
            )
            .context("initialize Meilisearch updater")?,
        ),
        None => None,
    };

    let (shutdown_tx, shutdown_rx) = watch::channel(false);

    info!(
        worker_count = cfg.worker_count,
        storage_root = %cfg.storage_root.display(),
        health_addr = %cfg.health_addr,
        ocr_available = ocr_available,
        meili_enabled = meili_updater.is_some(),
        "starting processor-rust workers"
    );

    // Spawn lightweight HTTP /health server on :9091
    let health_handle = tokio::spawn(health::serve_health(
        cfg.health_addr.clone(),
        redis_client.clone(),
        db.clone(),
        shutdown_rx.clone(),
    ));

    // Spawn WORKER_COUNT concurrent Redis Streams consumer tasks
    let mut worker_handles = Vec::with_capacity(cfg.worker_count);
    for worker_idx in 1..=cfg.worker_count {
        let rc = redis_client.clone();
        let db_clone = db.clone();
        let proc_clone = Arc::clone(&processor);
        let meili_clone = meili_updater.clone();
        let rx = shutdown_rx.clone();
        worker_handles.push(tokio::spawn(async move {
            if let Err(err) = consumer::run_worker_loop_with_meili(
                worker_idx,
                rc,
                db_clone,
                proc_clone,
                meili_clone,
                rx,
            )
            .await
            {
                error!(worker = worker_idx, error = %err, "worker exited with error");
            }
        }));
    }

    // Wait for SIGINT or SIGTERM
    wait_for_shutdown_signal().await;
    info!("shutdown signal received; stopping processor-rust");
    let _ = shutdown_tx.send(true);

    for handle in worker_handles {
        let _ = handle.await;
    }
    let _ = health_handle.await;

    info!("processor-rust stopped cleanly");
    Ok(())
}

async fn wait_for_shutdown_signal() {
    let ctrl_c = async {
        let _ = tokio::signal::ctrl_c().await;
    };

    #[cfg(unix)]
    let terminate = async {
        if let Ok(mut sig) =
            tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
        {
            sig.recv().await;
        }
    };

    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        _ = ctrl_c => {},
        _ = terminate => {},
    }
}
