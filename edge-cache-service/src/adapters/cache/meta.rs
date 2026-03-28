use std::path::Path;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use super::LogOnErr;
use crate::ports::cache::{CacheError, CacheMeta};

pub(super) fn compute_expires_at(
    max_age: Option<u64>,
    default_ttl: Duration,
) -> Result<i64, CacheError> {
    let now = SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs() as i64;
    let ttl = max_age.unwrap_or(default_ttl.as_secs()) as i64;
    Ok(now + ttl)
}

pub(super) async fn write_meta_atomic(
    tmp: &Path,
    final_path: &Path,
    meta: &CacheMeta,
) -> Result<(), CacheError> {
    let data = serde_json::to_vec(meta)?;
    tokio::fs::write(tmp, data)
        .await
        .warn_on_err("cache write meta failed")?;
    tokio::fs::rename(tmp, final_path)
        .await
        .warn_on_err("cache rename meta failed")?;
    Ok(())
}
