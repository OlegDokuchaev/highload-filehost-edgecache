use std::path::PathBuf;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use async_trait::async_trait;
use tokio::io::AsyncWriteExt;

use crate::ports::cache::CacheMeta;
use crate::ports::cache::{CacheError, CacheWriter};

use super::paths::{data_path, meta_path, tmp_data_path, tmp_meta_path};

pub struct CacheWriterImpl {
    file: tokio::fs::File,
    tmp_data: PathBuf,
    final_data: PathBuf,
    tmp_meta: PathBuf,
    final_meta: PathBuf,
    committed: bool,
    ttl: Duration,
}

impl CacheWriterImpl {
    pub(super) fn new(
        file: tokio::fs::File,
        cache_dir: &std::path::Path,
        file_id: &str,
        ttl: Duration,
    ) -> Self {
        Self {
            file,
            tmp_data: tmp_data_path(cache_dir, file_id),
            final_data: data_path(cache_dir, file_id),
            tmp_meta: tmp_meta_path(cache_dir, file_id),
            final_meta: meta_path(cache_dir, file_id),
            committed: false,
            ttl,
        }
    }
}

#[async_trait]
impl CacheWriter for CacheWriterImpl {
    async fn write_chunk(&mut self, chunk: &[u8]) -> Result<(), CacheError> {
        self.file.write_all(chunk).await?;
        Ok(())
    }

    async fn commit(
        mut self: Box<Self>,
        content_type: String,
        content_length: u64,
    ) -> Result<(), CacheError> {
        tokio::fs::rename(&self.tmp_data, &self.final_data).await?;

        let now = SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs() as i64;

        let meta = CacheMeta {
            content_type,
            content_length,
            expires_at: now + self.ttl.as_secs() as i64,
        };
        let data = serde_json::to_vec(&meta)?;
        tokio::fs::write(&self.tmp_meta, data).await?;
        tokio::fs::rename(&self.tmp_meta, &self.final_meta).await?;

        self.committed = true;
        Ok(())
    }
}

impl Drop for CacheWriterImpl {
    fn drop(&mut self) {
        if !self.committed {
            let _ = std::fs::remove_file(&self.tmp_data);
            let _ = std::fs::remove_file(&self.tmp_meta);
        }
    }
}
