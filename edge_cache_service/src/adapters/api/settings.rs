use config::{Config, Environment};
use serde::Deserialize;

#[derive(Debug, Deserialize, Clone)]
pub struct ApiSettings {
    pub listen_addr: String,
}

impl ApiSettings {
    pub fn load() -> Result<Self, config::ConfigError> {
        Config::builder()
            .add_source(
                Environment::with_prefix("API")
                    .separator("__")
                    .prefix_separator("__"),
            )
            .build()?
            .try_deserialize()
    }
}
