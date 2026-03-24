use std::io;
use std::sync::Arc;
use std::time::{SystemTime, UNIX_EPOCH};

use edge_cache_service::application::download::{DownloadAction, DownloadError, DownloadUseCase};
use edge_cache_service::ports::cache::{CacheError, CacheMeta, CachedFile};
use edge_cache_service::ports::origin::{ConditionalGetResult, OriginError, OriginResponse};
use futures_util::stream;

use crate::support::mocks::{MockCacheRepo, MockCacheWriter, MockOriginClient};

fn now_unix() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs() as i64
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

fn make_cached_file() -> CachedFile {
    CachedFile {
        meta: CacheMeta {
            content_type: "image/png".to_string(),
            content_length: 1024,
            expires_at: now_unix() + 3600,
            etag: None,
        },
        stream: Box::pin(stream::empty()),
    }
}

fn make_expired_cached_file() -> CachedFile {
    CachedFile {
        meta: CacheMeta {
            content_type: "image/png".to_string(),
            content_length: 1024,
            expires_at: now_unix() - 1,
            etag: None,
        },
        stream: Box::pin(stream::empty()),
    }
}

fn make_expired_cached_file_with_etag() -> CachedFile {
    CachedFile {
        meta: CacheMeta {
            content_type: "image/png".to_string(),
            content_length: 1024,
            expires_at: now_unix() - 1,
            etag: Some("\"etag-123\"".to_string()),
        },
        stream: Box::pin(stream::empty()),
    }
}

fn make_origin_response() -> OriginResponse {
    OriginResponse {
        content_type: "application/octet-stream".to_string(),
        etag: None,
        body: Box::pin(stream::empty()),
    }
}

fn make_writer() -> MockCacheWriter {
    MockCacheWriter::new()
}

fn make_use_case(cache: MockCacheRepo, origin: MockOriginClient) -> DownloadUseCase {
    DownloadUseCase::new(Arc::new(cache), Arc::new(origin))
}

/// Use case with no expectations — for validation-only tests.
fn use_case_bare() -> DownloadUseCase {
    make_use_case(MockCacheRepo::new(), MockOriginClient::new())
}

/// Pre-configured mocks for the standard MISS path:
/// lookup → None, fetch → Ok, begin_write → Ok.
fn mocks_for_miss() -> (MockCacheRepo, MockOriginClient) {
    let mut cache = MockCacheRepo::new();
    let mut origin = MockOriginClient::new();

    cache.expect_lookup().returning(|_| Ok(None));
    origin
        .expect_fetch()
        .returning(|_| Ok(make_origin_response()));
    cache
        .expect_begin_write()
        .returning(|_| Ok(Box::new(make_writer())));

    (cache, origin)
}

// ===========================================================================
// Validation
// ===========================================================================

mod validation {
    use super::*;

    #[tokio::test]
    async fn rejects_invalid_file_ids() {
        let uc = use_case_bare();

        for (id, label) in [
            ("", "empty"),
            (".", "dot"),
            ("..", "double dot"),
            ("../etc/passwd", "path traversal"),
            ("a\\b", "backslash"),
            ("file name", "spaces"),
            ("a@b", "special chars"),
        ] {
            let result = uc.execute(id).await;
            assert!(
                matches!(result, Err(DownloadError::InvalidFileId)),
                "{label}: expected InvalidFileId for {id:?}"
            );
        }
    }

    #[tokio::test]
    async fn accepts_valid_file_ids() {
        // given
        let (cache, origin) = mocks_for_miss();
        let uc = make_use_case(cache, origin);

        // when / then
        for id in ["abc", "test-file-123", "file.txt", "a..b", "A_B.c-3"] {
            let result = uc.execute(id).await;
            assert!(result.is_ok(), "expected Ok for file_id={id:?}");
        }
    }
}

// ===========================================================================
// Cache HIT
// ===========================================================================

mod cache_hit {
    use super::*;

    #[tokio::test]
    async fn returns_hit_when_file_is_cached() {
        // given
        let mut cache = MockCacheRepo::new();
        cache
            .expect_lookup()
            .withf(|id| id == "photo.png")
            .times(1)
            .returning(|_| Ok(Some(make_cached_file())));

        let uc = make_use_case(cache, MockOriginClient::new());

        // when
        let action = uc.execute("photo.png").await.expect("should succeed");

        // then
        match action {
            DownloadAction::Hit(f) => assert_eq!(f.meta.content_type, "image/png"),
            _ => panic!("expected Hit"),
        }
    }

