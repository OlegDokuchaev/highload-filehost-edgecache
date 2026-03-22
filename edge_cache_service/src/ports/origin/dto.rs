use crate::ports::ByteStream;

pub struct OriginResponse {
    pub content_type: String,
    pub body: ByteStream,
}
