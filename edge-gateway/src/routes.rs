#[apigate::service]
pub mod edge_cache {
    #[apigate::get("/download/{file_id}", policy = "file_sticky")]
    async fn download() {}
}
