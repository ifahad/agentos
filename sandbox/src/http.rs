//! HTTP layer: the axum router implementing the frozen contract.

use std::sync::Arc;

use axum::extract::State;
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::{get, post};
use axum::{Json, Router};
use serde::Deserialize;
use serde_json::json;

use crate::executor::execute_python;
use crate::Config;

const DEFAULT_TIMEOUT_S: u64 = 10;

/// `POST /execute` request body (frozen contract field names).
#[derive(Debug, Deserialize)]
pub struct ExecuteRequest {
    pub language: String,
    pub code: String,
    #[serde(default = "default_timeout_s")]
    pub timeout_s: u64,
    #[serde(default)]
    pub stdin: String,
}

fn default_timeout_s() -> u64 {
    DEFAULT_TIMEOUT_S
}

/// Build the sandbox router.
pub fn app(cfg: Config) -> Router {
    Router::new()
        .route("/healthz", get(healthz))
        .route("/execute", post(execute))
        .with_state(Arc::new(cfg))
}

async fn healthz() -> &'static str {
    "ok"
}

async fn execute(State(cfg): State<Arc<Config>>, Json(req): Json<ExecuteRequest>) -> Response {
    if req.language != "python" {
        return (
            StatusCode::BAD_REQUEST,
            Json(json!({"error": "unsupported language"})),
        )
            .into_response();
    }

    let timeout_s = cfg.effective_timeout_s(req.timeout_s);
    match execute_python(&cfg, &req.code, timeout_s, &req.stdin).await {
        Ok(result) => Json(result).into_response(),
        Err(err) => {
            tracing::error!(error = %err, "execution failed");
            (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(json!({"error": "execution failed"})),
            )
                .into_response()
        }
    }
}
