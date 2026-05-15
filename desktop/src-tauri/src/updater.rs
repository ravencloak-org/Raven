//! Auto-update logic for Raven Local.
//!
//! On app launch this module checks whether 24 hours have elapsed since the
//! last update check. If so, it queries the update manifest at
//! `https://download.raven.app/local/latest.json`. When a new version is
//! found, an `updater:available` event is emitted to the frontend so the UI
//! can show a non-blocking "Update available — Restart to apply" banner.
//!
//! Design goals:
//! - Never block app launch: all network I/O is async and errors are silently
//!   logged; the app continues normally whether or not the endpoint is
//!   reachable.
//! - Debounce to once per 24 h: the timestamp of the last check is persisted
//!   in the Tauri app-data directory (`last_update_check` plain-text file).
//! - Download → verify signature → stage on quit: the actual install is
//!   deferred until the user clicks "Restart to apply" in the UI.

use std::time::{Duration, SystemTime, UNIX_EPOCH};

use tauri::{AppHandle, Emitter, Manager};
use tauri_plugin_updater::UpdaterExt;

/// Debounce interval: only check for updates once every 24 hours.
const CHECK_INTERVAL_SECS: u64 = 24 * 60 * 60;

/// File name inside the app-data directory that stores the last-check epoch.
const TIMESTAMP_FILE: &str = "last_update_check";

/// Frontend event emitted when a new version is available.
pub const UPDATE_AVAILABLE_EVENT: &str = "updater:available";

/// Frontend event emitted when download + install is complete (quit to apply).
pub const UPDATE_READY_EVENT: &str = "updater:ready";

/// Frontend event emitted when the download fails.
pub const UPDATE_ERROR_EVENT: &str = "updater:error";

/// Payload sent with [`UPDATE_AVAILABLE_EVENT`].
#[derive(Debug, Clone, serde::Serialize)]
pub struct UpdateAvailablePayload {
    /// The new version string (e.g. `"0.2.0"`).
    pub version: String,
    /// Optional release notes / changelog snippet.
    pub notes: Option<String>,
}

/// Payload sent with [`UPDATE_ERROR_EVENT`].
#[derive(Debug, Clone, serde::Serialize)]
pub struct UpdateErrorPayload {
    pub message: String,
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/// Spawn the update-check task in the background.
///
/// Call once from `main.rs` setup, after registering the updater plugin.
/// All errors are swallowed — the update flow must never prevent the app
/// from starting.
pub fn spawn_update_check(app: AppHandle) {
    tauri::async_runtime::spawn(async move {
        if let Err(e) = check_once(&app).await {
            // Emit error to frontend (non-fatal); continue normal launch.
            let _ = app.emit(
                UPDATE_ERROR_EVENT,
                UpdateErrorPayload {
                    message: e.to_string(),
                },
            );
        }
    });
}

/// Called from the frontend `invoke("updater_install_and_restart")` handler
/// when the user clicks "Restart to apply". Downloads, verifies, and
/// installs the pending update, then restarts the app.
#[tauri::command]
pub async fn updater_install_and_restart(app: AppHandle) -> Result<(), String> {
    do_install_and_restart(app).await.map_err(|e| e.to_string())
}

// ---------------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------------

async fn check_once(app: &AppHandle) -> anyhow::Result<()> {
    // Skip the network call if we already checked within the last 24 h.
    if !should_check(app) {
        return Ok(());
    }

    // Mark timestamp *before* the network call so a hanging endpoint doesn't
    // cause repeated retries within the same session.
    write_timestamp(app);

    // Delegate to tauri-plugin-updater; this performs the HTTP request,
    // parses the JSON manifest, and verifies the server-side signature.
    let updater = app.updater().map_err(|e| anyhow::anyhow!("{e}"))?;

    match updater.check().await {
        Ok(Some(update)) => {
            let payload = UpdateAvailablePayload {
                version: update.version.clone(),
                notes: update.body.clone(),
            };
            let _ = app.emit(UPDATE_AVAILABLE_EVENT, payload);

            // Store the pending update handle in app state so the install
            // command can retrieve it without fetching a second time.
            app.manage(PendingUpdate(std::sync::Mutex::new(Some(update))));
        }
        Ok(None) => {
            // Already up-to-date — nothing to do.
        }
        Err(e) => {
            // Network/parse error: emit to frontend but don't propagate.
            let _ = app.emit(
                UPDATE_ERROR_EVENT,
                UpdateErrorPayload {
                    message: format!("Update check failed: {e}"),
                },
            );
        }
    }

    Ok(())
}

/// App-managed state holding the pending [`tauri_plugin_updater::Update`].
struct PendingUpdate(std::sync::Mutex<Option<tauri_plugin_updater::Update>>);

async fn do_install_and_restart(app: AppHandle) -> anyhow::Result<()> {
    // Retrieve the pending update that was stashed during `check_once`.
    let update = {
        let state = app
            .try_state::<PendingUpdate>()
            .ok_or_else(|| anyhow::anyhow!("No pending update available"))?;
        let mut guard = state
            .0
            .lock()
            .map_err(|e| anyhow::anyhow!("mutex poisoned: {e}"))?;
        guard.take()
    };

    let Some(update) = update else {
        return Err(anyhow::anyhow!("Pending update was already consumed"));
    };

    // Download and verify the package (tauri-plugin-updater handles sig check).
    update
        .download_and_install(
            |_chunk_len, _total| {
                // Progress reporting — could emit events here in a future PR.
            },
            || {
                // Called when download completes, before install begins.
            },
        )
        .await
        .map_err(|e| anyhow::anyhow!("Install failed: {e}"))?;

    let _ = app.emit(UPDATE_READY_EVENT, ());

    // Restart the app so the freshly installed binary takes effect.
    app.restart();
}

// ---------------------------------------------------------------------------
// Timestamp helpers
// ---------------------------------------------------------------------------

fn timestamp_path(app: &AppHandle) -> Option<std::path::PathBuf> {
    app.path()
        .app_data_dir()
        .ok()
        .map(|d| d.join(TIMESTAMP_FILE))
}

fn should_check(app: &AppHandle) -> bool {
    let Some(path) = timestamp_path(app) else {
        return true;
    };
    let Ok(contents) = std::fs::read_to_string(&path) else {
        return true;
    };
    let Ok(epoch_secs) = contents.trim().parse::<u64>() else {
        return true;
    };
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or(Duration::ZERO)
        .as_secs();
    now.saturating_sub(epoch_secs) >= CHECK_INTERVAL_SECS
}

fn write_timestamp(app: &AppHandle) {
    let Some(path) = timestamp_path(app) else {
        return;
    };
    if let Some(parent) = path.parent() {
        let _ = std::fs::create_dir_all(parent);
    }
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or(Duration::ZERO)
        .as_secs();
    let _ = std::fs::write(&path, now.to_string());
}
