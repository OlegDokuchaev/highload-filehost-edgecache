use async_trait::async_trait;
use futures_util::StreamExt;

use crate::ports::origin::{OriginClient, OriginError, OriginResponse};

use super::OriginSettings;

#[derive(Clone)]
pub struct OriginClientImpl {
    base_url: String,
    client: reqwest::Client,
}

impl OriginClientImpl {
    pub fn new(settings: OriginSettings, client: reqwest::Client) -> Self {
        Self {
            base_url: settings.base_url,
            client,
        }
    }
}

#[async_trait]
impl OriginClient for OriginClientImpl {
    async fn fetch(&self, file_id: &str) -> Result<OriginResponse, OriginError> {
        let url = format!("{}/files/{}", self.base_url, file_id);
        let resp = self
            .client
            .get(&url)
            .send()
            .await
            .map_err(|e| OriginError::Unavailable(e.into()))?;

        if resp.status() == reqwest::StatusCode::NOT_FOUND {
            return Err(OriginError::NotFound);
        }
        if !resp.status().is_success() {
            return Err(OriginError::Unavailable(anyhow::anyhow!(
                "unexpected status {}",
                resp.status()
            )));
        }

        let content_type = resp
            .headers()
            .get(reqwest::header::CONTENT_TYPE)
            .and_then(|v| v.to_str().ok())
            .unwrap_or("application/octet-stream")
            .to_string();

        let etag = resp
            .headers()
            .get(reqwest::header::ETAG)
            .and_then(|v| v.to_str().ok())
            .map(str::to_string);

        let body = Box::pin(
            resp.bytes_stream()
                .map(|r| r.map_err(std::io::Error::other)),
        );

        Ok(OriginResponse {
            content_type,
            etag,
            body,
        })
    }
}
