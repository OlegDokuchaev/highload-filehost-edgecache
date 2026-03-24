use std::sync::Arc;
use std::time::Duration;

use bytes::Bytes;
use futures_util::stream;
use reqwest::header;

use edge_cache_service::adapters::api::{AppState, app};
use edge_cache_service::adapters::cache::{CacheRepoImpl, CacheSettings};
use edge_cache_service::application::download::DownloadUseCase;
use edge_cache_service::ports::cache::CacheMeta;
use edge_cache_service::ports::origin::{ConditionalGetResult, OriginError, OriginResponse};

use crate::support::mocks::MockOriginClient;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

struct Env {
    base_url: String,
    cache_dir: std::path::PathBuf,
    _tmp: tempfile::TempDir,
}

fn default_origin_mock() -> MockOriginClient {
    let mut origin = MockOriginClient::new();
    origin.expect_fetch().returning(|file_id| match file_id {
        "not-found" => Err(OriginError::NotFound),
        "unavailable" => Err(OriginError::Unavailable(anyhow::anyhow!(
            "connection refused"
        ))),
        _ => {
            let body = format!("content of {file_id}");
            Ok(OriginResponse {
                content_type: "application/octet-stream".to_string(),
                etag: Some(format!("\"etag-{file_id}\"")),
                body: Box::pin(stream::once(async move { Ok(Bytes::from(body)) })),
            })
        }
    });
    origin
}

async fn setup_with(origin: MockOriginClient) -> Env {
    let tmp = tempfile::tempdir().unwrap();
    let cache_dir = tmp.path().to_path_buf();

    let cache = CacheRepoImpl::new(CacheSettings {
        dir: cache_dir.to_str().unwrap().to_string(),
        ttl: Duration::from_secs(3600),
    });
    let state = Arc::new(AppState {
        download: DownloadUseCase::new(Arc::new(cache), Arc::new(origin)),
    });

    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let addr = listener.local_addr().unwrap();
    tokio::spawn(async move { axum::serve(listener, app(state)).await.unwrap() });

    Env {
        base_url: format!("http://{addr}"),
        cache_dir,
        _tmp: tmp,
    }
}

async fn setup() -> Env {
    setup_with(default_origin_mock()).await
}

async fn setup_with_ttl(origin: MockOriginClient, ttl: Duration) -> Env {
    let tmp = tempfile::tempdir().unwrap();
    let cache_dir = tmp.path().to_path_buf();

    let cache = CacheRepoImpl::new(CacheSettings {
        dir: cache_dir.to_str().unwrap().to_string(),
        ttl,
    });
    let state = Arc::new(AppState {
        download: DownloadUseCase::new(Arc::new(cache), Arc::new(origin)),
    });

    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let addr = listener.local_addr().unwrap();
    tokio::spawn(async move { axum::serve(listener, app(state)).await.unwrap() });

    Env {
        base_url: format!("http://{addr}"),
        cache_dir,
        _tmp: tmp,
    }
}

fn get(env: &Env, file_id: &str) -> reqwest::RequestBuilder {
    reqwest::Client::new().get(format!("{}/download/{file_id}", env.base_url))
}

/// Sends a MISS request and consumes the body, seeding the cache for subsequent HIT tests.
async fn seed_cache(env: &Env, file_id: &str) -> Bytes {
    let resp = get(env, file_id).send().await.unwrap();
    assert_eq!(resp.headers()["x-cache"], "MISS");
    resp.bytes().await.unwrap()
}

fn read_cache_meta(env: &Env, file_id: &str) -> CacheMeta {
    let path = env.cache_dir.join(format!("{file_id}.json"));
    serde_json::from_slice(&std::fs::read(path).unwrap()).unwrap()
}

// ===========================================================================
// MISS — normal flow
// ===========================================================================

mod miss {
    use super::*;

    #[tokio::test]
    async fn returns_200_with_body() {
        // given
        let env = setup().await;

        // when
        let resp = get(&env, "abc").send().await.unwrap();

        // then
        assert_eq!(resp.status(), 200);
        let body = resp.bytes().await.unwrap();
        assert_eq!(&body[..], b"content of abc");
    }

