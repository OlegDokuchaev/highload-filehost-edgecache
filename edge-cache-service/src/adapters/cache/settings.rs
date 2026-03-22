use config::{Config, Environment};
use serde::Deserialize;

#[derive(Debug, Deserialize, Clone)]
pub struct CacheSettings {
    pub dir: String,
}

impl CacheSettings {
    pub fn load() -> Result<Self, config::ConfigError> {
        Config::builder()
            .add_source(
                Environment::with_prefix("CACHE")
                    .separator("__")
                    .prefix_separator("__"),
            )
            .build()?
            .try_deserialize()
    }
}
