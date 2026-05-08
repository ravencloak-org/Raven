//! Library entry point for `raven-local`.
//!
//! Tauri 2 templates split between `main.rs` (binary) and `lib.rs` so
//! mobile targets (iOS/Android) can link the same code. For now we keep
//! the orchestrator implementation here so it's available to both.

pub mod compose;
