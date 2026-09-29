use serde::{Deserialize, Serialize};
use serde_json::Value;

use super::{Action, Actor, Decision};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct EvidenceRecord {
    pub event_id: String,
    pub actor: Actor,
    pub action: Action,
    pub decision: Decision,
    pub timestamp: i64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parent_event: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub context: Option<Value>,
}

impl EvidenceRecord {
    pub fn new(
        event_id: impl Into<String>,
        actor: Actor,
        action: Action,
        decision: Decision,
        timestamp: i64,
    ) -> Self {
        Self {
            event_id: event_id.into(),
            actor,
            action,
            decision,
            timestamp,
            parent_event: None,
            context: None,
        }
    }

    pub fn with_parent(mut self, parent_hash: impl Into<String>) -> Self {
        self.parent_event = Some(parent_hash.into());
        self
    }

    pub fn with_context(mut self, context: Value) -> Self {
        self.context = Some(context);
        self
    }
}