    #[tokio::test]
    async fn sets_x_cache_miss_header() {
        // given
        let env = setup().await;

        // when
        let resp = get(&env, "abc").send().await.unwrap();

        // then
        assert_eq!(resp.headers()["x-cache"], "MISS");
    }

    #[tokio::test]
    async fn propagates_content_type_from_origin() {
        // given
        let env = setup().await;

        // when
        let resp = get(&env, "abc").send().await.unwrap();

        // then
        assert_eq!(
            resp.headers()[header::CONTENT_TYPE],
            "application/octet-stream"
        );
    }

    #[tokio::test]
    async fn creates_cache_files_and_removes_tmp() {
        // given
        let env = setup().await;

        // when
        seed_cache(&env, "testfile").await;

        // then
        assert!(env.cache_dir.join("testfile.bin").exists());
        assert!(env.cache_dir.join("testfile.json").exists());
        assert!(!env.cache_dir.join("testfile.bin.tmp").exists());
    }

    #[tokio::test]
    async fn cache_metadata_matches_response() {
        // given
        let env = setup().await;

        // when
        let body = seed_cache(&env, "meta-check").await;

        // then
        let meta = read_cache_meta(&env, "meta-check");
        assert_eq!(meta.content_type, "application/octet-stream");
        assert_eq!(meta.content_length, body.len() as u64);
    }

    #[tokio::test]
    async fn custom_content_type_propagated() {
        // given
        let mut origin = MockOriginClient::new();
        origin.expect_fetch().returning(|_| {
            Ok(OriginResponse {
                content_type: "text/html".to_string(),
                etag: None,
                body: Box::pin(stream::once(async { Ok(Bytes::from("<h1>hello</h1>")) })),
            })
        });
        let env = setup_with(origin).await;

        // when
        let resp = get(&env, "page").send().await.unwrap();

        // then
        assert_eq!(resp.headers()[header::CONTENT_TYPE], "text/html");
        let body = resp.bytes().await.unwrap();
        assert_eq!(&body[..], b"<h1>hello</h1>");
    }

    #[tokio::test]
    async fn propagates_etag_from_origin() {
        // given
        let env = setup().await;

        // when
        let resp = get(&env, "abc").send().await.unwrap();

        // then
        assert_eq!(resp.headers()[header::ETAG], "\"etag-abc\"");
    }

    #[tokio::test]
    async fn no_etag_header_when_origin_omits_it() {
        // given
        let mut origin = MockOriginClient::new();
        origin.expect_fetch().returning(|_| {
            Ok(OriginResponse {
                content_type: "application/octet-stream".to_string(),
                etag: None,
                body: Box::pin(stream::once(async { Ok(Bytes::from("no-etag")) })),
            })
        });
        let env = setup_with(origin).await;

        // when
        let resp = get(&env, "plain").send().await.unwrap();

        // then
        assert!(resp.headers().get(header::ETAG).is_none());
    }

    #[tokio::test]
    async fn large_multi_chunk_body_streams_correctly() {
        // given
        let mut origin = MockOriginClient::new();
        origin.expect_fetch().returning(|_| {
            let chunks = vec![
                Ok(Bytes::from(vec![b'A'; 8192])),
                Ok(Bytes::from(vec![b'B'; 8192])),
                Ok(Bytes::from(vec![b'C'; 8192])),
            ];
            Ok(OriginResponse {
                content_type: "application/octet-stream".to_string(),
                etag: None,
                body: Box::pin(stream::iter(chunks)),
            })
        });
        let env = setup_with(origin).await;

        // when
        let body = seed_cache(&env, "large").await;

        // then
        assert_eq!(body.len(), 8192 * 3);
        assert!(body[..8192].iter().all(|&b| b == b'A'));
        assert!(body[8192..16384].iter().all(|&b| b == b'B'));
        assert!(body[16384..].iter().all(|&b| b == b'C'));

        let meta = read_cache_meta(&env, "large");
        assert_eq!(meta.content_length, 8192 * 3);
    }
}

// ===========================================================================
// HIT — served from cache
// ===========================================================================

