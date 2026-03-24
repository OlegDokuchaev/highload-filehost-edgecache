use mockall::mock;

use edge_cache_service::ports::cache::{CacheError, CacheWriter};

mock! {
    pub CacheWriter {}

    #[async_trait::async_trait]
    impl CacheWriter for CacheWriter {
        async fn write_chunk(&mut self, chunk: &[u8]) -> Result<(), CacheError>;
        async fn commit(self: Box<Self>, content_type: String, etag: Option<String>) -> Result<(), CacheError>;
    }
}
