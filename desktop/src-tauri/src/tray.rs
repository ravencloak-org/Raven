//! Tray / menubar status app for Raven Local.
//!
//! Builds a platform-native tray icon (macOS menu bar / Windows system tray /
//! Linux indicator) on app boot. The icon's tooltip and menu update in
//! response to `compose:status` events from the orchestrator (#418), so the
//! user can see at a glance whether the stack is starting, ready, paused,
//! or errored without opening the main window.

use serde::{Deserialize, Serialize};
use tauri::{
    image::Image,
    menu::{Menu, MenuItem},
    tray::{TrayIconBuilder, TrayIconEvent},
    AppHandle, Manager, Wry,
};

const MENU_ID_OPEN: &str = "tray:open";
const MENU_ID_PAUSE: &str = "tray:pause";
const MENU_ID_RESUME: &str = "tray:resume";
const MENU_ID_QUIT: &str = "tray:quit";
pub const TRAY_ID: &str = "raven-local-tray";

/// Status the tray icon reflects. Mirrors `compose::ComposeStatus` but kept
/// separate so the tray module doesn't import compose internals (the value
/// arrives as a serialized `StatusEvent` payload via Tauri's event bus).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum TrayStatus {
    Starting,
    Ready,
    Paused,
    Stopped,
    Error,
}

impl TrayStatus {
    /// Short human-readable label used in the tray tooltip and the disabled
    /// menu item that shows current state.
    pub fn label(self) -> &'static str {
        match self {
            TrayStatus::Starting => "Starting…",
            TrayStatus::Ready => "Ready",
            TrayStatus::Paused => "Paused",
            TrayStatus::Stopped => "Stopped",
            TrayStatus::Error => "Error",
        }
    }
}

/// Build the tray icon and wire its menu. Call once from `setup`.
///
/// The "Pause" / "Resume" item is shown as enabled regardless of state — the
/// click handler is a no-op when it doesn't make sense (e.g. clicking
/// Resume while already Ready). Keeping the menu shape stable avoids
/// rebuilding the menu on every status event.
pub fn install(app: &AppHandle) -> tauri::Result<()> {
    let menu = build_menu(app)?;
    let icon = default_icon();

    TrayIconBuilder::with_id(TRAY_ID)
        .icon(icon)
        .tooltip(format!("Raven Local — {}", TrayStatus::Starting.label()))
        .menu(&menu)
        .show_menu_on_left_click(false) // left-click toggles the window
        .on_menu_event(|app, event| handle_menu_event(app, event.id().as_ref()))
        .on_tray_icon_event(|tray, event| {
            if let TrayIconEvent::Click { button, button_state, .. } = event {
                if matches!(
                    (button, button_state),
                    (tauri::tray::MouseButton::Left, tauri::tray::MouseButtonState::Up)
                ) {
                    toggle_main_window(tray.app_handle());
                }
            }
        })
        .build(app)?;

    Ok(())
}

/// Update the tray's tooltip and menu in response to a status change.
/// Wire this from the compose-status event listener in `main.rs`.
pub fn apply_status(app: &AppHandle, status: TrayStatus) {
    if let Some(tray) = app.tray_by_id(TRAY_ID) {
        let _ = tray.set_tooltip(Some(format!("Raven Local — {}", status.label())));
    }
}

fn build_menu(app: &AppHandle) -> tauri::Result<Menu<Wry>> {
    let open = MenuItem::with_id(app, MENU_ID_OPEN, "Open Raven", true, None::<&str>)?;
    let pause = MenuItem::with_id(app, MENU_ID_PAUSE, "Pause", true, None::<&str>)?;
    let resume = MenuItem::with_id(app, MENU_ID_RESUME, "Resume", true, None::<&str>)?;
    let quit = MenuItem::with_id(app, MENU_ID_QUIT, "Quit", true, None::<&str>)?;
    Menu::with_items(app, &[&open, &pause, &resume, &quit])
}

fn default_icon() -> Image<'static> {
    // Reuse the application icon for now. The tray icon is a 16-22 px
    // glyph on most platforms; the bundle's 32x32.png downscales fine.
    // A status-coloured per-state icon set is a follow-up (M12).
    Image::from_bytes(include_bytes!("../icons/32x32.png"))
        .expect("bundled tray icon must decode")
}

fn toggle_main_window(app: &AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let visible = window.is_visible().unwrap_or(false);
        let _ = if visible { window.hide() } else { window.show() };
        if !visible {
            let _ = window.set_focus();
        }
    }
}

fn handle_menu_event(app: &AppHandle, id: &str) {
    match id {
        MENU_ID_OPEN => {
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.show();
                let _ = window.set_focus();
            }
        }
        MENU_ID_PAUSE => {
            // Emit an event rather than calling compose directly — main.rs
            // owns the ComposeManager and can decide whether pausing means
            // `compose stop` (keep containers, free CPU) or `compose down`.
            let _ = app.emit("tray:pause-requested", ());
        }
        MENU_ID_RESUME => {
            let _ = app.emit("tray:resume-requested", ());
        }
        MENU_ID_QUIT => {
            app.exit(0);
        }
        _ => {}
    }
}

// Bring `Emitter` into scope for `app.emit` in handle_menu_event without
// polluting public API.
use tauri::Emitter;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn status_labels_are_stable() {
        // The Tauri tray API is not testable in unit tests (it needs a
        // real AppHandle and platform window manager). What we can pin
        // here is that the user-visible labels don't change accidentally —
        // breaking them would change the tray tooltip and confuse users.
        assert_eq!(TrayStatus::Starting.label(), "Starting…");
        assert_eq!(TrayStatus::Ready.label(), "Ready");
        assert_eq!(TrayStatus::Paused.label(), "Paused");
        assert_eq!(TrayStatus::Stopped.label(), "Stopped");
        assert_eq!(TrayStatus::Error.label(), "Error");
    }

    #[test]
    fn status_serializes_lowercase_to_match_compose_event() {
        // The compose orchestrator emits `compose:status` with status
        // serialized lowercase. The tray must accept the same payload
        // shape so we can deserialise compose events directly.
        let json = serde_json::to_string(&TrayStatus::Starting).unwrap();
        assert_eq!(json, "\"starting\"");

        let parsed: TrayStatus = serde_json::from_str("\"ready\"").unwrap();
        assert_eq!(parsed, TrayStatus::Ready);
    }
}
