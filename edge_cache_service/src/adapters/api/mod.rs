mod error;
mod handlers;
mod settings;

pub use settings::*;

use std::sync::Arc;

use axum::{Router, routing::get};

use crate::application::download::DownloadUseCase;

pub struct AppState {
    pub download: DownloadUseCase,
}

pub fn app(state: Arc<AppState>) -> Router {
    Router::new()
        .route("/download/{file_id}", get(handlers::download))
        .with_state(state)
}
