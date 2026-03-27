use super::CacheSettings;
use super::paths::{data_path, lock_path, meta_path, tmp_data_path, tmp_meta_path};
use super::writer::CacheWriterImpl;
use crate::ports::cache::{CacheError, CacheLock, CacheMeta, CacheRepo, CacheWriter, CachedFile};
use async_trait::async_trait;
use fs4::tokio::AsyncFileExt;
use std::path::PathBuf;
use std::time::{Duration, SystemTime, UNIX_EPOCH};
use tokio::fs::OpenOptions;
use tokio_util::io::ReaderStream;

#[derive(Clone)]
pub struct CacheRepoImpl {
    cache_dir: PathBuf,
    default_ttl: Duration,
}

impl CacheRepoImpl {
    pub fn new(settings: CacheSettings) -> Self {
        Self {
            cache_dir: PathBuf::from(settings.dir),
            default_ttl: settings.default_ttl,
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
            self.default_ttl,
        )))
    }

    async fn refresh_ttl(&self, file_id: &str, max_age: Option<u64>) -> Result<(), CacheError> {
        let meta_file = meta_path(&self.cache_dir, file_id);
        let meta_bytes = tokio::fs::read(&meta_file).await?;
        let mut meta: CacheMeta = serde_json::from_slice(&meta_bytes)?;

        let now = SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs() as i64;
        let ttl_secs = max_age.unwrap_or(self.default_ttl.as_secs()) as i64;
        meta.expires_at = now + ttl_secs;

        let tmp = tmp_meta_path(&self.cache_dir, file_id);
        let data = serde_json::to_vec(&meta)?;
        tokio::fs::write(&tmp, data).await?;
        tokio::fs::rename(&tmp, &meta_file).await?;

        Ok(())
    }

    async fn acquire_lock(&self, file_id: &str) -> Result<CacheLock, CacheError> {
        let path = lock_path(&self.cache_dir, file_id);

        let file = OpenOptions::new()
            .create(true)
            .truncate(false)
            .write(true)
            .open(path)
            .await
            .map_err(CacheError::Io)?;

        let file = tokio::task::spawn_blocking(move || -> Result<_, std::io::Error> {
            file.lock_exclusive()?;
            Ok(file)
        })
        .await
        .map_err(|e| CacheError::Io(e.into()))??;

        Ok(CacheLock::new(file))
    }
}
