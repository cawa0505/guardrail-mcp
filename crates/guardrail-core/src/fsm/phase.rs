use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum Phase {
    Init,
    Planning,
    Executing,
    Verifying,
    Paused,
    Completed,
    Failed,
    Cancelled,
}

impl Phase {
    pub fn as_str(&self) -> &'static str {
        match self {
            Phase::Init => "INIT",
            Phase::Planning => "PLANNING",
            Phase::Executing => "EXECUTING",
            Phase::Verifying => "VERIFYING",
            Phase::Paused => "PAUSED",
            Phase::Completed => "COMPLETED",
            Phase::Failed => "FAILED",
            Phase::Cancelled => "CANCELLED",
        }
    }

    pub fn parse(s: &str) -> Option<Self> {
        match s.to_uppercase().as_str() {
            "INIT" => Some(Phase::Init),
            "PLANNING" => Some(Phase::Planning),
            "EXECUTING" => Some(Phase::Executing),
            "VERIFYING" => Some(Phase::Verifying),
            "PAUSED" => Some(Phase::Paused),
            "COMPLETED" => Some(Phase::Completed),
            "FAILED" => Some(Phase::Failed),
            "CANCELLED" => Some(Phase::Cancelled),
            _ => None,
        }
    }

    pub fn is_terminal(&self) -> bool {
        matches!(self, Phase::Completed | Phase::Cancelled)
    }

    pub fn can_transition_to(&self, next: Phase) -> bool {
        match (self, next) {
            // Standard progression
            (Phase::Init, Phase::Planning) => true,
            (Phase::Planning, Phase::Executing) => true,
            (Phase::Executing, Phase::Verifying) => true,
            (Phase::Verifying, Phase::Completed) => true,

            // Self transition / checkpointing in same phase
            (p1, p2) if *p1 == p2 => true,

            // Failure can occur from active phases
            (Phase::Planning | Phase::Executing | Phase::Verifying, Phase::Failed) => true,

            // Retry from Failed back to Planning
            (Phase::Failed, Phase::Planning) => true,

            // Cancellation from any non-terminal phase
            (Phase::Init | Phase::Planning | Phase::Executing | Phase::Verifying | Phase::Paused | Phase::Failed, Phase::Cancelled) => true,

            // Backward compatibility with Go PAUSED state
            (Phase::Executing, Phase::Paused) => true,
            (Phase::Paused, Phase::Executing) => true,
            (Phase::Paused, Phase::Planning) => true,

            _ => false,
        }
    }

    /// Canonical action whitelist for this phase. Mirrors the Go `PhaseActions`
    /// table so `state.allowed_actions` can be persisted for legacy parity.
    pub fn default_actions(&self) -> Vec<&'static str> {
        match self {
            Phase::Init => vec!["checkpoint", "get_status"],
            Phase::Planning => vec!["inspect_context", "checkpoint", "get_status"],
            Phase::Executing => vec!["inspect_context", "apply_patch", "checkpoint", "get_status"],
            Phase::Verifying => vec!["inspect_context", "get_status"],
            Phase::Paused => vec!["checkpoint", "get_status"],
            Phase::Completed => vec!["get_status"],
            Phase::Failed => vec!["get_status", "checkpoint"],
            Phase::Cancelled => vec!["get_status"],
        }
    }

    /// Legacy / coding-specific action whitelist check (for 100% Go backward compatibility)
    pub fn is_action_allowed_legacy(&self, action: &str) -> bool {
        match self {
            Phase::Init => matches!(action, "get_status" | "checkpoint"),
            Phase::Planning => matches!(action, "inspect_context" | "checkpoint" | "get_status"),
            Phase::Executing => matches!(action, "inspect_context" | "apply_patch" | "checkpoint" | "get_status"),
            Phase::Verifying => matches!(action, "inspect_context" | "get_status"),
            Phase::Paused => matches!(action, "checkpoint" | "get_status"),
            Phase::Completed => matches!(action, "get_status"),
            Phase::Failed => matches!(action, "get_status" | "checkpoint"),
            Phase::Cancelled => matches!(action, "get_status"),
        }
    }
}
