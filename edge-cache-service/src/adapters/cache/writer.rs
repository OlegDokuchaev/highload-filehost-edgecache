use std::path::PathBuf;
use std::time::Duration;

use async_trait::async_trait;
use tokio::io::AsyncWriteExt;

use crate::ports::cache::CacheMeta;
use crate::ports::cache::{CacheError, CacheWriter};

use super::paths::{data_path, meta_path, tmp_data_path, tmp_meta_path};
use super::{LogOnErr, compute_expires_at, write_meta_atomic};

pub struct CacheWriterImpl {
    file: tokio::fs::File,
    tmp_data: PathBuf,
    final_data: PathBuf,
    tmp_meta: PathBuf,
    final_meta: PathBuf,
    committed: bool,
    default_ttl: Duration,
    written: u64,
}

impl CacheWriterImpl {
    pub(super) fn new(
        file: tokio::fs::File,
        cache_dir: &std::path::Path,
        file_id: &str,
        default_ttl: Duration,
    ) -> Self {
        Self {
            file,
            tmp_data: tmp_data_path(cache_dir, file_id),
            final_data: data_path(cache_dir, file_id),
            tmp_meta: tmp_meta_path(cache_dir, file_id),
            final_meta: meta_path(cache_dir, file_id),
            committed: false,
            default_ttl,
            written: 0,
        }
    }
}

#[async_trait]
impl CacheWriter for CacheWriterImpl {
    async fn write_chunk(&mut self, chunk: &[u8]) -> Result<(), CacheError> {
        self.file
            .write_all(chunk)
            .await
            .warn_on_err("cache write_chunk failed")?;
        self.written += chunk.len() as u64;
        Ok(())
    }

    async fn commit(
        mut self: Box<Self>,
        content_type: String,
        etag: Option<String>,
        max_age: Option<u64>,
    ) -> Result<(), CacheError> {
        tokio::fs::rename(&self.tmp_data, &self.final_data)
            .await
            .warn_on_err("cache commit rename data failed")?;

        let meta = CacheMeta {
            content_type,
            content_length: self.written,
            expires_at: compute_expires_at(max_age, self.default_ttl)?,
            etag,
        };
        write_meta_atomic(&self.tmp_meta, &self.final_meta, &meta).await?;

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
