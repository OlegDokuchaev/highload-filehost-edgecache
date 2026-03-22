use async_trait::async_trait;

use super::{OriginError, OriginResponse};

#[async_trait]
pub trait OriginClient: Send + Sync {
    async fn fetch(&self, file_id: &str) -> Result<OriginResponse, OriginError>;
}
