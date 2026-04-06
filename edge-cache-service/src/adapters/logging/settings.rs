use config::{Config, Environment};
use serde::Deserialize;

#[derive(Debug, Deserialize, Clone)]
pub struct LoggingSettings {
    pub level: String,
}

impl LoggingSettings {
    pub fn load() -> Result<Self, config::ConfigError> {
        Config::builder()
            .add_source(
                Environment::with_prefix("LOG")
                    .separator("__")
                    .prefix_separator("__"),
            )
            .build()?
            .try_deserialize()
    }
}
