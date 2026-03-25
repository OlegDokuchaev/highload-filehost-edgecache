use super::{CacheError, CacheLock, CacheWriter, CachedFile};
use async_trait::async_trait;

#[async_trait]
pub trait CacheRepo: Send + Sync {
    async fn lookup(&self, file_id: &str) -> Result<Option<CachedFile>, CacheError>;
    async fn begin_write(&self, file_id: &str) -> Result<Box<dyn CacheWriter>, CacheError>;
    async fn refresh_ttl(&self, file_id: &str) -> Result<(), CacheError>;
    async fn acquire_lock(&self, file_id: &str) -> Result<CacheLock, CacheError>;
}
