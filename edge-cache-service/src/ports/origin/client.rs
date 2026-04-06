use async_trait::async_trait;

use super::{ConditionalGetResult, OriginError, OriginResponse};

#[async_trait]
pub trait OriginClient: Send + Sync {
    async fn fetch(&self, file_id: &str) -> Result<OriginResponse, OriginError>;

    async fn fetch_if_changed(
        &self,
        file_id: &str,
        etag: &str,
    ) -> Result<ConditionalGetResult, OriginError>;
}
