use std::path::PathBuf;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use async_trait::async_trait;
use tokio_util::io::ReaderStream;

use crate::ports::cache::{CacheError, CacheMeta, CacheRepo, CacheWriter, CachedFile};

use super::CacheSettings;
use super::paths::{data_path, meta_path, tmp_data_path, tmp_meta_path};
use super::writer::CacheWriterImpl;

#[derive(Clone)]
pub struct CacheRepoImpl {
    cache_dir: PathBuf,
    ttl: Duration,
}

impl CacheRepoImpl {
    pub fn new(settings: CacheSettings) -> Self {
        Self {
            cache_dir: PathBuf::from(settings.dir),
            ttl: settings.ttl,
        }
    }
}

#[async_trait]
impl CacheRepo for CacheRepoImpl {
    async fn lookup(&self, file_id: &str) -> Result<Option<CachedFile>, CacheError> {
        let meta_bytes = match tokio::fs::read(meta_path(&self.cache_dir, file_id)).await {
            Ok(b) => b,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(None),
            Err(e) => return Err(e.into()),
        };
        let meta: CacheMeta = serde_json::from_slice(&meta_bytes)?;

        let file = match tokio::fs::File::open(data_path(&self.cache_dir, file_id)).await {
            Ok(f) => f,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(None),
            Err(e) => return Err(e.into()),
        };
        let stream = Box::pin(ReaderStream::new(file));

        Ok(Some(CachedFile { meta, stream }))
    }

    async fn begin_write(&self, file_id: &str) -> Result<Box<dyn CacheWriter>, CacheError> {
        let tmp = tmp_data_path(&self.cache_dir, file_id);
        let file = tokio::fs::File::create(&tmp).await?;

        Ok(Box::new(CacheWriterImpl::new(
            file,
            &self.cache_dir,
            file_id,
            self.ttl,
        )))
    }

    async fn refresh_ttl(&self, file_id: &str) -> Result<(), CacheError> {
        let meta_file = meta_path(&self.cache_dir, file_id);
        let meta_bytes = tokio::fs::read(&meta_file).await?;
        let mut meta: CacheMeta = serde_json::from_slice(&meta_bytes)?;

        let now = SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs() as i64;
        meta.expires_at = now + self.ttl.as_secs() as i64;

        let tmp = tmp_meta_path(&self.cache_dir, file_id);
        let data = serde_json::to_vec(&meta)?;
        tokio::fs::write(&tmp, data).await?;
        tokio::fs::rename(&tmp, &meta_file).await?;

        Ok(())
    }
}
