use bytes::Bytes;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CacheMeta {
    pub content_type: String,
    pub content_length: u64,
    pub expires_at: i64,
    pub etag: Option<String>,
}

pub struct CachedFile {
    pub meta: CacheMeta,
    pub data: Bytes,
}

/// Advisory file lock guard. Releasing happens automatically when dropped (fd close).
pub struct CacheLock {
    _file: tokio::fs::File,
}

impl CacheLock {
    pub fn new(file: tokio::fs::File) -> Self {
        Self { _file: file }
    }
}
