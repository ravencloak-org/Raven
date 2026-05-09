//! Tauri commands and helpers for managing local Ollama models.
//!
//! Ollama's `/api/pull` endpoint streams JSONL — one event per line. We
//! parse each line, emit progress events to the frontend (so the wizard's
//! progress bar can update), and stop when we see the terminal
//! `{"status":"success"}` (or an `{"error": ...}`).

use std::time::Duration;

use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Emitter};
use tokio::io::{AsyncBufReadExt, BufReader};

const OLLAMA_HOST: &str = "http://127.0.0.1:11434";
const PULL_PROGRESS_EVENT: &str = "ollama:pull-progress";

#[derive(Debug, Clone, Serialize)]
pub struct PullProgress {
    pub model: String,
    pub completed: u64,
    pub total: u64,
    pub percent: u8,
}

#[derive(Debug, Clone, Serialize)]
pub struct ModelInfo {
    pub name: String,
    pub size: u64,
    pub digest: String,
}

#[derive(Deserialize)]
struct OllamaPullEvent {
    #[serde(default)]
    status: Option<String>,
    #[serde(default)]
    completed: Option<u64>,
    #[serde(default)]
    total: Option<u64>,
    #[serde(default)]
    error: Option<String>,
}

/// Returns Some when the line contains a meaningful per-byte progress
/// snapshot (both `total` and `completed` are present and `total` is
/// non-zero). Returns None for status-only lines like
/// `{"status":"pulling manifest"}`.
pub(crate) fn parse_progress_line(line: &str) -> Option<PullProgress> {
    let ev: OllamaPullEvent = serde_json::from_str(line).ok()?;
    let total = ev.total?;
    let completed = ev.completed?;
    if total == 0 {
        return None;
    }
    let percent = ((completed as f64 / total as f64) * 100.0).round() as u8;
    Some(PullProgress {
        model: String::new(), // filled in by the caller — it knows which model
        completed,
        total,
        percent: percent.min(100),
    })
}

/// True when the line is the terminal `{"status":"success"}` event from
/// /api/pull, signalling the pull completed.
pub(crate) fn is_success_line(line: &str) -> bool {
    matches!(
        serde_json::from_str::<OllamaPullEvent>(line).ok().and_then(|e| e.status),
        Some(s) if s == "success"
    )
}

/// Returns Some(error_message) when the line is `{"error": "..."}`.
pub(crate) fn parse_error_line(line: &str) -> Option<String> {
    serde_json::from_str::<OllamaPullEvent>(line).ok().and_then(|e| e.error)
}

#[tauri::command]
pub async fn ollama_pull_model(app: AppHandle, model: String) -> Result<(), String> {
    let url = format!("{OLLAMA_HOST}/api/pull");
    let body = serde_json::json!({ "name": model, "stream": true });

    let client = reqwest::Client::builder()
        .timeout(Duration::from_secs(30 * 60)) // 30-min cap for big models
        .build()
        .map_err(|e| e.to_string())?;

    let resp = client
        .post(&url)
        .json(&body)
        .send()
        .await
        .map_err(|e| format!("connect to ollama: {e}"))?;

    if !resp.status().is_success() {
        return Err(format!("ollama pull rejected: status={}", resp.status()));
    }

    let stream = resp.bytes_stream();
    use futures_util::StreamExt;
    let reader = tokio_util::io::StreamReader::new(
        stream.map(|r| r.map_err(std::io::Error::other)),
    );
    let mut lines = BufReader::new(reader).lines();

    while let Some(line) = lines.next_line().await.map_err(|e| e.to_string())? {
        if line.trim().is_empty() {
            continue;
        }
        if let Some(err) = parse_error_line(&line) {
            return Err(err);
        }
        if let Some(mut progress) = parse_progress_line(&line) {
            progress.model = model.clone();
            let _ = app.emit(PULL_PROGRESS_EVENT, &progress);
        }
        if is_success_line(&line) {
            break;
        }
    }
    Ok(())
}

#[tauri::command]
pub async fn ollama_list_models() -> Result<Vec<ModelInfo>, String> {
    let url = format!("{OLLAMA_HOST}/api/tags");
    let resp = reqwest::get(&url).await.map_err(|e| e.to_string())?;
    if !resp.status().is_success() {
        return Err(format!("ollama list rejected: status={}", resp.status()));
    }

    #[derive(Deserialize)]
    struct ListResp {
        models: Vec<RawModel>,
    }
    #[derive(Deserialize)]
    struct RawModel {
        name: String,
        size: u64,
        digest: String,
    }

    let body: ListResp = resp.json().await.map_err(|e| e.to_string())?;
    Ok(body
        .models
        .into_iter()
        .map(|m| ModelInfo { name: m.name, size: m.size, digest: m.digest })
        .collect())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn pull_progress_parser_emits_percent_for_progress_lines() {
        let raw = include_str!("../tests/fixtures/ollama-pull-stream.jsonl");
        let mut percents = Vec::new();
        for line in raw.lines().filter(|l| !l.is_empty()) {
            if let Some(p) = parse_progress_line(line) {
                percents.push(p.percent);
            }
        }
        assert_eq!(percents, vec![25, 75, 100]);
    }

    #[test]
    fn pull_progress_parser_skips_status_only_lines() {
        let line = r#"{"status":"pulling manifest"}"#;
        assert!(parse_progress_line(line).is_none());
    }

    #[test]
    fn pull_progress_parser_marks_success() {
        let line = r#"{"status":"success"}"#;
        assert!(parse_progress_line(line).is_none());
        assert!(is_success_line(line));
    }

    #[test]
    fn pull_progress_parser_handles_error_lines() {
        let line = r#"{"error":"unknown model"}"#;
        let err = parse_error_line(line);
        assert_eq!(err.as_deref(), Some("unknown model"));
    }

    #[test]
    fn pull_progress_parser_caps_percent_at_100() {
        // Defensive: if completed somehow exceeds total, clamp to 100.
        let line = r#"{"status":"pulling x","total":100,"completed":150}"#;
        let p = parse_progress_line(line).unwrap();
        assert_eq!(p.percent, 100);
    }
}
