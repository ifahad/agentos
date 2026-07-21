//! AgentOS sandbox: executes untrusted Python code with per-run isolation.
//!
//! Frozen contract (Phase 3 plan):
//! - `POST /execute` `{"language","code","timeout_s","stdin"}` →
//!   `{"exit_code","stdout","stderr","duration_ms","timed_out","truncated"}`
//! - `GET /healthz` → `ok`
//!
//! Isolation per execution: fresh temp workdir, cleared environment
//! (`PATH` only), new process group, rlimits (CPU, address space, nproc,
//! file size), wall-clock timeout enforced by SIGKILL to the process group.

pub mod executor;
pub mod http;

/// Runtime configuration, sourced from the environment.
#[derive(Debug, Clone)]
pub struct Config {
    /// Upper bound on `timeout_s` (`AGENTOS_SANDBOX_MAX_TIMEOUT_S`, default 30).
    pub max_timeout_s: u64,
    /// Per-stream stdout/stderr cap in bytes
    /// (`AGENTOS_SANDBOX_MAX_OUTPUT_BYTES`, default 65536).
    pub max_output_bytes: usize,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            max_timeout_s: 30,
            max_output_bytes: 65536,
        }
    }
}

impl Config {
    /// Build a config from `AGENTOS_SANDBOX_*` environment variables.
    pub fn from_env() -> Self {
        let defaults = Self::default();
        Self {
            max_timeout_s: env_parse("AGENTOS_SANDBOX_MAX_TIMEOUT_S", defaults.max_timeout_s),
            max_output_bytes: env_parse(
                "AGENTOS_SANDBOX_MAX_OUTPUT_BYTES",
                defaults.max_output_bytes,
            ),
        }
    }

    /// Clamp a requested timeout to `[1, max_timeout_s]`.
    pub fn effective_timeout_s(&self, requested: u64) -> u64 {
        requested.clamp(1, self.max_timeout_s)
    }
}

fn env_parse<T: std::str::FromStr>(name: &str, default: T) -> T {
    std::env::var(name)
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(default)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn timeout_is_capped_at_max() {
        let cfg = Config {
            max_timeout_s: 30,
            ..Config::default()
        };
        assert_eq!(cfg.effective_timeout_s(100), 30);
        assert_eq!(cfg.effective_timeout_s(10), 10);
        assert_eq!(cfg.effective_timeout_s(0), 1);
    }
}
