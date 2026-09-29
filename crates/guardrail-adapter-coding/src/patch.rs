use std::path::{Path, PathBuf};
use thiserror::Error;

const MIN_PATCH_LINES: usize = 3;
const MIN_PATCH_CHARS: usize = 10;

#[derive(Debug, Error, PartialEq, Eq)]
pub enum PatchError {
    #[error("patch: search block is empty")]
    EmptySearch,
    #[error("patch: too short ({chars} chars, minimum {MIN_PATCH_CHARS})")]
    TooShort { chars: usize },
    #[error("patch: too few lines ({lines}, minimum {MIN_PATCH_LINES})")]
    TooFewLines { lines: usize },
    #[error("search block not found in file\n\nFile content preview:\n{preview}")]
    SearchNotFound { preview: String },
    #[error("io error: {0}")]
    Io(String),
}

pub struct PatchEngine;

impl PatchEngine {
    /// Parity with Go ValidatePatch
    pub fn validate_patch(search: &str, replace: &str) -> Result<(), PatchError> {
        if search.trim().is_empty() {
            return Err(PatchError::EmptySearch);
        }

        let total = format!("{}{}", search, replace);
        let total_trimmed = total.trim();
        if total_trimmed.len() < MIN_PATCH_CHARS {
            return Err(PatchError::TooShort {
                chars: total_trimmed.len(),
            });
        }

        let mut lines = 0;
        if !search.is_empty() {
            lines += search.matches('\n').count() + 1;
        }
        if !replace.is_empty() {
            lines += replace.matches('\n').count() + 1;
        }

        if lines < MIN_PATCH_LINES {
            return Err(PatchError::TooFewLines { lines });
        }

        Ok(())
    }

    /// Parity with Go ApplyPatchContent
    pub fn apply_patch_content(
        original: &str,
        search: &str,
        replace: &str,
    ) -> Result<String, PatchError> {
        match original.find(search) {
            Some(idx) => {
                let mut result = String::with_capacity(original.len() + replace.len());
                result.push_str(&original[..idx]);
                result.push_str(replace);
                result.push_str(&original[idx + search.len()..]);
                Ok(result)
            }
            None => {
                let preview: Vec<&str> = original.lines().take(11).collect();
                let mut ctx = preview.join("\n");
                if original.lines().count() > 11 {
                    ctx.push_str("\n...");
                }
                Err(PatchError::SearchNotFound { preview: ctx })
            }
        }
    }

    /// Locate project root based on project markers
    pub fn find_project_root(file_path: impl AsRef<Path>) -> Option<PathBuf> {
        let markers = [
            "Cargo.toml",
            "tsconfig.json",
            "go.mod",
            "package.json",
            "pyproject.toml",
            "setup.py",
            "CMakeLists.txt",
            "Makefile",
        ];

        let mut current = if file_path.as_ref().is_file() {
            file_path.as_ref().parent()?
        } else {
            file_path.as_ref()
        };

        loop {
            for m in &markers {
                if current.join(m).exists() {
                    return Some(current.to_path_buf());
                }
            }
            match current.parent() {
                Some(p) if p != current => current = p,
                _ => break,
            }
        }
        None
    }
}
