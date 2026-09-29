use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum DecisionKind {
    Allow,
    Deny,
    RequireVerification,
    RequireApproval,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Decision {
    pub kind: DecisionKind,
    pub reason: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub policy_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub verifier_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub commit_token: Option<String>,
    pub timestamp: i64,
}

impl Decision {
    pub fn allow(reason: impl Into<String>, timestamp: i64) -> Self {
        Self {
            kind: DecisionKind::Allow,
            reason: reason.into(),
            policy_id: None,
            verifier_id: None,
            commit_token: None,
            timestamp,
        }
    }

    pub fn deny(reason: impl Into<String>, timestamp: i64) -> Self {
        Self {
            kind: DecisionKind::Deny,
            reason: reason.into(),
            policy_id: None,
            verifier_id: None,
            commit_token: None,
            timestamp,
        }
    }

    pub fn require_verification(verifier_id: impl Into<String>, reason: impl Into<String>, timestamp: i64) -> Self {
        Self {
            kind: DecisionKind::RequireVerification,
            reason: reason.into(),
            policy_id: None,
            verifier_id: Some(verifier_id.into()),
            commit_token: None,
            timestamp,
        }
    }

    pub fn require_approval(reason: impl Into<String>, timestamp: i64) -> Self {
        Self {
            kind: DecisionKind::RequireApproval,
            reason: reason.into(),
            policy_id: None,
            verifier_id: None,
            commit_token: None,
            timestamp,
        }
    }

    pub fn with_policy(mut self, policy_id: impl Into<String>) -> Self {
        self.policy_id = Some(policy_id.into());
        self
    }

    pub fn with_token(mut self, token: impl Into<String>) -> Self {
        self.commit_token = Some(token.into());
        self
    }
}
