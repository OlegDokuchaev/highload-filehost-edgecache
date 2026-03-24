use crate::ports::common::ByteStream;

pub struct OriginResponse {
    pub content_type: String,
    pub etag: Option<String>,
    pub body: ByteStream,
}

pub enum ConditionalGetResult {
    NotModified,
    Modified(OriginResponse),
}
