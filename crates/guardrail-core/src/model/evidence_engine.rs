use std::fs::{File, OpenOptions};
use std::io::{BufRead, BufReader, Write};
use std::path::{Path, PathBuf};
use sha2::{Digest, Sha256};
use thiserror::Error;

use super::EvidenceRecord;

#[derive(Debug, Error)]
pub enum EvidenceError {
    #[error("io error: {0}")]
    Io(#[from] std::io::Error),
    #[error("json serialization/deserialization error: {0}")]
    Json(#[from] serde_json::Error),
    #[error("hash chain corruption at event {event_id}: expected parent {expected_parent}, got {actual_parent}")]
    ChainCorruption {
        event_id: String,
        expected_parent: String,
        actual_parent: String,
    },
}

pub struct EvidenceEngine {
    file_path: PathBuf,
}

impl EvidenceEngine {
    pub fn new(file_path: impl AsRef<Path>) -> Self {
        Self {
            file_path: file_path.as_ref().to_path_buf(),
        }
    }

    /// Calculate SHA-256 digest of serialized EvidenceRecord JSON bytes
    pub fn hash_record(record: &EvidenceRecord) -> Result<String, EvidenceError> {
        let bytes = serde_json::to_vec(record)?;
        let mut hasher = Sha256::new();
        hasher.update(&bytes);
        Ok(hex::encode(hasher.finalize()))
    }

    /// Read last record's hash to chain forward
    pub fn last_record_hash(&self) -> Result<Option<String>, EvidenceError> {
        if !self.file_path.exists() {
            return Ok(None);
        }

        let file = File::open(&self.file_path)?;
        let reader = BufReader::new(file);
        let mut last_line = None;

        for line in reader.lines() {
            let l = line?;
            if !l.trim().is_empty() {
                last_line = Some(l);
            }
        }

        match last_line {
            Some(line) => {
                let rec: EvidenceRecord = serde_json::from_str(&line)?;
                let hash = Self::hash_record(&rec)?;
                Ok(Some(hash))
            }
            None => Ok(None),
        }
    }

    /// Append record with automatic SHA-256 parent_event chaining
    pub fn append(&self, mut record: EvidenceRecord) -> Result<(EvidenceRecord, String), EvidenceError> {
        if let Some(parent) = self.file_path.parent() {
            std::fs::create_dir_all(parent)?;
        }

        let last_hash = self.last_record_hash()?;
        if let Some(parent_hash) = last_hash {
            record.parent_event = Some(parent_hash);
        } else {
            // Genesis event
            record.parent_event = None;
        }

        let current_hash = Self::hash_record(&record)?;

        let mut file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&self.file_path)?;

        let line = serde_json::to_string(&record)?;
        writeln!(file, "{}", line)?;

        Ok((record, current_hash))
    }

    /// Audit full ledger hash chain
    pub fn verify_chain(&self) -> Result<usize, EvidenceError> {
        if !self.file_path.exists() {
            return Ok(0);
        }

        let file = File::open(&self.file_path)?;
        let reader = BufReader::new(file);
        let mut prev_hash: Option<String> = None;
        let mut count = 0;

        for line in reader.lines() {
            let l = line?;
            if l.trim().is_empty() {
                continue;
            }
            let rec: EvidenceRecord = serde_json::from_str(&l)?;

            match (rec.parent_event.as_deref(), prev_hash.as_deref()) {
                (None, None) => {} // Genesis ok
                (Some(parent), Some(prev)) if parent == prev => {} // Valid chain step
                (actual, expected) => {
                    return Err(EvidenceError::ChainCorruption {
                        event_id: rec.event_id,
                        expected_parent: expected.unwrap_or("none").to_string(),
                        actual_parent: actual.unwrap_or("none").to_string(),
                    });
                }
            }

            prev_hash = Some(Self::hash_record(&rec)?);
            count += 1;
        }

        Ok(count)
    }
}
