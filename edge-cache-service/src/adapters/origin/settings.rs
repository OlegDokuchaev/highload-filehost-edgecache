use std::time::Duration;

use config::{Config, Environment};
use serde::Deserialize;

#[derive(Debug, Deserialize, Clone)]
pub struct OriginSettings {
    pub base_url: String,
    #[serde(with = "humantime_serde")]
    pub timeout: Duration,
}

impl OriginSettings {
    pub fn load() -> Result<Self, config::ConfigError> {
        Config::builder()
            .add_source(
                Environment::with_prefix("ORIGIN")
                    .separator("__")
                    .prefix_separator("__"),
            )
            .build()?
            .try_deserialize()
    }
}