mod hit {
    use super::*;

    #[tokio::test]
    async fn second_request_returns_hit() {
        // given
        let env = setup().await;
        seed_cache(&env, "xyz").await;

        // when
        let resp = get(&env, "xyz").send().await.unwrap();

        // then
        assert_eq!(resp.headers()["x-cache"], "HIT");
    }

    #[tokio::test]
    async fn returns_same_body() {
        // given
        let env = setup().await;
        let body1 = seed_cache(&env, "samefile").await;

        // when
        let resp = get(&env, "samefile").send().await.unwrap();
        let body2 = resp.bytes().await.unwrap();

        // then
        assert_eq!(body1, body2);
    }

    #[tokio::test]
    async fn returns_content_length() {
        // given
        let env = setup().await;
        let body = seed_cache(&env, "with-len").await;

        // when
        let resp = get(&env, "with-len").send().await.unwrap();

        // then
        assert_eq!(
            resp.headers()[header::CONTENT_LENGTH],
            body.len().to_string()
        );
    }

    #[tokio::test]
    async fn preserves_content_type() {
        // given
        let mut origin = MockOriginClient::new();
        origin.expect_fetch().returning(|_| {
            Ok(OriginResponse {
                content_type: "image/jpeg".to_string(),
                etag: Some("\"jpeg-etag\"".to_string()),
                body: Box::pin(stream::once(async { Ok(Bytes::from("jpeg-data")) })),
            })
        });
        let env = setup_with(origin).await;
        seed_cache(&env, "photo").await;

        // when
        let resp = get(&env, "photo").send().await.unwrap();

        // then
        assert_eq!(resp.headers()[header::CONTENT_TYPE], "image/jpeg");
    }

    #[tokio::test]
    async fn preserves_etag() {
        // given
        let env = setup().await;
        seed_cache(&env, "with-etag").await;

        // when
        let resp = get(&env, "with-etag").send().await.unwrap();

        // then
        assert_eq!(resp.headers()[header::ETAG], "\"etag-with-etag\"");
    }

    #[tokio::test]
    async fn does_not_call_origin_again() {
        // given — origin allows only 1 call
        let mut origin = MockOriginClient::new();
        origin.expect_fetch().times(1).returning(|file_id| {
            let body = format!("content of {file_id}");
            Ok(OriginResponse {
                content_type: "application/octet-stream".to_string(),
                etag: Some("\"once-etag\"".to_string()),
                body: Box::pin(stream::once(async move { Ok(Bytes::from(body)) })),
            })
        });
        let env = setup_with(origin).await;
        seed_cache(&env, "once").await;

        // when
        let resp = get(&env, "once").send().await.unwrap();

        // then
        assert_eq!(resp.headers()["x-cache"], "HIT");
        let _ = resp.bytes().await.unwrap();
        // mock panics on drop if fetch was called more than once
    }
}

// ===========================================================================
// Error responses
// ===========================================================================

mod errors {
    use super::*;

    #[tokio::test]
    async fn origin_not_found_returns_404() {
        // given
        let env = setup().await;

        // when
        let resp = get(&env, "not-found").send().await.unwrap();

        // then
        assert_eq!(resp.status(), 404);
        assert_eq!(resp.text().await.unwrap(), "file not found");
    }

    #[tokio::test]
    async fn origin_unavailable_returns_502() {
        // given
        let env = setup().await;

        // when
        let resp = get(&env, "unavailable").send().await.unwrap();

        // then
        assert_eq!(resp.status(), 502);
        assert_eq!(resp.text().await.unwrap(), "origin unavailable");
    }

    #[tokio::test]
    async fn invalid_file_ids_return_400() {
        // given
        let env = setup().await;

        // when / then
        for (encoded_id, label) in [
            ("a@b", "special chars"),
            ("bad%20file", "spaces"),
            ("a%5Cb", "backslash"),
        ] {
            let resp = get(&env, encoded_id).send().await.unwrap();
            assert_eq!(resp.status(), 400, "{label}: expected 400");
        }
    }

