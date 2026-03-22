use mockall::mock;

use edge_cache_service::ports::cache::{CacheError, CacheMeta, CacheWriter};

mock! {
    pub CacheWriter {}

    #[async_trait::async_trait]
    impl CacheWriter for CacheWriter {
        async fn write_chunk(&mut self, chunk: &[u8]) -> Result<(), CacheError>;
        async fn commit(self: Box<Self>, meta: CacheMeta) -> Result<(), CacheError>;
    }
}
