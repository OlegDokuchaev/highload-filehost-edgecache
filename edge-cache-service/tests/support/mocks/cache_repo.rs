use mockall::mock;

use edge_cache_service::ports::cache::{CacheError, CacheLock, CacheRepo, CacheWriter, CachedFile};

mock! {
    pub CacheRepo {}

    #[async_trait::async_trait]
    impl CacheRepo for CacheRepo {
        async fn lookup(&self, file_id: &str) -> Result<Option<CachedFile>, CacheError>;
        async fn begin_write(&self, file_id: &str) -> Result<Box<dyn CacheWriter>, CacheError>;
        async fn refresh_ttl(&self, file_id: &str) -> Result<(), CacheError>;
        async fn acquire_lock(&self, file_id: &str) -> Result<CacheLock, CacheError>;
    }
}
