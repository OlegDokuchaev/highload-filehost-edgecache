use tracing::{error, warn};

pub trait LogOnErr<T, E> {
    fn warn_on_err(self, msg: &str) -> Result<T, E>;
    fn error_on_err(self, msg: &str) -> Result<T, E>;
}

impl<T, E: std::fmt::Display> LogOnErr<T, E> for Result<T, E> {
    fn warn_on_err(self, msg: &str) -> Result<T, E> {
        if let Err(ref e) = self {
            warn!(error = %e, "{msg}");
        }
        self
    }

    fn error_on_err(self, msg: &str) -> Result<T, E> {
        if let Err(ref e) = self {
            error!(error = %e, "{msg}");
        }
        self
    }
}
