use std::sync::Arc;

use crate::ports::cache::CacheRepo;
use crate::ports::origin::OriginClient;

use super::{DownloadAction, DownloadError};

pub struct DownloadUseCase {
    cache: Arc<dyn CacheRepo>,
    origin: Arc<dyn OriginClient>,
}

impl DownloadUseCase {
    pub fn new(cache: Arc<dyn CacheRepo>, origin: Arc<dyn OriginClient>) -> Self {
        Self { cache, origin }
    }

    pub async fn execute(&self, file_id: &str) -> Result<DownloadAction, DownloadError> {
        if !is_valid_file_id(file_id) {
            return Err(DownloadError::InvalidFileId);
        }

        // HIT — cache errors are treated as a miss (graceful degradation).
        if let Ok(Some(cached)) = self.cache.lookup(file_id).await {
            return Ok(DownloadAction::Hit(cached));
        }

        // MISS — fetch from origin, `?` auto-converts via #[from].
        let origin_resp = self.origin.fetch(file_id).await?;
        let writer = self.cache.begin_write(file_id).await?;

        Ok(DownloadAction::Miss {
            origin: origin_resp,
            writer,
        })
    }
}

fn is_valid_file_id(id: &str) -> bool {
    !id.is_empty()
        && id.len() <= 200
        && !id.starts_with('.')
        && id
            .bytes()
            .all(|b| b.is_ascii_alphanumeric() || b == b'-' || b == b'_' || b == b'.')
}
