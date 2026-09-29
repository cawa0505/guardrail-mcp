use std::path::Path;
use async_trait::async_trait;
use guardrail_core::{Action, GuardError, SoftVerifier, VerificationResult};
use tokio::process::Command;

pub struct CompilerVerifier;

impl CompilerVerifier {
    pub async fn run_check(project_root: impl AsRef<Path>) -> Result<(bool, String), GuardError> {
        let root = project_root.as_ref();
        
        let (cmd, args) = if root.join("Cargo.toml").exists() {
            ("cargo", vec!["check", "--message-format=short"])
        } else if root.join("package.json").exists() && root.join("tsconfig.json").exists() {
            ("npx", vec!["tsc", "--noEmit"])
        } else if root.join("go.mod").exists() {
            ("go", vec!["vet", "./..."])
        } else {
            return Ok((true, "No recognized compiler configuration found; passed by default".to_string()));
        };

        let output = Command::new(cmd)
            .args(&args)
            .current_dir(root)
            .output()
            .await
            .map_err(|e| GuardError::VerifierError(format!("Failed to spawn compiler check ({}): {}", cmd, e)))?;

        let success = output.status.success();
        let stderr = String::from_utf8_lossy(&output.stderr).to_string();
        let stdout = String::from_utf8_lossy(&output.stdout).to_string();
        let combined = if stderr.is_empty() { stdout } else { format!("{}\n{}", stdout, stderr) };

        Ok((success, combined))
    }
}

#[async_trait]
impl SoftVerifier for CompilerVerifier {
    fn id(&self) -> &str {
        "compiler_verifier"
    }

    async fn verify(&self, action: &Action) -> Result<VerificationResult, GuardError> {
        if action.action_type != "coding.patch" && action.action_type != "apply_patch" {
            return Ok(VerificationResult {
                passed: true,
                verifier_id: self.id().to_string(),
                message: "Non-coding action skips compiler verification".to_string(),
            });
        }

        let target_path = &action.target;
        let root = crate::patch::PatchEngine::find_project_root(target_path)
            .unwrap_or_else(|| std::env::current_dir().unwrap_or_default());

        let (passed, output) = Self::run_check(root).await?;

        Ok(VerificationResult {
            passed,
            verifier_id: self.id().to_string(),
            message: output,
        })
    }
}
