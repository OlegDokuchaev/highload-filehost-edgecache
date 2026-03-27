use crate::ports::common::ByteStream;

pub struct OriginResponse {
    pub content_type: String,
    pub etag: Option<String>,
    pub max_age: Option<u64>,
    pub body: ByteStream,
}

pub enum ConditionalGetResult {
    NotModified { max_age: Option<u64> },
    Modified(OriginResponse),
}
