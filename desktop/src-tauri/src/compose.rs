//! Compose lifecycle orchestrator for Raven Local.
//!
//! Detects Docker, brings the bundled `docker-compose.local.yml` up on
//! launch, polls `/healthz` until ready, streams logs to a frontend
//! channel, and brings the stack down cleanly on app quit.

use std::path::PathBuf;
use std::process::Stdio;
use std::sync::Arc;
use std::time::Duration;

use serde::{Deserialize, Serialize};
use thiserror::Error;
use tokio::io::{AsyncBufReadExt, BufReader};
use tokio::process::Command;
use tokio::sync::Mutex;

const HEALTH_POLL_INTERVAL: Duration = Duration::from_secs(1);
const HEALTH_POLL_TIMEOUT: Duration = Duration::from_secs(60);

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ComposeStatus {
    Starting,
    Ready,
    Stopped,
    Error,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct StatusEvent {
    pub status: ComposeStatus,
    pub message: Option<String>,
}

impl StatusEvent {
    pub fn starting() -> Self {
        Self { status: ComposeStatus::Starting, message: None }
    }
    pub fn ready() -> Self {
        Self { status: ComposeStatus::Ready, message: None }
    }
    pub fn stopped() -> Self {
        Self { status: ComposeStatus::Stopped, message: None }
    }
    pub fn error(message: impl Into<String>) -> Self {
        Self { status: ComposeStatus::Error, message: Some(message.into()) }
    }
}

#[derive(Debug, Error)]
pub enum DockerError {
    #[error("Docker is not installed or not in PATH")]
    NotInstalled,
    #[error("Docker daemon is not running: {0}")]
    NotRunning(String),
}

#[derive(Debug, Error)]
pub enum ComposeError {
    #[error(transparent)]
    Docker(#[from] DockerError),
    #[error("compose `up` failed (exit {code}): {stderr}")]
    UpFailed { code: i32, stderr: String },
    #[error("compose `down` failed (exit {code}): {stderr}")]
    DownFailed { code: i32, stderr: String },
    #[error("health check timed out after {0:?}")]
    HealthTimeout(Duration),
    #[error("io error: {0}")]
    Io(#[from] std::io::Error),
}

/// Manages the lifecycle of the bundled docker compose stack.
#[derive(Clone)]
pub struct ComposeManager {
    inner: Arc<Inner>,
}

struct Inner {
    /// Directory containing `docker-compose.local.yml`. Used as the working
    /// directory for `docker compose` invocations so relative volume paths
    /// resolve consistently.
    project_dir: PathBuf,
    /// Filename inside `project_dir` (defaults to `docker-compose.local.yml`).
    compose_file: String,
    /// Healthcheck URL to poll once the stack is up.
    api_health_url: String,
    /// Tracks whether we've already brought the stack up so `down()` is safe
    /// to call multiple times.
    started: Mutex<bool>,
}

impl ComposeManager {
    pub fn new(
        project_dir: PathBuf,
        compose_file: impl Into<String>,
        api_health_url: impl Into<String>,
    ) -> Self {
        Self {
            inner: Arc::new(Inner {
                project_dir,
                compose_file: compose_file.into(),
                api_health_url: api_health_url.into(),
                started: Mutex::new(false),
            }),
        }
    }

    pub fn project_dir(&self) -> &std::path::Path {
        &self.inner.project_dir
    }

    pub fn api_health_url(&self) -> &str {
        &self.inner.api_health_url
    }

    /// Verify Docker is installed and the daemon is reachable.
    pub async fn detect_docker(&self) -> Result<(), DockerError> {
        let output = Command::new("docker")
            .args(["version", "--format", "{{.Server.Version}}"])
            .output()
            .await
            .map_err(|_| DockerError::NotInstalled)?;
        if output.status.success() {
            Ok(())
        } else {
            let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
            Err(DockerError::NotRunning(stderr))
        }
    }

    /// `docker compose up -d`. Returns once compose returns; does NOT wait
    /// for healthy — call `poll_health` after.
    pub async fn up(&self) -> Result<(), ComposeError> {
        self.detect_docker().await?;
        let out = Command::new("docker")
            .args(["compose", "-f", &self.inner.compose_file, "up", "-d"])
            .current_dir(&self.inner.project_dir)
            .output()
            .await?;
        if !out.status.success() {
            return Err(ComposeError::UpFailed {
                code: out.status.code().unwrap_or(-1),
                stderr: String::from_utf8_lossy(&out.stderr).into_owned(),
            });
        }
        *self.inner.started.lock().await = true;
        Ok(())
    }

    /// `docker compose down`. Idempotent — safe to call even if never
    /// started (handy for crash-recovery on next launch).
    pub async fn down(&self) -> Result<(), ComposeError> {
        let out = Command::new("docker")
            .args(["compose", "-f", &self.inner.compose_file, "down"])
            .current_dir(&self.inner.project_dir)
            .output()
            .await?;
        if !out.status.success() {
            return Err(ComposeError::DownFailed {
                code: out.status.code().unwrap_or(-1),
                stderr: String::from_utf8_lossy(&out.stderr).into_owned(),
            });
        }
        *self.inner.started.lock().await = false;
        Ok(())
    }

    /// Poll `api_health_url` until it returns 2xx or `HEALTH_POLL_TIMEOUT`
    /// elapses.
    pub async fn poll_health(&self) -> Result<(), ComposeError> {
        let client = reqwest::Client::builder()
            .timeout(Duration::from_millis(800))
            .build()
            .map_err(|e| ComposeError::Io(std::io::Error::other(e.to_string())))?;
        let deadline = tokio::time::Instant::now() + HEALTH_POLL_TIMEOUT;
        loop {
            if let Ok(resp) = client.get(&self.inner.api_health_url).send().await {
                if resp.status().is_success() {
                    return Ok(());
                }
            }
            if tokio::time::Instant::now() >= deadline {
                return Err(ComposeError::HealthTimeout(HEALTH_POLL_TIMEOUT));
            }
            tokio::time::sleep(HEALTH_POLL_INTERVAL).await;
        }
    }

    /// Spawn a task that streams `docker compose logs --follow` lines to
    /// the supplied callback. Returns the spawned `Child` so the caller
    /// can kill it on shutdown.
    pub fn stream_logs<F>(&self, mut on_line: F) -> Result<tokio::process::Child, ComposeError>
    where
        F: FnMut(String) + Send + 'static,
    {
        let mut child = Command::new("docker")
            .args(["compose", "-f", &self.inner.compose_file, "logs", "--follow", "--tail", "100"])
            .current_dir(&self.inner.project_dir)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()?;
        let stdout = child.stdout.take().ok_or_else(|| {
            ComposeError::Io(std::io::Error::other("logs: failed to capture stdout"))
        })?;
        tokio::spawn(async move {
            let reader = BufReader::new(stdout);
            let mut lines = reader.lines();
            while let Ok(Some(line)) = lines.next_line().await {
                on_line(line);
            }
        });
        Ok(child)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn status_event_serializes_lowercase() {
        let json = serde_json::to_string(&StatusEvent::starting()).unwrap();
        assert!(json.contains("\"status\":\"starting\""), "got {json}");

        let json = serde_json::to_string(&StatusEvent::ready()).unwrap();
        assert!(json.contains("\"status\":\"ready\""), "got {json}");

        let json = serde_json::to_string(&StatusEvent::error("docker missing")).unwrap();
        assert!(json.contains("\"status\":\"error\""), "got {json}");
        assert!(json.contains("\"message\":\"docker missing\""), "got {json}");
    }

    #[test]
    fn manager_exposes_project_dir_and_health_url() {
        let mgr = ComposeManager::new(
            PathBuf::from("/tmp/raven-local"),
            "docker-compose.local.yml",
            "http://127.0.0.1:8081/healthz",
        );
        assert_eq!(mgr.project_dir(), std::path::Path::new("/tmp/raven-local"));
        assert_eq!(mgr.api_health_url(), "http://127.0.0.1:8081/healthz");
    }

    #[tokio::test]
    async fn down_without_up_does_not_panic() {
        // We can't actually invoke docker in unit tests, but the started
        // flag plumbing should at least let down() be called safely. The
        // command itself will fail (exit non-zero or io error) which is fine
        // for this test — we only care that the structure is sound.
        let mgr = ComposeManager::new(
            PathBuf::from("/tmp/this-does-not-exist-raven"),
            "docker-compose.local.yml",
            "http://127.0.0.1:8081/healthz",
        );
        // Don't assert on the result: docker may or may not be installed in
        // CI. We only assert that the call returns without panicking and
        // that the started flag stays false.
        let _ = mgr.down().await;
        let started = *mgr.inner.started.lock().await;
        assert!(!started);
    }
}
