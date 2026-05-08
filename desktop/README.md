# Raven Local — Tauri shell

A desktop wrapper that runs the Raven stack locally via Docker compose with Ollama bundled as a local LLM provider. Tracks milestone [M11](https://github.com/ravencloak-org/Raven/milestone/14).

## Prerequisites

- Rust toolchain (stable)
- Docker Desktop (or compatible)
- macOS / Windows / Linux
- Tauri CLI v2: `cargo install tauri-cli --version '^2.0' --locked`

## Run in dev mode

```bash
cd desktop
cargo tauri dev
```

This opens a window that points at `http://127.0.0.1:8081/` — the URL of the local Raven API. Run `docker compose up -d` from the repo root first; full lifecycle integration lands in [#418](https://github.com/ravencloak-org/Raven/issues/418).

## Build a release bundle

```bash
cd desktop
cargo tauri build
```

Outputs `desktop/src-tauri/target/release/bundle/`.

For a debug bundle (faster, no optimisations):

```bash
cargo tauri build --debug
```

## Project layout

```
desktop/
├── Cargo.toml              # Workspace root (single member: src-tauri)
├── .gitignore
├── README.md
└── src-tauri/
    ├── Cargo.toml          # raven-local crate
    ├── build.rs            # tauri_build::build()
    ├── tauri.conf.json     # Tauri 2 configuration
    ├── splash.html         # Splash screen shown before compose is ready
    ├── icons/              # App icons (placeholder — replace for release)
    └── src/
        └── main.rs         # Entry point
```

## What's next

- [#418](https://github.com/ravencloak-org/Raven/issues/418): compose lifecycle from inside the app (replaces the 3-second splash timer with a real health-check event)
- [#419](https://github.com/ravencloak-org/Raven/issues/419): single-user mode flag
- [#420](https://github.com/ravencloak-org/Raven/issues/420): system-requirements precheck
- [#427](https://github.com/ravencloak-org/Raven/issues/427): Linux and Windows CI build jobs
