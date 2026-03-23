use super::CacheError;
use async_trait::async_trait;

#[async_trait]
pub trait CacheWriter: Send {
    async fn write_chunk(&mut self, chunk: &[u8]) -> Result<(), CacheError>;
    async fn commit(
        self: Box<Self>,
        content_type: String,
        content_length: u64,
    ) -> Result<(), CacheError>;
}
