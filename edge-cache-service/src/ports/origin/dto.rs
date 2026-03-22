use crate::ports::common::ByteStream;

pub struct OriginResponse {
    pub content_type: String,
    pub body: ByteStream,
}
