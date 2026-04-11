mod routes;
mod settings;

use apigate::{App, Policy};

use settings::GatewaySettings;

#[tokio::main]
async fn main() {
    let settings = GatewaySettings::load().expect("failed to load gateway settings");
    let backends = settings.backend_urls();

    let app = App::builder()
        .mount_service(routes::edge_cache::routes(), &backends)
        .policy("file_sticky", Policy::path_sticky("file_id"))
        .request_timeout(settings.request_timeout)
        .build()
        .expect("failed to build app");

    println!("edge-gateway listening on {}", settings.listen_addr);
    apigate::run(settings.listen_addr, app)
        .await
        .expect("failed to run app");
}
