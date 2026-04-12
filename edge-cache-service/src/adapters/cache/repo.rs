use super::CacheSettings;
use super::paths::{data_path, lock_path, meta_path, tmp_data_path, tmp_meta_path};
use super::writer::CacheWriterImpl;
use super::{LogOnErr, compute_expires_at, write_meta_atomic};
use crate::ports::cache::{CacheError, CacheLock, CacheMeta, CacheRepo, CacheWriter, CachedFile};
use async_trait::async_trait;
use bytes::Bytes;
use fs4::tokio::AsyncFileExt;
use std::path::PathBuf;
use std::time::Duration;
use tokio::fs::OpenOptions;

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
        // read meta
        let meta_bytes = match tokio::fs::read(meta_path(&self.cache_dir, file_id)).await {
            Ok(b) => b,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(None),
            Err(e) => return Err(e).warn_on_err("cache meta read failed")?,
        };
        let meta: CacheMeta =
            serde_json::from_slice(&meta_bytes).warn_on_err("cache meta parse failed")?;

        // mmap data file — zero-copy: pages served directly from kernel page cache
        let file = match tokio::fs::File::open(data_path(&self.cache_dir, file_id)).await {
            Ok(f) => f,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(None),
            Err(e) => return Err(e).warn_on_err("cache data open failed")?,
        };
        // SAFETY: cache files are immutable after write — no concurrent modification.
        let mmap = unsafe { memmap2::Mmap::map(&file) }.warn_on_err("cache data mmap failed")?;
        let data = Bytes::from_owner(mmap);

        Ok(Some(CachedFile { meta, data }))
    }

    async fn begin_write(&self, file_id: &str) -> Result<Box<dyn CacheWriter>, CacheError> {
        let tmp = tmp_data_path(&self.cache_dir, file_id);
        let file = tokio::fs::File::create(&tmp)
            .await
            .warn_on_err("cache begin_write failed")?;

        Ok(Box::new(CacheWriterImpl::new(
            file,
            &self.cache_dir,
            file_id,
            self.default_ttl,
        )))
    }

    async fn refresh_ttl(&self, file_id: &str, max_age: Option<u64>) -> Result<(), CacheError> {
        let meta_file = meta_path(&self.cache_dir, file_id);
        let meta_bytes = tokio::fs::read(&meta_file)
            .await
            .warn_on_err("cache refresh_ttl read failed")?;
        let mut meta: CacheMeta =
            serde_json::from_slice(&meta_bytes).warn_on_err("cache refresh_ttl parse failed")?;

        meta.expires_at = compute_expires_at(max_age, self.default_ttl)?;

        let tmp = tmp_meta_path(&self.cache_dir, file_id);
        write_meta_atomic(&tmp, &meta_file, &meta).await
    }

    async fn acquire_lock(&self, file_id: &str) -> Result<CacheLock, CacheError> {
        let path = lock_path(&self.cache_dir, file_id);

        let file = OpenOptions::new()
            .create(true)
            .truncate(false)
            .write(true)
            .open(path)
            .await
            .warn_on_err("cache lock open failed")?;

        let file = tokio::task::spawn_blocking(move || -> Result<_, std::io::Error> {
            file.lock_exclusive()?;
            Ok(file)
        })
        .await
        .map_err(|e| CacheError::Io(e.into()))
        .warn_on_err("cache lock task failed")?
        .warn_on_err("cache lock acquire failed")?;

        Ok(CacheLock::new(file))
    }
}
