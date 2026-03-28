use std::sync::Arc;

use edge_cache_service::adapters::api::{ApiSettings, AppState};
use edge_cache_service::adapters::cache::{CacheRepoImpl, CacheSettings};
use edge_cache_service::adapters::logging::{self, LoggingSettings};
use edge_cache_service::adapters::origin::{OriginClientImpl, OriginSettings};
use edge_cache_service::application::download::DownloadUseCase;
use tracing::info;

#[tokio::main]
async fn main() {
    // --- Logging ---
    let logging_settings = LoggingSettings::load().expect("failed to load logging settings");
    logging::init(&logging_settings);

    // --- Settings ---
    let cache_settings = CacheSettings::load().expect("failed to load cache settings");
    let origin_settings = OriginSettings::load().expect("failed to load origin settings");
    let api_settings = ApiSettings::load().expect("failed to load api settings");

    // --- Adapters ---
    let cache = Arc::new(CacheRepoImpl::new(cache_settings));

    let http_client = reqwest::Client::builder()
        .timeout(origin_settings.timeout)
        .build()
        .expect("failed to build HTTP client");
    let origin = Arc::new(OriginClientImpl::new(origin_settings, http_client));

    // --- Application ---
    let state = Arc::new(AppState {
        download: DownloadUseCase::new(cache, origin),
    });

    // --- Server ---
    let listener = tokio::net::TcpListener::bind(&api_settings.listen_addr)
        .await
        .expect("failed to bind");
    info!(addr = %api_settings.listen_addr, "EdgeCache listening");
    axum::serve(listener, edge_cache_service::adapters::api::app(state))
        .await
        .unwrap();
}
