use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use thiserror::Error;

pub const DEFAULT_TTL_SECS: i64 = 30 * 60; // 30 minutes

#[derive(Debug, Error, PartialEq, Eq)]
pub enum TokenError {
    #[error("token is expired (expired at {expired_at}, now {now})")]
    Expired { expired_at: i64, now: i64 },
    #[error("token has already been used")]
    AlreadyUsed,
    #[error("token has been revoked")]
    Revoked,
    #[error("proposal hash mismatch: expected {expected}, got {actual}")]
    ProposalHashMismatch { expected: String, actual: String },
    #[error("workspace path mismatch: expected {expected}, got {actual}")]
    WorkspaceMismatch { expected: String, actual: String },
    #[error("git revision mismatch: expected {expected}, got {actual}")]
    RevisionMismatch { expected: String, actual: String },
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct TokenBindings {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub proposal_hash: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub workspace_path: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub revision: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct CommitToken {
    pub id: String,
    pub created_at: i64,
    pub expires_at: i64,
    pub bindings: TokenBindings,
    #[serde(default)]
    pub used: bool,
    #[serde(default)]
    pub revoked: bool,
}

impl CommitToken {
    pub fn new(
        id: impl Into<String>,
        proposal_hash: impl Into<String>,
        workspace_path: impl Into<String>,
        revision: impl Into<String>,
        now_secs: i64,
        ttl_secs: Option<i64>,
    ) -> Self {
        let ttl = ttl_secs.unwrap_or(DEFAULT_TTL_SECS);
        Self {
            id: id.into(),
            created_at: now_secs,
            expires_at: now_secs + ttl,
            bindings: TokenBindings {
                proposal_hash: proposal_hash.into(),
                workspace_path: workspace_path.into(),
                revision: revision.into(),
            },
            used: false,
            revoked: false,
        }
    }

    pub fn is_expired(&self, now_secs: i64) -> bool {
        now_secs > self.expires_at
    }

    pub fn is_valid(&self, now_secs: i64) -> bool {
        !self.is_expired(now_secs) && !self.used && !self.revoked
    }

    pub fn validate(
        &self,
        proposal_hash: Option<&str>,
        workspace_path: Option<&str>,
        revision: Option<&str>,
        now_secs: i64,
    ) -> Result<(), TokenError> {
        if self.is_expired(now_secs) {
            return Err(TokenError::Expired {
                expired_at: self.expires_at,
                now: now_secs,
            });
        }
        if self.used {
            return Err(TokenError::AlreadyUsed);
        }
        if self.revoked {
            return Err(TokenError::Revoked);
        }

        if let Some(expected) = proposal_hash {
            if !self.bindings.proposal_hash.is_empty() && self.bindings.proposal_hash != expected {
                return Err(TokenError::ProposalHashMismatch {
                    expected: self.bindings.proposal_hash.clone(),
                    actual: expected.to_string(),
                });
            }
        }

        if let Some(expected) = workspace_path {
            if !self.bindings.workspace_path.is_empty() && self.bindings.workspace_path != expected {
                return Err(TokenError::WorkspaceMismatch {
                    expected: self.bindings.workspace_path.clone(),
                    actual: expected.to_string(),
                });
            }
        }

        if let Some(expected) = revision {
            if !self.bindings.revision.is_empty() && self.bindings.revision != expected {
                return Err(TokenError::RevisionMismatch {
                    expected: self.bindings.revision.clone(),
                    actual: expected.to_string(),
                });
            }
        }

        Ok(())
    }

    pub fn consume(&mut self, now_secs: i64) -> Result<(), TokenError> {
        self.validate(None, None, None, now_secs)?;
        self.used = true;
        Ok(())
    }

    pub fn revoke(&mut self) {
        self.revoked = true;
    }
}

pub fn hash_proposal_content(content: &str) -> String {
    let mut hasher = Sha256::new();
    hasher.update(content.as_bytes());
    hex::encode(hasher.finalize())
}
