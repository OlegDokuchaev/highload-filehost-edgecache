use crate::ports::cache::{CacheLock, CacheWriter, CachedFile};
use crate::ports::origin::OriginResponse;

pub enum DownloadAction {
    Hit(CachedFile),
    Miss {
        origin: OriginResponse,
        writer: Box<dyn CacheWriter>,
        lock: CacheLock,
    },
    Revalidated(CachedFile),
    RevalidatedWithNewContent {
        origin: OriginResponse,
        writer: Box<dyn CacheWriter>,
        lock: CacheLock,
    },
}
