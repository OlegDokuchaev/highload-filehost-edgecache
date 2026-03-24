use std::time::SystemTimeError;

use thiserror::Error;

#[derive(Error, Debug)]
pub enum CacheError {
    #[error(transparent)]
    Io(#[from] std::io::Error),

    #[error("corrupted cache metadata: {0}")]
    CorruptedMeta(#[from] serde_json::Error),

    #[error("system clock error: {0}")]
    Clock(#[from] SystemTimeError),
}

/// Allows `?` inside `try_stream!` where the stream error type is `io::Error`.
impl From<CacheError> for std::io::Error {
    fn from(e: CacheError) -> Self {
        match e {
            CacheError::Io(io_err) => io_err,
            other => std::io::Error::other(other),
        }
    }
}
