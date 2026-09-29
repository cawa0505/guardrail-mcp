use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use serde::{Deserialize, Serialize};
use thiserror::Error;

use crate::fsm::Phase;
use crate::token::CommitToken;

#[derive(Debug, Error)]
pub enum StorageError {
    #[error("io error: {0}")]
    Io(#[from] std::io::Error),
    #[error("json serialization/deserialization error: {0}")]
    Json(#[from] serde_json::Error),
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct CompResult {
    pub success: bool,
    pub raw_output: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct StagingBuf {
    #[serde(default)]
    pub dir: String,
    #[serde(default)]
    pub has_pending_patch: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub target_file: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub patch_content: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub last_compiler_result: Option<CompResult>,
}

impl Default for StagingBuf {
    fn default() -> Self {
        Self {
            dir: String::new(),
            has_pending_patch: false,
            target_file: None,
            patch_content: None,
            last_compiler_result: None,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Checkpoint {
    pub id: String,
    pub timestamp: String,
    pub summary: String,
    #[serde(default)]
    pub modified_files: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct StateData {
    pub version: String,
    pub phase: Phase,
    #[serde(default)]
    pub active_goal: String,
    #[serde(default)]
    pub allowed_actions: Vec<String>,
    #[serde(default)]
    pub staging_buffer: StagingBuf,
    #[serde(default)]
    pub checkpoints: Vec<Checkpoint>,
    #[serde(default)]
    pub failed_attempts: i32,
    #[serde(default)]
    pub ast_synced: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub commit_token: Option<CommitToken>,
}

impl Default for StateData {
    fn default() -> Self {
        Self {
            version: "1.0.0".to_string(),
            phase: Phase::Init,
            active_goal: String::new(),
            allowed_actions: Vec::new(),
            staging_buffer: StagingBuf::default(),
            checkpoints: Vec::new(),
            failed_attempts: 0,
            ast_synced: false,
            commit_token: None,
        }
    }
}

pub struct StateStore {
    root_dir: PathBuf,
}

impl StateStore {
    pub fn new(root_dir: impl AsRef<Path>) -> Self {
        Self {
            root_dir: root_dir.as_ref().to_path_buf(),
        }
    }

    /// Resolve state file path with canonical `.guardrail/state.json` and fallback `.opencode/state.json`
    pub fn state_file_path(&self) -> PathBuf {
        let guardrail_path = self.root_dir.join(".guardrail").join("state.json");
        let opencode_path = self.root_dir.join(".opencode").join("state.json");

        if !guardrail_path.exists() && opencode_path.exists() {
            opencode_path
        } else {
            guardrail_path
        }
    }

    pub fn load(&self) -> Result<StateData, StorageError> {
        let path = self.state_file_path();
        if !path.exists() {
            return Ok(StateData::default());
        }
        let content = std::fs::read_to_string(&path)?;
        let state: StateData = serde_json::from_str(&content)?;
        Ok(state)
    }

    pub fn save(&self, state: &StateData) -> Result<(), StorageError> {
        let path = self.state_file_path();
        if let Some(parent) = path.parent() {
            std::fs::create_dir_all(parent)?;
        }
        let json = serde_json::to_string_pretty(state)?;

        // ponytail: atomic rename; per-process unique tmp avoids concurrent
        // writers clobbering each other's temp file.
        static TMP_SEQ: AtomicU64 = AtomicU64::new(0);
        let seq = TMP_SEQ.fetch_add(1, Ordering::Relaxed);
        let tmp = path.with_extension(format!("json.tmp.{}.{}", std::process::id(), seq));
        std::fs::write(&tmp, json)?;
        std::fs::rename(&tmp, &path)?;
        Ok(())
    }
}
