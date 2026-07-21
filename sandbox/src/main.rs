use sandbox::{http, Config};

const BIND_ADDR: &str = "0.0.0.0:8070";

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "sandbox=info,tower_http=info".into()),
        )
        .init();

    let cfg = Config::from_env();
    tracing::info!(
        max_timeout_s = cfg.max_timeout_s,
        max_output_bytes = cfg.max_output_bytes,
        addr = BIND_ADDR,
        "starting sandbox"
    );

    let listener = tokio::net::TcpListener::bind(BIND_ADDR)
        .await
        .expect("bind sandbox listener");
    axum::serve(listener, http::app(cfg))
        .await
        .expect("serve sandbox");
}
