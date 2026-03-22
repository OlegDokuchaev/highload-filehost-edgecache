use serde::{Deserialize, Serialize};

use crate::ports::common::ByteStream;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CacheMeta {
    pub content_type: String,
    pub content_length: u64,
}

pub struct CachedFile {
    pub meta: CacheMeta,
    pub stream: ByteStream,
}
