//! Integration tests: exercise the axum app and the executor against a real
//! python3 interpreter.

use std::time::Instant;

use axum::body::Body;
use axum::http::{Request, StatusCode};
use http_body_util::BodyExt;
use sandbox::executor::execute_python;
use sandbox::{http, Config};
use serde_json::{json, Value};
use tower::ServiceExt;

async fn post_execute(cfg: Config, body: Value) -> (StatusCode, Value) {
    let app = http::app(cfg);
    let response = app
        .oneshot(
            Request::builder()
                .method("POST")
                .uri("/execute")
                .header("content-type", "application/json")
                .body(Body::from(body.to_string()))
                .unwrap(),
        )
        .await
        .unwrap();
    let status = response.status();
    let bytes = response.into_body().collect().await.unwrap().to_bytes();
    (status, serde_json::from_slice(&bytes).unwrap())
}

#[tokio::test]
async fn healthz_returns_ok() {
    let app = http::app(Config::default());
    let response = app
        .oneshot(
            Request::builder()
                .uri("/healthz")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();
    assert_eq!(response.status(), StatusCode::OK);
    let bytes = response.into_body().collect().await.unwrap().to_bytes();
    assert_eq!(&bytes[..], b"ok");
}

#[tokio::test]
async fn happy_path_prints_to_stdout() {
    let (status, body) = post_execute(
        Config::default(),
        json!({"language": "python", "code": "print('hello sandbox')"}),
    )
    .await;
    assert_eq!(status, StatusCode::OK);
    assert_eq!(body["exit_code"], 0);
    assert_eq!(body["stdout"], "hello sandbox\n");
    assert_eq!(body["stderr"], "");
    assert_eq!(body["timed_out"], false);
    assert_eq!(body["truncated"], false);
    assert!(body["duration_ms"].is_u64());
}

#[tokio::test]
async fn nonzero_exit_with_stderr() {
    let code = "import sys\nsys.stderr.write('boom\\n')\nsys.exit(3)";
    let (status, body) = post_execute(
        Config::default(),
        json!({"language": "python", "code": code}),
    )
    .await;
    assert_eq!(status, StatusCode::OK);
    assert_eq!(body["exit_code"], 3);
    assert_eq!(body["stderr"], "boom\n");
    assert_eq!(body["timed_out"], false);
}

#[tokio::test]
async fn wall_clock_timeout_kills_process_group_quickly() {
    let start = Instant::now();
    let (status, body) = post_execute(
        Config::default(),
        json!({
            "language": "python",
            "code": "import time\ntime.sleep(30)",
            "timeout_s": 1
        }),
    )
    .await;
    let elapsed = start.elapsed();
    assert_eq!(status, StatusCode::OK);
    assert_eq!(body["timed_out"], true);
    assert_eq!(body["exit_code"], -1);
    // Killed promptly: wall time near the 1s timeout, nowhere near 2x.
    assert!(elapsed.as_secs_f64() >= 1.0, "returned before the timeout");
    assert!(
        elapsed.as_secs_f64() < 2.0,
        "kill took too long: {elapsed:?}"
    );
}

#[tokio::test]
async fn output_is_capped_and_flagged_truncated() {
    let cfg = Config::default();
    let cap = cfg.max_output_bytes;
    // ~5 MiB of stdout, far past the 64 KiB cap.
    let code = "print('x' * (5 * 1024 * 1024))";
    let (status, body) = post_execute(cfg, json!({"language": "python", "code": code})).await;
    assert_eq!(status, StatusCode::OK);
    assert_eq!(body["exit_code"], 0);
    assert_eq!(body["truncated"], true);
    assert_eq!(body["timed_out"], false);
    assert!(body["stdout"].as_str().unwrap().len() <= cap);
}

#[tokio::test]
async fn environment_is_cleared_to_path_only() {
    // /proc/self/environ is the exec-time environment, immune to variables
    // CPython itself injects after startup (PEP 538 adds LC_CTYPE when the
    // locale is unset — expected, and excluded from os.environ below).
    let code = concat!(
        "import os\n",
        "raw = open('/proc/self/environ', 'rb').read().split(b'\\0')\n",
        "print(sorted(e.decode() for e in raw if e))\n",
        "print(sorted(k for k in os.environ if k != 'LC_CTYPE'))\n",
    );
    let (status, body) = post_execute(
        Config::default(),
        json!({"language": "python", "code": code}),
    )
    .await;
    assert_eq!(status, StatusCode::OK);
    assert_eq!(body["exit_code"], 0);
    let stdout = body["stdout"].as_str().unwrap();
    let mut lines = stdout.lines();
    assert_eq!(
        lines.next().unwrap(),
        "['PATH=/usr/local/bin:/usr/bin:/bin']"
    );
    assert_eq!(lines.next().unwrap(), "['PATH']");
}

#[tokio::test]
async fn stdin_round_trip() {
    let (status, body) = post_execute(
        Config::default(),
        json!({
            "language": "python",
            "code": "import sys\nprint(sys.stdin.read().upper(), end='')",
            "stdin": "hello agentos\n"
        }),
    )
    .await;
    assert_eq!(status, StatusCode::OK);
    assert_eq!(body["exit_code"], 0);
    assert_eq!(body["stdout"], "HELLO AGENTOS\n");
}

#[tokio::test]
async fn unsupported_language_is_400() {
    let (status, body) = post_execute(
        Config::default(),
        json!({"language": "ruby", "code": "puts 1"}),
    )
    .await;
    assert_eq!(status, StatusCode::BAD_REQUEST);
    assert_eq!(body, json!({"error": "unsupported language"}));
}

#[tokio::test]
async fn timeout_above_max_is_capped() {
    let cfg = Config {
        max_timeout_s: 1,
        ..Config::default()
    };
    let start = Instant::now();
    let (status, body) = post_execute(
        cfg,
        json!({
            "language": "python",
            "code": "import time\ntime.sleep(30)",
            "timeout_s": 300
        }),
    )
    .await;
    let elapsed = start.elapsed();
    assert_eq!(status, StatusCode::OK);
    // A 300s request against max_timeout_s=1 dies at ~1s, proving the cap.
    assert_eq!(body["timed_out"], true);
    assert_eq!(body["exit_code"], -1);
    assert!(elapsed.as_secs_f64() < 2.0, "cap not applied: {elapsed:?}");
}

#[tokio::test]
async fn rlimit_as_blocks_large_allocations() {
    // ~1 GiB allocation must fail under RLIMIT_AS=512 MiB.
    let code = "data = bytearray(1024 * 1024 * 1024)\nprint('allocated')";
    let result = execute_python(&Config::default(), code, 10, "")
        .await
        .expect("executor should not error");
    assert!(!result.timed_out);
    assert_ne!(
        result.exit_code, 0,
        "1 GiB allocation unexpectedly succeeded"
    );
    assert!(
        result.stderr.contains("MemoryError"),
        "expected MemoryError, got: {}",
        result.stderr
    );
    assert!(!result.stdout.contains("allocated"));
}
