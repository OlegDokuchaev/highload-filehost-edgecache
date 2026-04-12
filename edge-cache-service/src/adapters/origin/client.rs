use async_trait::async_trait;
use futures_util::StreamExt;

use crate::adapters::LogOnErr;
use crate::ports::origin::{ConditionalGetResult, OriginClient, OriginError, OriginResponse};

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

    async fn send_get(
        &self,
        file_id: &str,
        if_none_match: Option<&str>,
    ) -> Result<reqwest::Response, OriginError> {
        let url = format!("{}/files/{}", self.base_url, file_id);
        let mut req = self.client.get(&url);
        if let Some(etag) = if_none_match {
            req = req.header(reqwest::header::IF_NONE_MATCH, etag);
        }

        let resp = req
            .send()
            .await
            .map_err(|e| OriginError::Unavailable(e.into()))
            .error_on_err("origin request failed")?;

        if resp.status() == reqwest::StatusCode::NOT_FOUND {
            return Err(OriginError::NotFound);
        }

        Ok(resp)
    }

    fn parse_success_response(resp: reqwest::Response) -> OriginResponse {
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

        let max_age = resp
            .headers()
            .get(reqwest::header::CACHE_CONTROL)
            .and_then(|v| v.to_str().ok())
            .and_then(parse_cache_ttl);

        let body = Box::pin(
            resp.bytes_stream()
                .map(|r| r.map_err(std::io::Error::other)),
        );

        OriginResponse {
            content_type,
            etag,
            max_age,
            body,
        }
    }

    fn check_success(resp: &reqwest::Response) -> Result<(), OriginError> {
        if !resp.status().is_success() {
            Err(anyhow::anyhow!("unexpected status {}", resp.status()))
                .error_on_err("origin returned unexpected status")?;
        }
        Ok(())
    }
}

#[async_trait]
impl OriginClient for OriginClientImpl {
    async fn fetch(&self, file_id: &str) -> Result<OriginResponse, OriginError> {
        let resp = self.send_get(file_id, None).await?;
        Self::check_success(&resp)?;
        Ok(Self::parse_success_response(resp))
    }

    async fn fetch_if_changed(
        &self,
        file_id: &str,
        etag: &str,
    ) -> Result<ConditionalGetResult, OriginError> {
        let resp = self.send_get(file_id, Some(etag)).await?;

        if resp.status() == reqwest::StatusCode::NOT_MODIFIED {
            let max_age = resp
                .headers()
                .get(reqwest::header::CACHE_CONTROL)
                .and_then(|v| v.to_str().ok())
                .and_then(parse_cache_ttl);
            return Ok(ConditionalGetResult::NotModified { max_age });
        }
        Self::check_success(&resp)?;

        Ok(ConditionalGetResult::Modified(
            Self::parse_success_response(resp),
        ))
    }
}

/// Extracts TTL from Cache-Control header.
/// Priority: s-maxage > max-age.
fn parse_cache_ttl(header: &str) -> Option<u64> {
    let mut max_age = None;

    for part in header.split(',').map(str::trim) {
        let Some((name, value)) = part.split_once('=') else {
            continue;
        };

        let name = name.trim();

        if name.eq_ignore_ascii_case("s-maxage") {
            return value.trim().parse().ok();
        }
        if max_age.is_none() && name.eq_ignore_ascii_case("max-age") {
            max_age = value.trim().parse().ok();
        }
    }

    max_age
}
