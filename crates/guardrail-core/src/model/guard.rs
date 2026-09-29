use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use thiserror::Error;

use crate::fsm::StateMachine;
use crate::model::{Action, Decision};

#[derive(Debug, Error)]
pub enum GuardError {
    #[error("hard guard violation: {0}")]
    HardViolation(String),
    #[error("soft guard verifier error: {0}")]
    VerifierError(String),
}

/// Hard Guard evaluates synchronous, deterministic constraints (Phase gates, blacklists, schemas)
pub struct HardGuard;

impl HardGuard {
    pub fn evaluate(fsm: &StateMachine, action: &Action, timestamp: i64) -> Decision {
        // 1. Phase whitelist check
        if !fsm.is_action_allowed(&action.action_type) {
            return Decision::deny(
                format!(
                    "Action '{}' is denied in current phase {:?}",
                    action.action_type,
                    fsm.current_phase()
                ),
                timestamp,
            );
        }

        // 2. Sensitive action approval gate
        if action.action_type == "coding.patch" || action.action_type == "apply_patch" {
            // Check if destructive
            if let Some(target) = action.payload.get("path").and_then(|v| v.as_str()) {
                if target.starts_with("/etc") || target.contains(".env") {
                    return Decision::deny(
                        format!("Target '{}' is protected by security policy", target),
                        timestamp,
                    );
                }
            }
        }

        // 3. Sensitive-but-overridable targets require explicit human approval.
        if action.action_type == "coding.patch" || action.action_type == "apply_patch" {
            let target = action.target.to_lowercase();
            const APPROVAL_PATTERNS: &[&str] =
                &[".pem", ".key", "id_rsa", "credentials", "secrets."];
            if APPROVAL_PATTERNS.iter().any(|p| target.contains(p)) {
                return Decision::require_approval(
                    format!("Target '{}' requires human approval before modification", action.target),
                    timestamp,
                );
            }
        }

        Decision::allow("Passed hard guard phase & deterministic checks", timestamp)
    }
}

/// Result from a Soft Guard Verifier
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct VerificationResult {
    pub passed: bool,
    pub verifier_id: String,
    pub message: String,
}

#[async_trait]
pub trait SoftVerifier: Send + Sync {
    fn id(&self) -> &str;
    async fn verify(&self, action: &Action) -> Result<VerificationResult, GuardError>;
}

/// Soft Guard orchestrates pluggable external verifiers (AST, Compiler, LLM, etc.)
pub struct SoftGuard {
    verifiers: Vec<Box<dyn SoftVerifier>>,
}

impl SoftGuard {
    pub fn new() -> Self {
        Self {
            verifiers: Vec::new(),
        }
    }

    pub fn register(&mut self, verifier: Box<dyn SoftVerifier>) {
        self.verifiers.push(verifier);
    }

    pub async fn evaluate_all(&self, action: &Action, timestamp: i64) -> Result<Decision, GuardError> {
        for v in &self.verifiers {
            let res = v.verify(action).await?;
            if !res.passed {
                return Ok(Decision::require_verification(
                    v.id(),
                    format!("Verifier '{}' reported: {}", v.id(), res.message),
                    timestamp,
                ));
            }
        }

        Ok(Decision::allow("All soft verifiers passed", timestamp))
    }
}