    #[tokio::test]
    async fn does_not_call_origin_on_hit() {
        // given
        let mut cache = MockCacheRepo::new();
        let mut origin = MockOriginClient::new();

        cache
            .expect_lookup()
            .returning(|_| Ok(Some(make_cached_file())));
        origin.expect_fetch().never();

        let uc = make_use_case(cache, origin);

        // when
        let _ = uc.execute("cached-file").await;
    }

    #[tokio::test]
    async fn preserves_cached_metadata() {
        // given
        let mut cache = MockCacheRepo::new();
        cache.expect_lookup().returning(|_| {
            Ok(Some(CachedFile {
                meta: CacheMeta {
                    content_type: "video/mp4".to_string(),
                    content_length: 999_999,
                    expires_at: now_unix() + 3600,
                    etag: Some("\"abc123\"".to_string()),
                },
                stream: Box::pin(stream::empty()),
            }))
        });

        let uc = make_use_case(cache, MockOriginClient::new());

        // when
        let action = uc.execute("movie.mp4").await.unwrap();

        // then
        match action {
            DownloadAction::Hit(f) => {
                assert_eq!(f.meta.content_type, "video/mp4");
                assert_eq!(f.meta.content_length, 999_999);
            }
            _ => panic!("expected Hit"),
        }
    }
}

// ===========================================================================
// Cache MISS — normal flow
// ===========================================================================

mod cache_miss {
    use super::*;

    #[tokio::test]
    async fn fetches_from_origin_when_cache_empty() {
        // given
        let mut cache = MockCacheRepo::new();
        let mut origin = MockOriginClient::new();

        cache.expect_lookup().returning(|_| Ok(None));
        origin
            .expect_fetch()
            .withf(|id| id == "new-file")
            .times(1)
            .returning(|_| Ok(make_origin_response()));
        cache
            .expect_begin_write()
            .withf(|id| id == "new-file")
            .times(1)
            .returning(|_| Ok(Box::new(make_writer())));

        let uc = make_use_case(cache, origin);

        // when
        let action = uc.execute("new-file").await.expect("should succeed");

        // then
        match action {
            DownloadAction::Miss { origin, .. } => {
                assert_eq!(origin.content_type, "application/octet-stream");
            }
            _ => panic!("expected Miss"),
        }
    }

    #[tokio::test]
    async fn returns_writer_in_miss_action() {
        // given
        let (cache, origin) = mocks_for_miss();
        let uc = make_use_case(cache, origin);

        // when
        let action = uc.execute("abc").await.unwrap();

        // then
        assert!(matches!(action, DownloadAction::Miss { .. }));
    }
}

// ===========================================================================
// Graceful degradation — cache errors treated as miss
// ===========================================================================

mod graceful_degradation {
    use super::*;

    #[tokio::test]
    async fn cache_io_error_falls_through_to_origin() {
        // given
        let mut cache = MockCacheRepo::new();
        let mut origin = MockOriginClient::new();

        cache.expect_lookup().returning(|_| {
            Err(CacheError::Io(io::Error::new(
                io::ErrorKind::PermissionDenied,
                "disk error",
            )))
        });
        origin
            .expect_fetch()
            .times(1)
            .returning(|_| Ok(make_origin_response()));
        cache
            .expect_begin_write()
            .returning(|_| Ok(Box::new(make_writer())));

        let uc = make_use_case(cache, origin);

        // when
        let result = uc.execute("some-file").await;

        // then
        assert!(matches!(result, Ok(DownloadAction::Miss { .. })));
    }

    #[tokio::test]
    async fn corrupted_meta_falls_through_to_origin() {
        // given
        let mut cache = MockCacheRepo::new();
        let mut origin = MockOriginClient::new();

        cache.expect_lookup().returning(|_| {
            let serde_err = serde_json::from_str::<serde_json::Value>("not json").unwrap_err();
            Err(CacheError::CorruptedMeta(serde_err))
        });
        origin
            .expect_fetch()
            .times(1)
            .returning(|_| Ok(make_origin_response()));
        cache
            .expect_begin_write()
            .returning(|_| Ok(Box::new(make_writer())));

        let uc = make_use_case(cache, origin);

        // when
        let result = uc.execute("broken-meta").await;

        // then
        assert!(matches!(result, Ok(DownloadAction::Miss { .. })));
    }
}

// ===========================================================================
// Origin errors
// ===========================================================================

