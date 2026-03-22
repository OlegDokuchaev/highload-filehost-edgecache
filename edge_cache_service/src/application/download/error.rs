use thiserror::Error;

use crate::ports::cache::CacheError;
use crate::ports::origin::OriginError;

#[derive(Error, Debug)]
pub enum DownloadError {
    #[error("invalid file id")]
    InvalidFileId,

    #[error(transparent)]
    Origin(#[from] OriginError),

    #[error(transparent)]
    Cache(#[from] CacheError),
}
