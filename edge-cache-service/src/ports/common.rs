use std::pin::Pin;

use bytes::Bytes;
use futures_util::Stream;

/// A stream of byte chunks with `io::Error`.
pub type ByteStream = Pin<Box<dyn Stream<Item = Result<Bytes, std::io::Error>> + Send>>;
