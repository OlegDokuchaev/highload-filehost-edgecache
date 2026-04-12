mod settings;

pub use settings::*;

use tracing_subscriber::EnvFilter;

pub fn init(settings: &LoggingSettings) {
    tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::new(&settings.level))
        .init();
}
