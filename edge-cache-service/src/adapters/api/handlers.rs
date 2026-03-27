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
use crate::ports::cache::{CacheLock, CacheWriter, CachedFile};
use crate::ports::origin::OriginResponse;

use super::AppState;
use super::error::ApiError;

pub async fn download(
    State(state): State<Arc<AppState>>,
    Path(file_id): Path<String>,
) -> Result<Response<Body>, ApiError> {
    let action = state.download.execute(&file_id).await?;
    Ok(match action {
        DownloadAction::Hit(cached) => serve_from_cache(cached, "HIT"),
        DownloadAction::Revalidated(cached) => serve_from_cache(cached, "REVALIDATED"),
        DownloadAction::Miss {
            origin,
            writer,
            lock,
        } => serve_from_origin(origin, writer, lock, "MISS"),
        DownloadAction::RevalidatedWithNewContent {
            origin,
            writer,
            lock,
        } => serve_from_origin(origin, writer, lock, "REVALIDATED"),
    })
}

fn download_response(
    body: Body,
    x_cache: &'static str,
    content_type: HeaderValue,
    etag: Option<HeaderValue>,
) -> Response<Body> {
    let mut resp = Response::new(body);

    let h = resp.headers_mut();
    h.insert("x-cache", HeaderValue::from_static(x_cache));
    h.insert(header::CONTENT_TYPE, content_type);
    if let Some(val) = etag {
        h.insert(header::ETAG, val);
    }

    resp
}

fn serve_from_cache(cached: CachedFile, x_cache: &'static str) -> Response<Body> {
    let Ok(content_type) = HeaderValue::from_str(&cached.meta.content_type) else {
        return ApiError::internal().into_response();
    };
    let Ok(etag) = try_parse_etag(cached.meta.etag.as_deref()) else {
        return ApiError::internal().into_response();
    };

    let mut resp = download_response(
        Body::from_stream(cached.stream),
        x_cache,
        content_type,
        etag,
    );
    resp.headers_mut().insert(
        header::CONTENT_LENGTH,
        HeaderValue::from(cached.meta.content_length),
    );

    resp
}

fn serve_from_origin(
    origin: OriginResponse,
    writer: Box<dyn CacheWriter>,
    lock: CacheLock,
    x_cache: &'static str,
) -> Response<Body> {
    let Ok(content_type) = HeaderValue::from_str(&origin.content_type) else {
        return ApiError::internal().into_response();
    };
    let Ok(etag) = try_parse_etag(origin.etag.as_deref()) else {
        return ApiError::internal().into_response();
    };

    let body_stream = stream_and_cache(origin, writer, lock);
    download_response(Body::from_stream(body_stream), x_cache, content_type, etag)
}

fn try_parse_etag(etag: Option<&str>) -> Result<Option<HeaderValue>, header::InvalidHeaderValue> {
    etag.map(HeaderValue::from_str).transpose()
}

/// Tees the origin byte stream: yields chunks to the client while writing them
/// to the cache via the writer. On completion, commits the cache entry.
///
/// `CacheError` converts to `io::Error` via `From`, so `?` works seamlessly.
fn stream_and_cache(
    origin: OriginResponse,
    mut writer: Box<dyn CacheWriter>,
    lock: CacheLock,
) -> impl futures_util::Stream<Item = Result<Bytes, std::io::Error>> + Send {
    let mut body = origin.body;
    let content_type = origin.content_type;
    let etag = origin.etag;
    let max_age = origin.max_age;

    try_stream! {
        while let Some(chunk) = body.next().await {
            let chunk = chunk?;
            writer.write_chunk(&chunk).await?;
            yield chunk;
        }

        writer.commit(content_type, etag, max_age).await?;
        drop(lock);
    }
}