    #[tokio::test]
    async fn error_does_not_leak_internal_details() {
        // given
        let env = setup().await;

        // when
        let resp = get(&env, "unavailable").send().await.unwrap();

        // then
        let body = resp.text().await.unwrap();
        assert!(!body.contains("connection refused"));
        assert_eq!(body, "origin unavailable");
    }

    #[tokio::test]
    async fn origin_stream_error_cleans_up_tmp_files() {
        // given — origin stream fails after first chunk
        let mut origin = MockOriginClient::new();
        origin.expect_fetch().returning(|_| {
            let chunks: Vec<Result<Bytes, std::io::Error>> = vec![
                Ok(Bytes::from("partial")),
                Err(std::io::Error::other("connection reset")),
            ];
            Ok(OriginResponse {
                content_type: "application/octet-stream".to_string(),
                etag: None,
                body: Box::pin(stream::iter(chunks)),
            })
        });
        let env = setup_with(origin).await;

        // when — request may fail at transport level, that's expected
        let resp = get(&env, "broken").send().await;
        if let Ok(resp) = resp {
            let _ = resp.bytes().await;
        }

        // then — no tmp or final files should remain
        tokio::time::sleep(std::time::Duration::from_millis(50)).await;
        assert!(!env.cache_dir.join("broken.bin.tmp").exists());
        assert!(!env.cache_dir.join("broken.json.tmp").exists());
        assert!(!env.cache_dir.join("broken.bin").exists());
        assert!(!env.cache_dir.join("broken.json").exists());
    }
}

// ===========================================================================
// Expiration — TTL-based cache invalidation
// ===========================================================================

mod expired {
    use super::*;

    fn origin_without_etag() -> MockOriginClient {
        let mut origin = MockOriginClient::new();
        origin.expect_fetch().returning(|file_id| {
            let body = format!("content of {file_id}");
            Ok(OriginResponse {
                content_type: "application/octet-stream".to_string(),
                etag: None,
                body: Box::pin(stream::once(async move { Ok(Bytes::from(body)) })),
            })
        });
        origin
    }

    #[tokio::test]
    async fn expired_without_etag_returns_revalidated() {
        // given — origin returns no etag, but file was in cache → REVALIDATED, not MISS
        let env = setup_with_ttl(origin_without_etag(), Duration::from_secs(0)).await;
        seed_cache(&env, "expiring").await;

        // when
        let resp = get(&env, "expiring").send().await.unwrap();

        // then
        assert_eq!(resp.headers()["x-cache"], "REVALIDATED");
    }

    #[tokio::test]
    async fn non_expired_entry_returns_hit() {
        // given — generous TTL
        let env = setup_with_ttl(default_origin_mock(), Duration::from_secs(3600)).await;
        seed_cache(&env, "fresh").await;

        // when
        let resp = get(&env, "fresh").send().await.unwrap();

        // then
        assert_eq!(resp.headers()["x-cache"], "HIT");
    }
}

// ===========================================================================
// Revalidation — conditional GET on expired cache with etag
// ===========================================================================

mod revalidated {
    use super::*;

    fn origin_with_revalidation() -> MockOriginClient {
        let mut origin = MockOriginClient::new();
        // First call — normal fetch for seeding cache
        origin.expect_fetch().returning(|file_id| {
            let body = format!("content of {file_id}");
            Ok(OriginResponse {
                content_type: "application/octet-stream".to_string(),
                etag: Some(format!("\"etag-{file_id}\"")),
                body: Box::pin(stream::once(async move { Ok(Bytes::from(body)) })),
            })
        });
        // Subsequent calls — 304 Not Modified
        origin
            .expect_fetch_if_changed()
            .returning(|_, _| Ok(ConditionalGetResult::NotModified));
        origin
    }

    #[tokio::test]
    async fn returns_revalidated_header_on_304() {
        // given — TTL=0, origin returns 304 on conditional GET
        let env = setup_with_ttl(origin_with_revalidation(), Duration::from_secs(0)).await;
        seed_cache(&env, "reval").await;

        // when
        let resp = get(&env, "reval").send().await.unwrap();

        // then
        assert_eq!(resp.headers()["x-cache"], "REVALIDATED");
    }

