use std::sync::Arc;

use async_stream::try_stream;
use axum::response::IntoResponse;
use axum::{
    body::Body,
    extract::{Path, State},
    http::{HeaderValue, Response, header},
};
use bytes::Bytes;
use futures_util::StreamExt;

use crate::application::download::DownloadAction;
use crate::ports::cache::{CacheWriter, CachedFile};
use crate::ports::origin::OriginResponse;

use super::AppState;
use super::error::ApiError;

pub async fn download(
    State(state): State<Arc<AppState>>,
    Path(file_id): Path<String>,
) -> Result<Response<Body>, ApiError> {
    let action = state.download.execute(&file_id).await?;
    Ok(match action {
        DownloadAction::Hit(cached) => serve_cached(cached),
        DownloadAction::Miss { origin, writer } => serve_miss(origin, writer),
    })
}

fn download_response(
    body: Body,
    x_cache: &'static str,
    content_type: HeaderValue,
) -> Response<Body> {
    let mut resp = Response::new(body);

    let h = resp.headers_mut();
    h.insert("x-cache", HeaderValue::from_static(x_cache));
    h.insert(header::CONTENT_TYPE, content_type);

    resp
}

fn serve_cached(cached: CachedFile) -> Response<Body> {
    let Ok(content_type) = HeaderValue::from_str(&cached.meta.content_type) else {
        return ApiError::internal().into_response();
    };

    let mut resp = download_response(Body::from_stream(cached.stream), "HIT", content_type);
    resp.headers_mut().insert(
        header::CONTENT_LENGTH,
        HeaderValue::from(cached.meta.content_length),
    );

    resp
}

fn serve_miss(origin: OriginResponse, writer: Box<dyn CacheWriter>) -> Response<Body> {
    let Ok(content_type) = HeaderValue::from_str(&origin.content_type) else {
        return ApiError::internal().into_response();
    };

    let body_stream = stream_and_cache(origin, writer);
    download_response(Body::from_stream(body_stream), "MISS", content_type)
}

/// Tees the origin byte stream: yields chunks to the client while writing them
/// to the cache via the writer. On completion, commits the cache entry.
///
/// `CacheError` converts to `io::Error` via `From`, so `?` works seamlessly.
fn stream_and_cache(
    origin: OriginResponse,
    mut writer: Box<dyn CacheWriter>,
) -> impl futures_util::Stream<Item = Result<Bytes, std::io::Error>> + Send {
    let mut body = origin.body;
    let content_type = origin.content_type;

    try_stream! {
        let mut written = 0;

        while let Some(chunk) = body.next().await {
            let chunk = chunk?;
            written += chunk.len() as u64;
            writer.write_chunk(&chunk).await?;
            yield chunk;
        }

        writer.commit(content_type, written).await?;
    }
}