mod origin_errors {
    use super::*;

    #[tokio::test]
    async fn origin_not_found_returns_error() {
        // given
        let mut cache = MockCacheRepo::new();
        let mut origin = MockOriginClient::new();

        cache.expect_lookup().returning(|_| Ok(None));
        origin
            .expect_fetch()
            .returning(|_| Err(OriginError::NotFound));

        let uc = make_use_case(cache, origin);

        // when
        let result = uc.execute("missing").await;

        // then
        assert!(matches!(
            result,
            Err(DownloadError::Origin(OriginError::NotFound))
        ));
    }

    #[tokio::test]
    async fn origin_unavailable_returns_error_with_message() {
        // given
        let mut cache = MockCacheRepo::new();
        let mut origin = MockOriginClient::new();

        cache.expect_lookup().returning(|_| Ok(None));
        origin.expect_fetch().returning(|_| {
            Err(OriginError::Unavailable(anyhow::anyhow!(
                "connection refused"
            )))
        });

        let uc = make_use_case(cache, origin);

        // when
        let result = uc.execute("unavailable").await;

        // then
        match result {
            Err(DownloadError::Origin(OriginError::Unavailable(err))) => {
                assert_eq!(err.to_string(), "connection refused");
            }
            Err(other) => panic!("expected Origin(Unavailable), got err={other:?}"),
            Ok(_) => panic!("expected error, got Ok"),
        }
    }

    #[tokio::test]
    async fn origin_error_does_not_call_begin_write() {
        // given
        let mut cache = MockCacheRepo::new();
        let mut origin = MockOriginClient::new();

        cache.expect_lookup().returning(|_| Ok(None));
        origin
            .expect_fetch()
            .returning(|_| Err(OriginError::NotFound));
        cache.expect_begin_write().never();

        let uc = make_use_case(cache, origin);

        // when
        let _ = uc.execute("missing").await;
    }
}

// ===========================================================================
// Cache write errors
// ===========================================================================

mod cache_write_errors {
    use super::*;

    #[tokio::test]
    async fn begin_write_failure_returns_cache_error() {
        // given
        let mut cache = MockCacheRepo::new();
        let mut origin = MockOriginClient::new();

        cache.expect_lookup().returning(|_| Ok(None));
        origin
            .expect_fetch()
            .returning(|_| Ok(make_origin_response()));
        cache.expect_begin_write().returning(|_| {
            Err(CacheError::Io(io::Error::new(
                io::ErrorKind::ReadOnlyFilesystem,
                "read-only",
            )))
        });

        let uc = make_use_case(cache, origin);

        // when
        let result = uc.execute("write-fail").await;

        // then
        assert!(matches!(
            result,
            Err(DownloadError::Cache(CacheError::Io(_)))
        ));
    }
}

// ===========================================================================
// Cache expiration — expires_at check in use case
// ===========================================================================

mod cache_expiration {
    use super::*;

    #[tokio::test]
    async fn expired_cache_entry_refetches_as_revalidated() {
        // given — expired without etag → full re-fetch, REVALIDATED
        let mut cache = MockCacheRepo::new();
        let mut origin = MockOriginClient::new();

        cache
            .expect_lookup()
            .returning(|_| Ok(Some(make_expired_cached_file())));
        origin
            .expect_fetch()
            .times(1)
            .returning(|_| Ok(make_origin_response()));
        cache
            .expect_begin_write()
            .returning(|_| Ok(Box::new(make_writer())));

        let uc = make_use_case(cache, origin);

        // when
        let result = uc.execute("stale-file").await;

        // then
        assert!(matches!(
            result,
            Ok(DownloadAction::RevalidatedWithNewContent { .. })
        ));
    }

    #[tokio::test]
    async fn non_expired_cache_entry_returns_hit() {
        // given
        let mut cache = MockCacheRepo::new();
        cache
            .expect_lookup()
            .returning(|_| Ok(Some(make_cached_file())));

        let uc = make_use_case(cache, MockOriginClient::new());

        // when
        let action = uc.execute("fresh-file").await.unwrap();

        // then
        assert!(matches!(action, DownloadAction::Hit(_)));
    }
}

// ===========================================================================
// Revalidation — conditional GET on expired cache with etag
// ===========================================================================

mod revalidation {
    use super::*;

