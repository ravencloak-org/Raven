// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::path::{Path, PathBuf};

use raven_ai_lib::compose::{ComposeManager, StatusEvent};
use raven_ai_lib::precheck::{run_precheck, skip_requested, RealSystemInfo};
use raven_ai_lib::tray::{self, TrayStatus};
use raven_ai_lib::updater;
use tauri::{Emitter, Listener, Manager};

const COMPOSE_FILE: &str = "docker-compose.local.yml";
const API_HEALTH_URL: &str = "http://127.0.0.1:8081/healthz";
const STATUS_EVENT: &str = "compose:status";
const LOG_EVENT: &str = "compose:log";
const PRECHECK_EVENT: &str = "precheck:result";

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(tauri_plugin_process::init())
        .setup(|app| {
            let resource_dir = app
                .path()
                .resource_dir()
                .expect("Tauri resource directory should be available");
            let project_dir = resolve_project_dir(&resource_dir);

            let manager = ComposeManager::new(project_dir, COMPOSE_FILE, API_HEALTH_URL);
            let manager_for_setup = manager.clone();
            let manager_for_shutdown = manager.clone();
            app.manage(manager);

            // Install the tray icon up front so users see status as soon as
            // the app launches (before compose has finished coming up).
            tray::install(app.handle())?;

            // Update the tray on compose:status events so the tooltip reflects
            // the live state. We listen on the same channel the frontend uses.
            let tray_handle = app.handle().clone();
            app.listen("compose:status", move |event| {
                if let Ok(payload) = serde_json::from_str::<StatusEvent>(event.payload()) {
                    let status = match payload.status {
                        raven_ai_lib::compose::ComposeStatus::Starting => TrayStatus::Starting,
                        raven_ai_lib::compose::ComposeStatus::Ready => TrayStatus::Ready,
                        raven_ai_lib::compose::ComposeStatus::Stopped => TrayStatus::Stopped,
                        raven_ai_lib::compose::ComposeStatus::Error => TrayStatus::Error,
                    };
                    tray::apply_status(&tray_handle, status);
                }
            });

            // Wire the tray menu's Pause/Resume into the compose orchestrator.
            let pause_manager = manager_for_setup.clone();
            let pause_handle = app.handle().clone();
            app.listen("tray:pause-requested", move |_| {
                let mgr = pause_manager.clone();
                let handle = pause_handle.clone();
                tauri::async_runtime::spawn(async move {
                    if let Err(e) = mgr.down().await {
                        let _ = handle.emit(STATUS_EVENT, StatusEvent::error(e.to_string()));
                    } else {
                        tray::apply_status(&handle, TrayStatus::Paused);
                        let _ = handle.emit(STATUS_EVENT, StatusEvent::stopped());
                    }
                });
            });
            let resume_manager = manager_for_setup.clone();
            let resume_handle = app.handle().clone();
            app.listen("tray:resume-requested", move |_| {
                let mgr = resume_manager.clone();
                let handle = resume_handle.clone();
                tauri::async_runtime::spawn(async move {
                    let _ = handle.emit(STATUS_EVENT, StatusEvent::starting());
                    if let Err(e) = mgr.up().await {
                        let _ = handle.emit(STATUS_EVENT, StatusEvent::error(e.to_string()));
                        return;
                    }
                    if let Err(e) = mgr.poll_health().await {
                        let _ = handle.emit(STATUS_EVENT, StatusEvent::error(e.to_string()));
                    } else {
                        let _ = handle.emit(STATUS_EVENT, StatusEvent::ready());
                    }
                });
            });

            let handle = app.handle().clone();
            tauri::async_runtime::spawn(async move {
                // System-requirements precheck. The frontend always receives
                // the structured result so the splash can render warnings;
                // RAVEN_LOCAL_SKIP_REQS=true lets power users proceed even
                // when the host is below thresholds.
                let precheck = run_precheck(&RealSystemInfo);
                let _ = handle.emit(PRECHECK_EVENT, &precheck);
                if !precheck.ok && !skip_requested() {
                    let summary = precheck
                        .warnings
                        .iter()
                        .map(|w| w.remediation())
                        .collect::<Vec<_>>()
                        .join(" / ");
                    let _ = handle.emit(
                        STATUS_EVENT,
                        StatusEvent::error(format!(
                            "system-requirements precheck failed: {summary} \
                             (set RAVEN_LOCAL_SKIP_REQS=true to bypass)"
                        )),
                    );
                    return;
                }

                if let Err(e) = manager_for_setup.detect_docker().await {
                    let _ = handle.emit(STATUS_EVENT, StatusEvent::error(e.to_string()));
                    return;
                }

                let _ = handle.emit(STATUS_EVENT, StatusEvent::starting());

                if let Err(e) = manager_for_setup.up().await {
                    let _ = handle.emit(STATUS_EVENT, StatusEvent::error(e.to_string()));
                    return;
                }

                let log_handle = handle.clone();
                if let Err(e) = manager_for_setup.stream_logs(move |line| {
                    let _ = log_handle.emit(LOG_EVENT, line);
                }) {
                    let _ = handle.emit(STATUS_EVENT, StatusEvent::error(e.to_string()));
                }

                match manager_for_setup.poll_health().await {
                    Ok(()) => {
                        let _ = handle.emit(STATUS_EVENT, StatusEvent::ready());
                    }
                    Err(e) => {
                        let _ = handle.emit(STATUS_EVENT, StatusEvent::error(e.to_string()));
                    }
                }
            });

            // Bring compose down cleanly on app quit. Tauri 2 doesn't expose
            // a stable on-exit hook from setup; instead we listen for the
            // close-requested webview event AND register a global shutdown
            // via run_event below in main.
            let _ = manager_for_shutdown;

            // Kick off a background update check (debounced to once per 24 h).
            // Errors are emitted as `updater:error` events and never block launch.
            updater::spawn_update_check(app.handle().clone());

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            raven_ai_lib::ollama::ollama_pull_model,
            raven_ai_lib::ollama::ollama_list_models,
            raven_ai_lib::updater::updater_install_and_restart
        ])
        .build(tauri::generate_context!())
        .expect("error while building Raven AI")
        .run(|app, event| {
            if let tauri::RunEvent::Exit | tauri::RunEvent::ExitRequested { .. } = event {
                if let Some(manager) = app.try_state::<ComposeManager>() {
                    let manager = manager.inner().clone();
                    tauri::async_runtime::block_on(async move {
                        let _ = manager.down().await;
                    });
                }
            }
        });
}

/// In packaged builds, the compose file lives next to the resource bundle.
/// In dev mode (`cargo tauri dev`), it's at `<repo>/desktop/`. We pick the
/// first parent containing the compose file, falling back to the resource
/// dir itself.
fn resolve_project_dir(resource_dir: &Path) -> PathBuf {
    let candidate = resource_dir.join(COMPOSE_FILE);
    if candidate.exists() {
        return resource_dir.to_path_buf();
    }
    let dev_candidate = std::env::current_dir()
        .ok()
        .map(|cwd| cwd.join("desktop").join(COMPOSE_FILE));
    if let Some(p) = dev_candidate {
        if p.exists() {
            return p.parent().unwrap().to_path_buf();
        }
    }
    resource_dir.to_path_buf()
}
