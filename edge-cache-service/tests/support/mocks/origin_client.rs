use mockall::mock;

use edge_cache_service::ports::origin::{
    ConditionalGetResult, OriginClient, OriginError, OriginResponse,
};

mock! {
    pub OriginClient {}

    #[async_trait::async_trait]
    impl OriginClient for OriginClient {
        async fn fetch(&self, file_id: &str) -> Result<OriginResponse, OriginError>;
        async fn fetch_if_changed(&self, file_id: &str, etag: &str) -> Result<ConditionalGetResult, OriginError>;
    }
}
