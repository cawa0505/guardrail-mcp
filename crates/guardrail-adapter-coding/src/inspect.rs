use std::path::Path;
use regex::Regex;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct InspectResult {
    pub language: String,
    pub total_lines: usize,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub token_reduced_from: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub token_reduced_to: Option<usize>,
    pub content: String,
    #[serde(default)]
    pub truncated: bool,
}

pub struct InspectEngine;

impl InspectEngine {
    pub fn detect_lang(path: impl AsRef<Path>) -> &'static str {
        match path.as_ref().extension().and_then(|e| e.to_str()).unwrap_or("") {
            "rs" => "rust",
            "ts" => "typescript",
            "tsx" => "tsx",
            "js" => "javascript",
            "jsx" => "jsx",
            "mjs" => "javascript",
            "py" => "python",
            "go" => "go",
            _ => "unknown",
        }
    }

    /// Strip comments, redundant empty lines, collapse repetition
    pub fn reduce_text(text: &str) -> String {
        let re_single = Regex::new(r"^\s*(//|#|;).*$").unwrap();
        let re_ml_start = Regex::new(r"/\*").unwrap();
        let re_ml_end = Regex::new(r"\*/").unwrap();

        let mut lines = Vec::new();
        let mut in_ml = false;
        let mut prev_line = String::new();
        let mut repeat_count = 0;

        for raw_line in text.lines() {
            let mut line = raw_line.to_string();

            if in_ml {
                if re_ml_end.is_match(&line) {
                    in_ml = false;
                }
                continue;
            }

            if re_ml_start.is_match(&line) {
                if !re_ml_end.is_match(&line) {
                    in_ml = true;
                    continue;
                }
                if let Some(mat) = re_ml_start.find(&line) {
                    line = line[..mat.start()].trim_end().to_string();
                }
            }

            if !line.starts_with("#!") && re_single.is_match(&line) {
                continue;
            }

            if line.trim().is_empty() {
                continue;
            }

            let trimmed = line.trim();
            if trimmed == prev_line {
                repeat_count += 1;
                if repeat_count >= 3 {
                    continue;
                }
            } else {
                repeat_count = 0;
                prev_line = trimmed.to_string();
            }

            lines.push(line);
        }

        lines.join("\n")
    }

    pub fn inspect_content(
        path: impl AsRef<Path>,
        raw_text: &str,
        mode: &str,
    ) -> InspectResult {
        let total_lines = raw_text.lines().count();
        let lang = Self::detect_lang(&path).to_string();

        match mode {
            "skeleton" | "full_cleaned" => {
                let reduced = Self::reduce_text(raw_text);
                let orig_chars = raw_text.chars().count();
                let reduced_chars = reduced.chars().count();

                InspectResult {
                    language: lang,
                    total_lines,
                    token_reduced_from: Some(orig_chars / 4),
                    token_reduced_to: Some(reduced_chars / 4),
                    content: reduced,
                    truncated: false,
                }
            }
            _ => InspectResult {
                language: lang,
                total_lines,
                token_reduced_from: None,
                token_reduced_to: None,
                content: raw_text.to_string(),
                truncated: false,
            },
        }
    }
}