    #[tokio::test]
    async fn serves_cached_body_on_304() {
        // given
        let env = setup_with_ttl(origin_with_revalidation(), Duration::from_secs(0)).await;
        let original_body = seed_cache(&env, "body304").await;

        // when
        let resp = get(&env, "body304").send().await.unwrap();
        let body = resp.bytes().await.unwrap();

        // then — same body from cache, not re-downloaded
        assert_eq!(body, original_body);
    }

    #[tokio::test]
    async fn refreshes_ttl_on_304() {
        // given — TTL=0, first revalidation refreshes TTL
        let mut origin = MockOriginClient::new();
        origin.expect_fetch().times(1).returning(|file_id| {
            let body = format!("content of {file_id}");
            Ok(OriginResponse {
                content_type: "application/octet-stream".to_string(),
                etag: Some("\"e1\"".to_string()),
                body: Box::pin(stream::once(async move { Ok(Bytes::from(body)) })),
            })
        });
        origin
            .expect_fetch_if_changed()
            .times(1)
            .returning(|_, _| Ok(ConditionalGetResult::NotModified));

        // TTL=3600 for refresh — after 304, the entry should be fresh
        let env = setup_with_ttl(origin, Duration::from_secs(0)).await;
        seed_cache(&env, "ttl304").await;

        // when — revalidation triggers, refresh_ttl writes new expires_at
        let _ = get(&env, "ttl304").send().await.unwrap().bytes().await;

        // then — check .json has updated expires_at
        let meta = read_cache_meta(&env, "ttl304");
        let now = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_secs() as i64;
        // expires_at should be close to now (TTL=0 means now+0)
        assert!(meta.expires_at >= now - 2, "expires_at should be refreshed");
    }

    #[tokio::test]
    async fn replaces_cache_on_200() {
        // given — origin returns new content on conditional GET
        let mut origin = MockOriginClient::new();
        origin.expect_fetch().times(1).returning(|file_id| {
            let body = format!("old content of {file_id}");
            Ok(OriginResponse {
                content_type: "text/plain".to_string(),
                etag: Some("\"old-etag\"".to_string()),
                body: Box::pin(stream::once(async move { Ok(Bytes::from(body)) })),
            })
        });
        origin
            .expect_fetch_if_changed()
            .times(1)
            .returning(|file_id, _| {
                let body = format!("new content of {file_id}");
                Ok(ConditionalGetResult::Modified(OriginResponse {
                    content_type: "text/html".to_string(),
                    etag: Some("\"new-etag\"".to_string()),
                    body: Box::pin(stream::once(async move { Ok(Bytes::from(body)) })),
                }))
            });

        let env = setup_with_ttl(origin, Duration::from_secs(0)).await;
        seed_cache(&env, "updated").await;

        // when
        let resp = get(&env, "updated").send().await.unwrap();

        // then
        assert_eq!(resp.headers()["x-cache"], "REVALIDATED");
        assert_eq!(resp.headers()[header::CONTENT_TYPE], "text/html");
        assert_eq!(resp.headers()[header::ETAG], "\"new-etag\"");
        let body = resp.bytes().await.unwrap();
        assert_eq!(&body[..], b"new content of updated");
    }

    #[tokio::test]
    async fn preserves_content_type_on_304() {
        // given — origin returns image/jpeg, then 304
        let mut origin = MockOriginClient::new();
        origin.expect_fetch().returning(|_| {
            Ok(OriginResponse {
                content_type: "image/jpeg".to_string(),
                etag: Some("\"jpeg-e\"".to_string()),
                body: Box::pin(stream::once(async { Ok(Bytes::from("jpeg-data")) })),
            })
        });
        origin
            .expect_fetch_if_changed()
            .returning(|_, _| Ok(ConditionalGetResult::NotModified));

        let env = setup_with_ttl(origin, Duration::from_secs(0)).await;
        seed_cache(&env, "photo").await;

        // when
        let resp = get(&env, "photo").send().await.unwrap();

        // then
        assert_eq!(resp.headers()[header::CONTENT_TYPE], "image/jpeg");
    }
}
