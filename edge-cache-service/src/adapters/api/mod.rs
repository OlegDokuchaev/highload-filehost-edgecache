mod error;
mod handlers;
mod settings;

pub use settings::*;

use std::sync::Arc;

use axum::Router;
use tower_http::trace::TraceLayer;
use utoipa::OpenApi;
use utoipa_axum::router::OpenApiRouter;
use utoipa_swagger_ui::SwaggerUi;

use crate::application::download::DownloadUseCase;

pub struct AppState {
    pub download: DownloadUseCase,
}

#[derive(OpenApi)]
#[openapi(
    info(
        title = "EdgeCache Service",
        version = "0.1.0",
        description = "Кэширующий прокси-сервис для раздачи файлов."
    ),
    tags(
        (name = "files", description = "Скачивание файлов через кэширующий прокси")
    )
)]
struct ApiDoc;

pub fn app(state: Arc<AppState>) -> Router {
    let (router, api) = OpenApiRouter::with_openapi(ApiDoc::openapi())
        .routes(utoipa_axum::routes!(handlers::download))
        .split_for_parts();

    router
        .merge(SwaggerUi::new("/docs").url("/openapi.json", api))
        .layer(TraceLayer::new_for_http())
        .with_state(state)
}
