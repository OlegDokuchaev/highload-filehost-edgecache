use axum::{
    body::Body,
    http::{Response, StatusCode},
    response::IntoResponse,
};

use crate::application::download::DownloadError;
use crate::ports::cache::CacheError;
use crate::ports::origin::OriginError;

pub struct ApiError {
    status: StatusCode,
    message: String,
}

impl ApiError {
    pub fn new(status: StatusCode, message: impl Into<String>) -> Self {
        Self {
            status,
            message: message.into(),
        }
    }

    pub fn internal() -> Self {
        Self::new(StatusCode::INTERNAL_SERVER_ERROR, "internal server error")
    }
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response<Body> {
        (self.status, self.message).into_response()
    }
}

impl From<DownloadError> for ApiError {
    fn from(err: DownloadError) -> Self {
        match err {
            DownloadError::InvalidFileId => ApiError::new(StatusCode::BAD_REQUEST, err.to_string()),
            DownloadError::Origin(origin_err) => ApiError::from(origin_err),
            DownloadError::Cache(cache_err) => ApiError::from(cache_err),
        }
    }
}

impl From<OriginError> for ApiError {
    fn from(err: OriginError) -> Self {
        match err {
            OriginError::NotFound => ApiError::new(StatusCode::NOT_FOUND, "file not found"),
            OriginError::Unavailable(_) => {
                ApiError::new(StatusCode::BAD_GATEWAY, "origin unavailable")
            }
        }
    }
}

impl From<CacheError> for ApiError {
    fn from(_err: CacheError) -> Self {
        ApiError::internal()
    }
}
