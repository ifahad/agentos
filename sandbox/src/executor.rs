//! Process executor: runs Python code in an isolated child process.

use std::process::Stdio;
use std::time::{Duration, Instant};

use tokio::io::{AsyncRead, AsyncReadExt, AsyncWriteExt};
use tokio::process::Command;

use crate::Config;

/// Restricted PATH the child sees; the only environment variable set.
const CHILD_PATH: &str = "/usr/local/bin:/usr/bin:/bin";
/// RLIMIT_AS: 512 MiB of address space.
const LIMIT_AS_BYTES: u64 = 512 * 1024 * 1024;
/// RLIMIT_NPROC: max processes for the executing user.
const LIMIT_NPROC: u64 = 64;
/// RLIMIT_FSIZE: 8 MiB max file size the child may create.
const LIMIT_FSIZE_BYTES: u64 = 8 * 1024 * 1024;

/// Outcome of a sandboxed execution. Field names are the frozen wire contract.
#[derive(Debug, serde::Serialize)]
pub struct ExecutionResult {
    pub exit_code: i32,
    pub stdout: String,
    pub stderr: String,
    pub duration_ms: u64,
    pub timed_out: bool,
    pub truncated: bool,
}

/// Execute `code` with python3 under per-run isolation.
///
/// `timeout_s` must already be clamped via [`Config::effective_timeout_s`].
pub async fn execute_python(
    cfg: &Config,
    code: &str,
    timeout_s: u64,
    stdin: &str,
) -> std::io::Result<ExecutionResult> {
    // Fresh working directory, removed when `workdir` drops.
    let workdir = tempfile::tempdir()?;
    let script = workdir.path().join("main.py");
    tokio::fs::write(&script, code).await?;

    let mut command = Command::new("python3");
    command
        .arg("main.py")
        .current_dir(workdir.path())
        .env_clear()
        .env("PATH", CHILD_PATH)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .kill_on_drop(true);

    // SAFETY: pre_exec runs after fork() in the child, before exec().
    // Only async-signal-safe calls are made (setpgid, setrlimit).
    unsafe {
        command.pre_exec(move || {
            if libc::setpgid(0, 0) != 0 {
                return Err(std::io::Error::last_os_error());
            }
            set_rlimit(libc::RLIMIT_CPU, timeout_s)?;
            set_rlimit(libc::RLIMIT_AS, LIMIT_AS_BYTES)?;
            set_rlimit(libc::RLIMIT_NPROC, LIMIT_NPROC)?;
            set_rlimit(libc::RLIMIT_FSIZE, LIMIT_FSIZE_BYTES)?;
            Ok(())
        });
    }

    let start = Instant::now();
    let mut child = command.spawn()?;
    // The child is its own process group leader (setpgid(0,0) above), so its
    // pid doubles as the pgid for killpg.
    let pgid = child.id().map(|pid| pid as libc::pid_t);

    // Feed stdin, then close it so the child sees EOF.
    if let Some(mut child_stdin) = child.stdin.take() {
        let _ = child_stdin.write_all(stdin.as_bytes()).await;
        drop(child_stdin);
    }

    // Drain stdout/stderr concurrently with the wait: readers cap what they
    // keep but keep draining, so a chatty child never deadlocks on a full pipe
    // and we never buffer unbounded output.
    let cap = cfg.max_output_bytes;
    let stdout_task = tokio::spawn(read_capped(child.stdout.take(), cap));
    let stderr_task = tokio::spawn(read_capped(child.stderr.take(), cap));

    let wait = tokio::time::timeout(Duration::from_secs(timeout_s), child.wait()).await;
    let (timed_out, exit_code) = match wait {
        Ok(status) => (false, status?.code().unwrap_or(-1)),
        Err(_elapsed) => {
            kill_process_group(pgid);
            // Reap the child; it is gone after SIGKILL.
            let _ = child.wait().await;
            (true, -1)
        }
    };
    let duration_ms = start.elapsed().as_millis() as u64;

    let (stdout, stdout_truncated) = stdout_task.await.unwrap_or_default();
    let (stderr, stderr_truncated) = stderr_task.await.unwrap_or_default();

    Ok(ExecutionResult {
        exit_code,
        stdout: String::from_utf8_lossy(&stdout).into_owned(),
        stderr: String::from_utf8_lossy(&stderr).into_owned(),
        duration_ms,
        timed_out,
        truncated: stdout_truncated || stderr_truncated,
    })
}

/// Read a child stream to EOF, keeping at most `cap` bytes.
///
/// Bytes past the cap are drained and discarded (never buffered) so the child
/// cannot block on a full pipe; the second element reports whether anything
/// was discarded.
async fn read_capped<R>(reader: Option<R>, cap: usize) -> (Vec<u8>, bool)
where
    R: AsyncRead + Unpin,
{
    let Some(mut reader) = reader else {
        return (Vec::new(), false);
    };
    let mut kept = Vec::new();
    let mut truncated = false;
    let mut chunk = [0u8; 8192];
    loop {
        match reader.read(&mut chunk).await {
            Ok(0) | Err(_) => break,
            Ok(n) => {
                let room = cap.saturating_sub(kept.len());
                let take = room.min(n);
                kept.extend_from_slice(&chunk[..take]);
                if take < n {
                    truncated = true;
                }
            }
        }
    }
    (kept, truncated)
}

/// SIGKILL the whole process group so grandchildren die with the child.
fn kill_process_group(pgid: Option<libc::pid_t>) {
    if let Some(pgid) = pgid {
        // SAFETY: plain syscall wrapper; a stale pgid at worst returns ESRCH.
        unsafe {
            libc::killpg(pgid, libc::SIGKILL);
        }
    }
}

/// Set one rlimit (soft = hard = `value`) in the pre-exec child context.
///
/// # Safety
/// Must only be called from a `pre_exec` closure (post-fork, pre-exec).
unsafe fn set_rlimit(resource: libc::__rlimit_resource_t, value: u64) -> std::io::Result<()> {
    let limit = libc::rlimit {
        rlim_cur: value,
        rlim_max: value,
    };
    if libc::setrlimit(resource, &limit) != 0 {
        return Err(std::io::Error::last_os_error());
    }
    Ok(())
}
