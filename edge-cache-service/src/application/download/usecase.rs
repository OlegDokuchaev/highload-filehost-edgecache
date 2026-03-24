use std::sync::Arc;
use std::time::{SystemTime, UNIX_EPOCH};

use crate::ports::cache::{CacheMeta, CacheRepo, CachedFile};
use crate::ports::origin::{ConditionalGetResult, OriginClient};

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

        // Cache lookup — errors treated as miss (graceful degradation).
        if let Ok(Some(cached)) = self.cache.lookup(file_id).await {
            if !is_expired(&cached.meta) {
                return Ok(DownloadAction::Hit(cached));
            }

            // Expired — revalidate (conditional GET or full re-fetch).
            return self.revalidate(file_id, cached).await;
        }

        // MISS — not in cache at all.
        self.fetch_and_cache(file_id).await
    }

    async fn revalidate(
        &self,
        file_id: &str,
        cached: CachedFile,
    ) -> Result<DownloadAction, DownloadError> {
        // Has etag — conditional GET via If-None-Match.
        if let Some(etag) = cached.meta.etag.as_deref() {
            let result = self.origin.fetch_if_changed(file_id, etag).await?;

            return match result {
                ConditionalGetResult::NotModified => {
                    self.cache.refresh_ttl(file_id).await?;
                    Ok(DownloadAction::Revalidated(cached))
                }
                ConditionalGetResult::Modified(origin_resp) => {
                    let writer = self.cache.begin_write(file_id).await?;
                    Ok(DownloadAction::RevalidatedWithNewContent {
                        origin: origin_resp,
                        writer,
                    })
                }
            };
        }

        // No etag — full re-fetch.
        let origin_resp = self.origin.fetch(file_id).await?;
        let writer = self.cache.begin_write(file_id).await?;

        Ok(DownloadAction::RevalidatedWithNewContent {
            origin: origin_resp,
            writer,
        })
    }

    async fn fetch_and_cache(&self, file_id: &str) -> Result<DownloadAction, DownloadError> {
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

fn is_expired(meta: &CacheMeta) -> bool {
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs() as i64;
    now >= meta.expires_at
}
