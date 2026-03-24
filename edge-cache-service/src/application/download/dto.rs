use crate::ports::cache::{CacheWriter, CachedFile};
use crate::ports::origin::OriginResponse;

pub enum DownloadAction {
    Hit(CachedFile),
    Miss {
        origin: OriginResponse,
        writer: Box<dyn CacheWriter>,
    },
    Revalidated(CachedFile),
    RevalidatedWithNewContent {
        origin: OriginResponse,
        writer: Box<dyn CacheWriter>,
    },
}
