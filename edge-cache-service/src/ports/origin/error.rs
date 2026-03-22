use thiserror::Error;

#[derive(Error, Debug)]
pub enum OriginError {
    #[error("file not found on origin")]
    NotFound,

    #[error(transparent)]
    Unavailable(#[from] anyhow::Error),
}
