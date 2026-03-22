use super::CacheError;
use super::CacheMeta;
use async_trait::async_trait;

#[async_trait]
pub trait CacheWriter: Send {
    async fn write_chunk(&mut self, chunk: &[u8]) -> Result<(), CacheError>;
    async fn commit(self: Box<Self>, meta: CacheMeta) -> Result<(), CacheError>;
}
