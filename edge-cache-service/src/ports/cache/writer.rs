use super::CacheError;
use async_trait::async_trait;

#[async_trait]
pub trait CacheWriter: Send {
    async fn write_chunk(&mut self, chunk: &[u8]) -> Result<(), CacheError>;
    async fn commit(
        self: Box<Self>,
        content_type: String,
        etag: Option<String>,
        max_age: Option<u64>,
    ) -> Result<(), CacheError>;
}
