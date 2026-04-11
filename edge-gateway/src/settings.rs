use std::net::SocketAddr;
use std::time::Duration;

use config::{Config, Environment};
use serde::Deserialize;

#[derive(Debug, Deserialize, Clone)]
pub struct GatewaySettings {
    pub listen_addr: SocketAddr,
    pub backends: String,
    #[serde(with = "humantime_serde")]
    pub request_timeout: Duration,
}

impl GatewaySettings {
    pub fn load() -> Result<Self, config::ConfigError> {
        Config::builder()
            .add_source(
                Environment::with_prefix("GATEWAY")
                    .separator("__")
                    .prefix_separator("__"),
            )
            .build()?
            .try_deserialize()
    }

    pub fn backend_urls(&self) -> Vec<String> {
        self.backends
            .split(',')
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
            .collect()
    }
}
