//! Soft Guard HTTP verifier client — mirrors the deleted Go `internal/softguard`.
//!
//! Each enabled verifier is POSTed a JSON `VerifierInput`; the response body is
//! read as `{ passed, message }`. A missing required verifier failure surfaces
//! as a `Decision::RequireVerification`.

use std::path::Path;
use std::time::Duration;

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VerifierConfig {
    pub name: String,
    #[serde(default)]
    pub r#type: String,
    pub url: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub api_token: String,
    #[serde(default)]
    pub enabled: bool,
    #[serde(default)]
    pub required: bool,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SoftGuardConfig {
    #[serde(default)]
    pub verifiers: Vec<VerifierConfig>,
}

impl SoftGuardConfig {
    pub fn load(dir: &Path) -> Result<Self, String> {
        let path = dir.join("softguard.json");
        match std::fs::read_to_string(&path) {
            Ok(data) => serde_json::from_str(&data).map_err(|e| format!("parse softguard config: {e}")),
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(Self { verifiers: Vec::new() }),
            Err(e) => Err(format!("read softguard config: {e}")),
        }
    }
}

#[derive(Debug, Clone, Serialize)]
pub struct VerifierInput {
    pub file_path: String,
    pub content: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub patch: Option<String>,
    pub phase: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VerifierResult {
    pub verifier_name: String,
    pub passed: bool,
    pub required: bool,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub message: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub error: String,
}

impl VerifierResult {
    fn failure(name: &str, required: bool, error: impl Into<String>) -> Self {
        Self {
            verifier_name: name.to_string(),
            passed: false,
            required,
            message: String::new(),
            error: error.into(),
        }
    }
}

/// Run every enabled verifier. Returns an empty vec when none are configured.
pub fn run_all(cfg: &SoftGuardConfig, input: &VerifierInput) -> Vec<VerifierResult> {
    let mut results = Vec::new();
    for vc in cfg.verifiers.iter().filter(|v| v.enabled) {
        results.push(run_one(vc, input));
    }
    results
}

/// Returns the first required verifier that failed, if any.
pub fn first_required_failure(results: &[VerifierResult]) -> Option<&VerifierResult> {
    results.iter().find(|r| r.required && !r.passed)
}

fn run_one(vc: &VerifierConfig, input: &VerifierInput) -> VerifierResult {
    let mut req = ureq::post(&vc.url).config().timeout_global(Some(Duration::from_secs(30))).build();
    if !vc.api_token.is_empty() {
        req = req.header("Authorization", &format!("Bearer {}", vc.api_token));
    }

    let resp = match req.send_json(input) {
        Ok(r) => r,
        Err(e) => return VerifierResult::failure(&vc.name, vc.required, format!("http call: {e}")),
    };

    let mut resp = resp;
    let parsed: Result<VerifierResponse, _> = resp.body_mut().read_json();
    match parsed {
        Ok(vr) => VerifierResult {
            verifier_name: vc.name.clone(),
            passed: vr.passed,
            required: vc.required,
            message: vr.message,
            error: String::new(),
        },
        Err(e) => VerifierResult::failure(&vc.name, vc.required, format!("parse response: {e}")),
    }
}

#[derive(Debug, Deserialize)]
struct VerifierResponse {
    #[serde(default)]
    passed: bool,
    #[serde(default)]
    message: String,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn config_load_missing_file_is_empty() {
        let dir = tempfile::tempdir().unwrap();
        let cfg = SoftGuardConfig::load(dir.path()).unwrap();
        assert!(cfg.verifiers.is_empty());
    }

    #[test]
    fn config_parses_example_shape() {
        let dir = tempfile::tempdir().unwrap();
        std::fs::write(
            dir.path().join("softguard.json"),
            r#"{"verifiers":[
                {"name":"c","type":"compiler","url":"http://x/verify","enabled":true,"required":true},
                {"name":"l","type":"llm","url":"http://y/verify","enabled":false,"required":false}
            ]}"#,
        )
        .unwrap();
        let cfg = SoftGuardConfig::load(dir.path()).unwrap();
        assert_eq!(cfg.verifiers.len(), 2);
        assert!(cfg.verifiers[0].enabled && cfg.verifiers[0].required);
        assert!(!cfg.verifiers[1].enabled);
    }

    #[test]
    fn first_required_failure_selects_only_required() {
        let results = vec![
            VerifierResult { verifier_name: "opt".into(), passed: false, required: false, message: String::new(), error: "x".into() },
            VerifierResult { verifier_name: "req".into(), passed: false, required: true, message: String::new(), error: "boom".into() },
            VerifierResult { verifier_name: "req2".into(), passed: false, required: true, message: String::new(), error: "later".into() },
        ];
        let failed = first_required_failure(&results).unwrap();
        assert_eq!(failed.verifier_name, "req");
    }

    #[test]
    fn first_required_failure_none_when_all_pass() {
        let results = vec![VerifierResult {
            verifier_name: "req".into(),
            passed: true,
            required: true,
            message: "ok".into(),
            error: String::new(),
        }];
        assert!(first_required_failure(&results).is_none());
    }
}
