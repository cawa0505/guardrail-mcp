use serde::{Deserialize, Serialize};
use serde_json::Value;

use super::Actor;

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Action {
    pub action_id: String,
    pub actor: Actor,
    pub action_type: String,
    pub target: String,
    #[serde(default)]
    pub payload: Value,
    #[serde(default)]
    pub context: Value,
    pub timestamp: i64,
}

impl Action {
    pub fn new(
        action_id: impl Into<String>,
        actor: Actor,
        action_type: impl Into<String>,
        target: impl Into<String>,
        timestamp: i64,
    ) -> Self {
        Self {
            action_id: action_id.into(),
            actor,
            action_type: action_type.into(),
            target: target.into(),
            payload: Value::Null,
            context: Value::Null,
            timestamp,
        }
    }

    pub fn with_payload(mut self, payload: Value) -> Self {
        self.payload = payload;
        self
    }

    pub fn with_context(mut self, context: Value) -> Self {
        self.context = context;
        self
    }
}
