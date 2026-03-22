use mockall::mock;

use edge_cache_service::ports::origin::{OriginClient, OriginError, OriginResponse};

mock! {
    pub OriginClient {}

    #[async_trait::async_trait]
    impl OriginClient for OriginClient {
        async fn fetch(&self, file_id: &str) -> Result<OriginResponse, OriginError>;
    }
}
