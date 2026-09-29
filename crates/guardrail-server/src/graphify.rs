//! Graphify MCP stdio client — spawns the external Graphify server and calls
//! `graphify_skeleton_extract` for deep AST skeletons.
//!
//! Mirrors the deleted Go `internal/graphify` package: the core does no
//! tree-sitter parsing itself; AST extraction is delegated over MCP.

use std::time::Duration;

use rmcp::model::CallToolRequestParams;
use rmcp::transport::TokioChildProcess;
use rmcp::ServiceExt;
use serde::{Deserialize, Serialize};
use tokio::process::Command;

/// Config mirrors the Go `graphify.json` schema (`binary_path`, `timeout_ms`,
/// `required`). Stored at `<state_dir>/graphify.json`.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GraphifyConfig {
    #[serde(default)]
    pub binary_path: String,
    #[serde(default = "default_timeout_ms")]
    pub timeout_ms: u64,
    #[serde(default)]
    pub required: bool,
}

fn default_timeout_ms() -> u64 {
    30_000
}

impl Default for GraphifyConfig {
    fn default() -> Self {
        Self {
            binary_path: String::new(),
            timeout_ms: default_timeout_ms(),
            required: false,
        }
    }
}

impl GraphifyConfig {
    /// Load from `<dir>/graphify.json`; missing file returns defaults, matching
    /// the Go loader.
    pub fn load(dir: &std::path::Path) -> Result<Self, String> {
        let path = dir.join("graphify.json");
        match std::fs::read_to_string(&path) {
            Ok(data) => serde_json::from_str(&data).map_err(|e| format!("parse graphify config: {e}")),
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(Self::default()),
            Err(e) => Err(format!("read graphify config: {e}")),
        }
    }
}

pub struct GraphifyClient {
    binary_path: String,
    timeout: Duration,
}

impl GraphifyClient {
    pub fn new(cfg: &GraphifyConfig) -> Self {
        Self {
            binary_path: cfg.binary_path.clone(),
            timeout: Duration::from_millis(cfg.timeout_ms.max(1)),
        }
    }

    /// Returns true when a binary path is configured.
    pub fn is_configured(&self) -> bool {
        !self.binary_path.is_empty()
    }

    /// Spawn Graphify over stdio, call `graphify_skeleton_extract`, and return
    /// the concatenated text content. Any failure returns an error string; the
    /// caller decides whether it is fatal (`required`) or ignorable.
    pub async fn skeleton_extract(&self, file_path: &str) -> Result<String, String> {
        if self.binary_path.is_empty() {
            return Err("graphify binary not configured".to_string());
        }

        let result = tokio::time::timeout(self.timeout, self.call_extract(file_path)).await;
        match result {
            Ok(inner) => inner,
            Err(_) => Err("graphify skeleton_extract timed out".to_string()),
        }
    }

    async fn call_extract(&self, file_path: &str) -> Result<String, String> {
        let cmd = Command::new(&self.binary_path);
        let transport =
            TokioChildProcess::new(cmd).map_err(|e| format!("graphify spawn: {e}"))?;

        let client = ()
            .serve(transport)
            .await
            .map_err(|e| format!("graphify connect: {e}"))?;

        let mut args = serde_json::Map::new();
        args.insert("path".to_string(), serde_json::json!(file_path));

        let params = CallToolRequestParams::new("graphify_skeleton_extract").with_arguments(args);
        let res = client
            .call_tool(params)
            .await
            .map_err(|e| format!("graphify skeleton_extract: {e}"))?;

        if res.is_error == Some(true) {
            return Err("graphify skeleton_extract returned error".to_string());
        }

        let mut text = String::new();
        for block in &res.content {
            if let rmcp::model::ContentBlock::Text(t) = block {
                if !text.is_empty() {
                    text.push('\n');
                }
                text.push_str(&t.text);
            }
        }

        // Best-effort graceful shutdown; ignore errors.
        let _ = client.cancel().await;

        Ok(text)
    }
}