    #[tokio::test]
    async fn expired_with_etag_304_returns_revalidated() {
        // given
        let mut cache = MockCacheRepo::new();
        let mut origin = MockOriginClient::new();

        cache
            .expect_lookup()
            .returning(|_| Ok(Some(make_expired_cached_file_with_etag())));
        origin
            .expect_fetch_if_changed()
            .withf(|_, etag| etag == "\"etag-123\"")
            .times(1)
            .returning(|_, _| Ok(ConditionalGetResult::NotModified));
        cache.expect_refresh_ttl().times(1).returning(|_| Ok(()));

        let uc = make_use_case(cache, origin);

        // when
        let action = uc.execute("stale").await.unwrap();

        // then
        assert!(matches!(action, DownloadAction::Revalidated(_)));
    }

    #[tokio::test]
    async fn expired_with_etag_200_returns_revalidated_with_new_content() {
        // given
        let mut cache = MockCacheRepo::new();
        let mut origin = MockOriginClient::new();

        cache
            .expect_lookup()
            .returning(|_| Ok(Some(make_expired_cached_file_with_etag())));
        origin.expect_fetch_if_changed().returning(|_, _| {
            Ok(ConditionalGetResult::Modified(OriginResponse {
                content_type: "image/webp".to_string(),
                etag: Some("\"new-etag\"".to_string()),
                body: Box::pin(stream::empty()),
            }))
        });
        cache
            .expect_begin_write()
            .returning(|_| Ok(Box::new(make_writer())));

        let uc = make_use_case(cache, origin);

        // when
        let action = uc.execute("changed").await.unwrap();

        // then
        assert!(matches!(
            action,
            DownloadAction::RevalidatedWithNewContent { .. }
        ));
    }

    #[tokio::test]
    async fn expired_without_etag_returns_revalidated_with_new_content() {
        // given — expired, no etag → full re-fetch, but still REVALIDATED (not MISS)
        let mut cache = MockCacheRepo::new();
        let mut origin = MockOriginClient::new();

        cache
            .expect_lookup()
            .returning(|_| Ok(Some(make_expired_cached_file())));
        origin.expect_fetch_if_changed().never();
        origin
            .expect_fetch()
            .times(1)
            .returning(|_| Ok(make_origin_response()));
        cache
            .expect_begin_write()
            .returning(|_| Ok(Box::new(make_writer())));

        let uc = make_use_case(cache, origin);

        // when
        let action = uc.execute("no-etag").await.unwrap();

        // then
        assert!(matches!(
            action,
            DownloadAction::RevalidatedWithNewContent { .. }
        ));
    }

    #[tokio::test]
    async fn revalidation_304_does_not_call_begin_write() {
        // given
        let mut cache = MockCacheRepo::new();
        let mut origin = MockOriginClient::new();

        cache
            .expect_lookup()
            .returning(|_| Ok(Some(make_expired_cached_file_with_etag())));
        origin
            .expect_fetch_if_changed()
            .returning(|_, _| Ok(ConditionalGetResult::NotModified));
        cache.expect_refresh_ttl().returning(|_| Ok(()));
        cache.expect_begin_write().never();

        let uc = make_use_case(cache, origin);

        // when
        let _ = uc.execute("no-write").await;
    }

    #[tokio::test]
    async fn revalidation_200_does_not_call_refresh_ttl() {
        // given
        let mut cache = MockCacheRepo::new();
        let mut origin = MockOriginClient::new();

        cache
            .expect_lookup()
            .returning(|_| Ok(Some(make_expired_cached_file_with_etag())));
        origin
            .expect_fetch_if_changed()
            .returning(|_, _| Ok(ConditionalGetResult::Modified(make_origin_response())));
        cache.expect_refresh_ttl().never();
        cache
            .expect_begin_write()
            .returning(|_| Ok(Box::new(make_writer())));

        let uc = make_use_case(cache, origin);

        // when
        let _ = uc.execute("new-content").await;
    }

    #[tokio::test]
    async fn revalidation_origin_error_propagates() {
        // given
        let mut cache = MockCacheRepo::new();
        let mut origin = MockOriginClient::new();

        cache
            .expect_lookup()
            .returning(|_| Ok(Some(make_expired_cached_file_with_etag())));
        origin
            .expect_fetch_if_changed()
            .returning(|_, _| Err(OriginError::Unavailable(anyhow::anyhow!("timeout"))));

        let uc = make_use_case(cache, origin);

        // when
        let result = uc.execute("err").await;

        // then
        assert!(matches!(
            result,
            Err(DownloadError::Origin(OriginError::Unavailable(_)))
        ));
    }
}
