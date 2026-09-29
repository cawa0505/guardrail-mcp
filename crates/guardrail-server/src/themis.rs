//! Themis Enterprise Connector for GuardrailMcp
//!
//! Complies with CONTRACT.md (v1.1) and Themis MVP requirements:
//! 1. Remote Policy Sync: GET /api/v1/policy/{tenant_id}/{agent_id} with ETag & If-None-Match caching.
//! 2. Non-blocking Evidence Batch Ingestion: POST /api/v1/evidence/batch with background task.
//! 3. Remote Human Approval Protocol:
//!    - POST /api/v1/approval/request (state=PENDING)
//!    - Poll GET /api/v1/approval/{request_id} until APPROVED/REJECTED/EXPIRED
//!    - Verify Ed25519 signature from Themis authority
//! 4. CONTRACT §1.5 Session Closing Anchor:
//!    - Emits records: [] + chain_anchor on session close.

use std::path::Path;
use std::sync::atomic::{AtomicI64, Ordering};
use std::sync::Arc;
use std::time::Duration;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use tokio::sync::{mpsc, Mutex};
use ed25519_dalek::{Signature, Verifier, VerifyingKey};
use sha2::{Digest, Sha256};

pub const GENESIS: &str = "genesis";

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ThemisConfig {
    #[serde(default = "default_endpoint")]
    pub endpoint: String,
    #[serde(default = "default_tenant")]
    pub tenant_id: String,
    #[serde(default = "default_agent")]
    pub agent_id: String,
    #[serde(default)]
    pub instance_id: String,
    #[serde(default)]
    pub machine_id: String,
    #[serde(default)]
    pub server_public_key: String,
    #[serde(default)]
    pub enabled: bool,
    #[serde(default = "default_poll_interval_millis")]
    pub poll_interval_millis: u64,
}

fn default_endpoint() -> String {
    "http://127.0.0.1:8080".to_string()
}
fn default_tenant() -> String {
    "default".to_string()
}
fn default_agent() -> String {
    "coding-agent".to_string()
}
fn default_poll_interval_millis() -> u64 {
    500
}

impl Default for ThemisConfig {
    fn default() -> Self {
        Self {
            endpoint: default_endpoint(),
            tenant_id: default_tenant(),
            agent_id: default_agent(),
            instance_id: format!("inst-{}", std::process::id()),
            machine_id: "local-machine".to_string(),
            server_public_key: String::new(),
            enabled: false,
            poll_interval_millis: default_poll_interval_millis(),
        }
    }
}

impl ThemisConfig {
    pub fn load(dir: &Path) -> Result<Self, String> {
        let path = dir.join("themis.json");
        match std::fs::read_to_string(&path) {
            Ok(content) => serde_json::from_str(&content).map_err(|e| format!("parse themis config: {e}")),
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => {
                if let Ok(endpoint) = std::env::var("THEMIS_ENDPOINT") {
                    let mut cfg = Self::default();
                    cfg.endpoint = endpoint;
                    cfg.enabled = true;
                    if let Ok(t) = std::env::var("THEMIS_TENANT_ID") { cfg.tenant_id = t; }
                    if let Ok(a) = std::env::var("THEMIS_AGENT_ID") { cfg.agent_id = a; }
                    if let Ok(pk) = std::env::var("THEMIS_SERVER_PUBLIC_KEY") { cfg.server_public_key = pk; }
                    Ok(cfg)
                } else {
                    Ok(Self::default())
                }
            }
            Err(e) => Err(format!("read themis config: {e}")),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ActorType {
    Agent,
    Human,
    Service,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Actor {
    pub actor_type: ActorType,
    pub id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RemotePolicyRule {
    pub id: String,
    pub effect: String,
    pub actions: Vec<String>,
    pub resources: Vec<String>,
    #[serde(default)]
    pub conditions: Value,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RemotePolicy {
    pub version: String,
    pub hash: String,
    pub rules: Vec<RemotePolicyRule>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EvidenceRecord {
    pub event_id: String,
    pub parent_event_hash: String,
    pub event_hash: String,
    pub timestamp: String,
    pub actor: Actor,
    pub action: String,
    pub resource: String,
    pub decision: String,
    pub policy_version: String,
    pub policy_hash: String,
    pub verifier: Option<String>,
    pub verification_result: Option<String>,
    pub input: Value,
    pub output: Value,
    pub metadata: Value,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChainAnchor {
    pub head_event_hash: String,
    pub record_count: i64,
    pub instance_id: String,
    pub machine_id: String,
    pub prev_session_id: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BatchBody {
    pub records: Vec<EvidenceRecord>,
    pub session_id: String,
    pub tenant_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub chain_anchor: Option<ChainAnchor>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ApprovalScope {
    pub action: String,
    pub resource: String,
    pub session_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ApprovalSignature {
    pub request_id: String,
    pub signature: String,
    pub public_key: String,
    pub signed_at: String,
    pub expires_at: String,
    pub scope: ApprovalScope,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ApprovalState {
    Pending,
    Approved,
    Rejected,
    Expired,
    Cancelled,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApprovalDetailResponse {
    pub state: ApprovalState,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub signature: Option<ApprovalSignature>,
}

pub struct ThemisClient {
    pub cfg: ThemisConfig,
    cached_policy: Mutex<Option<(String, RemotePolicy)>>, // (etag, policy)
    evidence_tx: mpsc::Sender<EvidenceRecord>,
    session_id: String,
    record_count: AtomicI64,
    last_event_hash: Mutex<String>,
}

impl ThemisClient {
    pub fn new(cfg: ThemisConfig, session_id: String) -> Arc<Self> {
        let (tx, mut rx) = mpsc::channel::<EvidenceRecord>(100);
        let client = Arc::new(Self {
            cfg: cfg.clone(),
            cached_policy: Mutex::new(None),
            evidence_tx: tx,
            session_id: session_id.clone(),
            record_count: AtomicI64::new(0),
            last_event_hash: Mutex::new(GENESIS.to_string()),
        });

        if cfg.enabled && tokio::runtime::Handle::try_current().is_ok() {
            let endpoint = cfg.endpoint.clone();
            let tenant_id = cfg.tenant_id.clone();
            let sess = session_id;
            tokio::spawn(async move {
                let mut buffer = Vec::new();
                let mut interval = tokio::time::interval(Duration::from_millis(500));
                loop {
                    tokio::select! {
                        Some(rec) = rx.recv() => {
                            buffer.push(rec);
                            if buffer.len() >= 10 {
                                Self::flush_batch(&endpoint, &tenant_id, &sess, std::mem::take(&mut buffer)).await;
                            }
                        }
                        _ = interval.tick() => {
                            if !buffer.is_empty() {
                                Self::flush_batch(&endpoint, &tenant_id, &sess, std::mem::take(&mut buffer)).await;
                            }
                        }
                        else => break,
                    }
                }
            });
        }

        client
    }

    pub fn is_enabled(&self) -> bool {
        self.cfg.enabled
    }

    #[allow(dead_code)]
    pub fn session_id(&self) -> &str {
        &self.session_id
    }

    pub async fn fetch_policy(&self) -> Result<Option<RemotePolicy>, String> {
        if !self.cfg.enabled {
            return Ok(None);
        }
        let url = format!(
            "{}/api/v1/policy/{}/{}",
            self.cfg.endpoint.trim_end_matches('/'),
            self.cfg.tenant_id,
            self.cfg.agent_id
        );

        let cached_etag = {
            let guard = self.cached_policy.lock().await;
            guard.as_ref().map(|(etag, _)| etag.clone())
        };

        let mut req = ureq::get(&url);
        if let Some(etag) = cached_etag {
            req = req.header("If-None-Match", &etag);
        }

        let res = req.call();
        match res {
            Ok(mut response) => {
                let status = response.status().as_u16();
                if status == 304 {
                    let guard = self.cached_policy.lock().await;
                    return Ok(guard.as_ref().map(|(_, p)| p.clone()));
                }
                if status == 200 {
                    let new_etag = response
                        .headers()
                        .get("ETag")
                        .and_then(|v| v.to_str().ok())
                        .unwrap_or("")
                        .to_string();
                    let body: Value = response
                        .body_mut()
                        .read_json()
                        .map_err(|e| format!("parse policy json: {e}"))?;
                    let policy_val = body.get("policy").unwrap_or(&body);
                    let policy: RemotePolicy = serde_json::from_value(policy_val.clone())
                        .map_err(|e| format!("deserialize policy: {e}"))?;
                    let mut guard = self.cached_policy.lock().await;
                    *guard = Some((new_etag, policy.clone()));
                    return Ok(Some(policy));
                }
                Err(format!("policy fetch status {status}"))
            }
            Err(e) => Err(format!("policy request failed: {e}")),
        }
    }

    pub async fn report_evidence(&self, mut rec: EvidenceRecord) {
        if !self.cfg.enabled {
            return;
        }
        {
            let mut last = self.last_event_hash.lock().await;
            rec.parent_event_hash = last.clone();
            rec.event_hash = compute_record_hash(&rec);
            *last = rec.event_hash.clone();
        }
        self.record_count.fetch_add(1, Ordering::SeqCst);
        let _ = self.evidence_tx.try_send(rec);
    }

    async fn flush_batch(endpoint: &str, tenant_id: &str, session_id: &str, records: Vec<EvidenceRecord>) {
        if records.is_empty() {
            return;
        }
        let url = format!("{}/api/v1/evidence/batch", endpoint.trim_end_matches('/'));
        let batch = BatchBody {
            session_id: session_id.to_string(),
            tenant_id: tenant_id.to_string(),
            records,
            chain_anchor: None,
        };
        let _ = tokio::task::spawn_blocking(move || {
            let _ = ureq::post(&url).send_json(&batch);
        }).await;
    }

    pub async fn close_session(&self, prev_session_id: Option<String>) -> Result<(), String> {
        if !self.cfg.enabled {
            return Ok(());
        }
        let head_event_hash = self.last_event_hash.lock().await.clone();
        let record_count = self.record_count.load(Ordering::SeqCst);

        let anchor = ChainAnchor {
            head_event_hash,
            record_count,
            instance_id: self.cfg.instance_id.clone(),
            machine_id: self.cfg.machine_id.clone(),
            prev_session_id,
        };

        let batch = BatchBody {
            session_id: self.session_id.clone(),
            tenant_id: self.cfg.tenant_id.clone(),
            records: vec![],
            chain_anchor: Some(anchor),
        };

        let url = format!("{}/api/v1/evidence/batch", self.cfg.endpoint.trim_end_matches('/'));
        tokio::task::spawn_blocking(move || {
            let res = ureq::post(&url).send_json(&batch);
            match res {
                Ok(_) => Ok(()),
                Err(e) => Err(format!("failed to send closing anchor: {e}")),
            }
        })
        .await
        .map_err(|e| format!("join error: {e}"))?
    }

    pub async fn request_and_await_approval(
        &self,
        request_id: &str,
        action: &str,
        resource: &str,
        policy_rule_id: &str,
        expires_at: &str,
        timeout: Duration,
    ) -> Result<ApprovalSignature, String> {
        if !self.cfg.enabled {
            return Err("Themis connector disabled".to_string());
        }

        let create_url = format!("{}/api/v1/approval/request", self.cfg.endpoint.trim_end_matches('/'));
        let req_body = json!({
            "request_id": request_id,
            "session_id": self.session_id,
            "tenant_id": self.cfg.tenant_id,
            "actor": {
                "actor_type": "agent",
                "id": self.cfg.agent_id,
            },
            "action": action,
            "resource": resource,
            "policy_rule_id": policy_rule_id,
            "context": {},
            "expires_at": expires_at,
        });

        let status = tokio::task::spawn_blocking(move || {
            ureq::post(&create_url)
                .send_json(&req_body)
                .map(|r| r.status().as_u16())
                .map_err(|e| format!("approval request creation failed: {e}"))
        })
        .await
        .map_err(|e| format!("join error: {e}"))??;

        if status != 200 && status != 201 {
            return Err(format!("approval request status {status}"));
        }

        let poll_url = format!(
            "{}/api/v1/approval/{}",
            self.cfg.endpoint.trim_end_matches('/'),
            request_id
        );

        let start = std::time::Instant::now();
        let interval = Duration::from_millis(self.cfg.poll_interval_millis.max(100));

        while start.elapsed() < timeout {
            tokio::time::sleep(interval).await;
            let url = poll_url.clone();
            let poll_res = tokio::task::spawn_blocking(move || {
                ureq::get(&url).call().and_then(|mut r| {
                    if r.status().as_u16() == 200 {
                        r.body_mut().read_json::<ApprovalDetailResponse>().map(Some)
                    } else {
                        Ok(None)
                    }
                })
            })
            .await
            .map_err(|e| format!("join error: {e}"))?;

            if let Ok(Some(detail)) = poll_res {
                match detail.state {
                    ApprovalState::Approved => {
                        let sig = detail.signature.ok_or_else(|| "approved but missing signature".to_string())?;
                        self.verify_signature(&sig, action, resource)?;
                        return Ok(sig);
                    }
                    ApprovalState::Rejected => return Err("human operator rejected the request".to_string()),
                    ApprovalState::Expired => return Err("approval request expired".to_string()),
                    ApprovalState::Cancelled => return Err("approval request cancelled".to_string()),
                    ApprovalState::Pending => {}
                }
            }
        }

        Err("approval request timed out".to_string())
    }

    pub fn verify_signature(
        &self,
        sig: &ApprovalSignature,
        expected_action: &str,
        expected_resource: &str,
    ) -> Result<(), String> {
        if sig.scope.action != expected_action
            || sig.scope.resource != expected_resource
            || sig.scope.session_id != self.session_id
        {
            return Err("approval signature scope mismatch".to_string());
        }

        let pubkey_str = if !self.cfg.server_public_key.is_empty() {
            &self.cfg.server_public_key
        } else {
            &sig.public_key
        };

        let pubkey_bytes = decode_hex_or_base64(pubkey_str).map_err(|e| format!("decode pubkey error: {e}"))?;
        let pubkey_arr: [u8; 32] = pubkey_bytes
            .try_into()
            .map_err(|_| "public key must be 32 bytes".to_string())?;
        let verifying_key = VerifyingKey::from_bytes(&pubkey_arr)
            .map_err(|e| format!("invalid verifying key: {e}"))?;

        let sig_bytes = decode_hex_or_base64(&sig.signature).map_err(|e| format!("decode signature error: {e}"))?;
        let sig_arr: [u8; 64] = sig_bytes
            .try_into()
            .map_err(|_| "signature must be 64 bytes".to_string())?;
        let signature = Signature::from_bytes(&sig_arr);

        // CONTRACT §3.5: canonical signing payload is
        // format!("{request_id}{action}{resource}{session_id}{expires_at}")
        let payload = format!(
            "{}{}{}{}{}",
            sig.request_id,
            sig.scope.action,
            sig.scope.resource,
            sig.scope.session_id,
            sig.expires_at
        );

        verifying_key
            .verify(payload.as_bytes(), &signature)
            .map_err(|e| format!("ed25519 signature verification failed: {e}"))?;

        Ok(())
    }
}

pub fn compute_record_hash(rec: &EvidenceRecord) -> String {
    let payload = json!({
        "action": rec.action,
        "actor": rec.actor,
        "decision": rec.decision,
        "event_id": rec.event_id,
        "input": rec.input,
        "metadata": rec.metadata,
        "output": rec.output,
        "parent_event_hash": rec.parent_event_hash,
        "policy_hash": rec.policy_hash,
        "policy_version": rec.policy_version,
        "resource": rec.resource,
        "timestamp": rec.timestamp,
        "verification_result": rec.verification_result,
        "verifier": rec.verifier,
    });
    let canonical = canonical_json_string(&payload);
    let mut hasher = Sha256::new();
    hasher.update(canonical.as_bytes());
    hex::encode(hasher.finalize())
}

pub fn canonical_json_string(value: &Value) -> String {
    match value {
        Value::Null => "null".to_string(),
        Value::Bool(b) => b.to_string(),
        Value::Number(n) => n.to_string(),
        Value::String(s) => serde_json::to_string(s).unwrap_or_default(),
        Value::Array(arr) => {
            let items: Vec<String> = arr.iter().map(canonical_json_string).collect();
            format!("[{}]", items.join(","))
        }
        Value::Object(map) => {
            let mut keys: Vec<&String> = map.keys().collect();
            keys.sort();
            let entries: Vec<String> = keys
                .into_iter()
                .map(|k| format!("{}:{}", serde_json::to_string(k).unwrap_or_default(), canonical_json_string(&map[k])))
                .collect();
            format!("{{{}}}", entries.join(","))
        }
    }
}

fn decode_hex_or_base64(input: &str) -> Result<Vec<u8>, String> {
    if input.len() % 2 == 0 && input.chars().all(|c| c.is_ascii_hexdigit()) {
        if let Ok(b) = hex::decode(input) {
            return Ok(b);
        }
    }
    decode_base64(input).map_err(|e| format!("base64 decode error: {e}"))
}

fn decode_base64(s: &str) -> Result<Vec<u8>, &'static str> {
    let s = s.trim_end_matches('=');
    let mut out = Vec::new();
    let mut buf: u32 = 0;
    let mut bits = 0;
    for c in s.chars() {
        let val = match c {
            'A'..='Z' => c as u32 - 'A' as u32,
            'a'..='z' => c as u32 - 'a' as u32 + 26,
            '0'..='9' => c as u32 - '0' as u32 + 52,
            '+' => 62,
            '/' => 63,
            _ => return Err("invalid base64 char"),
        };
        buf = (buf << 6) | val;
        bits += 6;
        if bits >= 8 {
            bits -= 8;
            out.push((buf >> bits) as u8);
            buf &= (1 << bits) - 1;
        }
    }
    Ok(out)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_canonical_json_sorts_keys() {
        let v = json!({ "z": 1, "a": "hello", "m": [3, 2, 1] });
        assert_eq!(canonical_json_string(&v), "{\"a\":\"hello\",\"m\":[3,2,1],\"z\":1}");
    }

    #[test]
    fn test_verify_signature_roundtrip() {
        use ed25519_dalek::{Signer, SigningKey};

        // Create a 32-byte secret key and signing key
        let secret = [42u8; 32];
        let signing_key = SigningKey::from_bytes(&secret);
        let verifying_key = signing_key.verifying_key();
        let pubkey_hex = hex::encode(verifying_key.as_bytes());

        let session_id = "sess-test-123";
        let action = "write_file";
        let resource = "src/secret.rs";
        let request_id = "req-999";
        let expires_at = "1999999999";

        // CONTRACT §3.5 format: {request_id}{action}{resource}{session_id}{expires_at}
        let payload = format!("{request_id}{action}{resource}{session_id}{expires_at}");
        let sig = signing_key.sign(payload.as_bytes());
        let sig_hex = hex::encode(sig.to_bytes());

        let cfg = ThemisConfig {
            enabled: true,
            endpoint: "http://localhost:9999".to_string(),
            tenant_id: "tenant-demo".to_string(),
            agent_id: "agent-alpha".to_string(),
            instance_id: "inst-1".to_string(),
            machine_id: "mac-1".to_string(),
            server_public_key: pubkey_hex.clone(),
            poll_interval_millis: 100,
        };

        let client = ThemisClient::new(cfg, session_id.to_string());

        let approval_sig = ApprovalSignature {
            request_id: request_id.to_string(),
            signed_at: "1720000000".to_string(),
            expires_at: expires_at.to_string(),
            scope: ApprovalScope {
                action: action.to_string(),
                resource: resource.to_string(),
                session_id: client.session_id().to_string(),
            },
            signature: sig_hex,
            public_key: pubkey_hex,
        };

        assert!(client.verify_signature(&approval_sig, action, resource).is_ok());

        // Scope mismatch should fail
        assert!(client.verify_signature(&approval_sig, "other_action", resource).is_err());
        assert!(client.verify_signature(&approval_sig, action, "other_resource").is_err());
    }

    #[tokio::test]
    async fn test_evidence_reporting_channel() {
        let cfg = ThemisConfig {
            enabled: true,
            endpoint: "http://localhost:9999".to_string(),
            tenant_id: "t-1".to_string(),
            agent_id: "a-1".to_string(),
            instance_id: "inst-1".to_string(),
            machine_id: "mac-1".to_string(),
            server_public_key: String::new(),
            poll_interval_millis: 100,
        };

        let client = ThemisClient::new(cfg, "sess-anchor-test".to_string());
        let rec = EvidenceRecord {
            event_id: "ev-1".to_string(),
            parent_event_hash: String::new(),
            event_hash: String::new(),
            timestamp: "100".to_string(),
            actor: Actor { actor_type: ActorType::Agent, id: "a-1".to_string() },
            action: "apply_patch".to_string(),
            resource: "main.rs".to_string(),
            decision: "ALLOW".to_string(),
            policy_version: "1".to_string(),
            policy_hash: "h".to_string(),
            verifier: None,
            verification_result: None,
            input: json!({}),
            output: json!({}),
            metadata: json!({}),
        };

        // report_evidence should succeed over the bounded mpsc channel
        client.report_evidence(rec).await;
    }

    #[tokio::test]
    async fn test_themis_connector_mock_http_roundtrip() {
        use ed25519_dalek::{Signer, SigningKey};
        use std::io::{BufRead, BufReader, Write};
        use std::net::TcpListener;
        use std::sync::atomic::{AtomicUsize, Ordering};

        let secret = [88u8; 32];
        let signing_key = SigningKey::from_bytes(&secret);
        let pubkey_hex = hex::encode(signing_key.verifying_key().as_bytes());

        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();

        let poll_count = Arc::new(AtomicUsize::new(0));
        let poll_count_server = poll_count.clone();

        std::thread::spawn(move || {
            for stream in listener.incoming() {
                let mut stream = match stream {
                    Ok(s) => s,
                    Err(_) => break,
                };
                let mut reader = BufReader::new(stream.try_clone().unwrap());
                let mut request_line = String::new();
                if reader.read_line(&mut request_line).is_err() || request_line.is_empty() {
                    continue;
                }

                let mut headers = Vec::new();
                let mut content_length = 0;
                let mut if_none_match = String::new();
                loop {
                    let mut line = String::new();
                    if reader.read_line(&mut line).is_err() || line.trim().is_empty() {
                        break;
                    }
                    if line.to_ascii_lowercase().starts_with("content-length:") {
                        if let Some(val) = line.split(':').nth(1) {
                            content_length = val.trim().parse::<usize>().unwrap_or(0);
                        }
                    }
                    if line.to_ascii_lowercase().starts_with("if-none-match:") {
                        if let Some(val) = line.split(':').nth(1) {
                            if_none_match = val.trim().to_string();
                        }
                    }
                    headers.push(line);
                }

                let mut body = vec![0u8; content_length];
                if content_length > 0 {
                    let _ = std::io::Read::read_exact(&mut reader, &mut body);
                }

                if request_line.starts_with("GET /api/v1/policy") {
                    if if_none_match == "\"v1-hash1\"" {
                        let resp = "HTTP/1.1 304 Not Modified\r\nContent-Length: 0\r\n\r\n";
                        let _ = stream.write_all(resp.as_bytes());
                    } else {
                        let json_resp = r#"{"version":"1","hash":"hash1","rules":[]}"#;
                        let resp = format!(
                            "HTTP/1.1 200 OK\r\nETag: \"v1-hash1\"\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n{}",
                            json_resp.len(),
                            json_resp
                        );
                        let _ = stream.write_all(resp.as_bytes());
                    }
                } else if request_line.starts_with("POST /api/v1/evidence/batch") {
                    let json_resp = r#"{"status":"ok"}"#;
                    let resp = format!(
                        "HTTP/1.1 201 Created\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n{}",
                        json_resp.len(),
                        json_resp
                    );
                    let _ = stream.write_all(resp.as_bytes());
                } else if request_line.starts_with("POST /api/v1/approval/request") {
                    let json_resp = r#"{"status":"pending"}"#;
                    let resp = format!(
                        "HTTP/1.1 201 Created\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n{}",
                        json_resp.len(),
                        json_resp
                    );
                    let _ = stream.write_all(resp.as_bytes());
                } else if request_line.starts_with("GET /api/v1/approval/req-mock-1") {
                    let count = poll_count_server.fetch_add(1, Ordering::SeqCst);
                    if count == 0 {
                        let json_resp = r#"{"state":"pending","signature":null}"#;
                        let resp = format!(
                            "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n{}",
                            json_resp.len(),
                            json_resp
                        );
                        let _ = stream.write_all(resp.as_bytes());
                    } else {
                        let payload = "req-mock-1apply_patchsrc/main.rssess-mock-11999999999";
                        let sig = signing_key.sign(payload.as_bytes());
                        let sig_hex = hex::encode(sig.to_bytes());
                        let pkey_hex = hex::encode(signing_key.verifying_key().as_bytes());
                        let json_resp = format!(
                            r#"{{"state":"approved","signature":{{"request_id":"req-mock-1","signature":"{}","public_key":"{}","signed_at":"1720000000","expires_at":"1999999999","scope":{{"action":"apply_patch","resource":"src/main.rs","session_id":"sess-mock-1"}}}}}}"#,
                            sig_hex, pkey_hex
                        );
                        let resp = format!(
                            "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n{}",
                            json_resp.len(),
                            json_resp
                        );
                        let _ = stream.write_all(resp.as_bytes());
                    }
                }
            }
        });

        let cfg = ThemisConfig {
            enabled: true,
            endpoint: format!("http://127.0.0.1:{port}"),
            tenant_id: "t1".to_string(),
            agent_id: "a1".to_string(),
            instance_id: "i1".to_string(),
            machine_id: "m1".to_string(),
            server_public_key: pubkey_hex,
            poll_interval_millis: 50,
        };

        let client = ThemisClient::new(cfg, "sess-mock-1".to_string());

        // 1. Initial Policy Fetch
        let pol = client.fetch_policy().await.unwrap().unwrap();
        assert_eq!(pol.version, "1");
        assert_eq!(pol.hash, "hash1");

        // 2. Cached Fetch (304)
        let pol_cached = client.fetch_policy().await.unwrap().unwrap();
        assert_eq!(pol_cached.version, "1");

        // 3. Evidence Batch / Session Close
        assert!(client.close_session(None).await.is_ok());

        // 4. Approval Request + Poll + Ed25519 signature verification
        let sig = client
            .request_and_await_approval(
                "req-mock-1",
                "apply_patch",
                "src/main.rs",
                "sensitive file",
                "1999999999",
                Duration::from_secs(3),
            )
            .await
            .unwrap();

        assert_eq!(sig.request_id, "req-mock-1");
        assert_eq!(sig.scope.action, "apply_patch");
        assert_eq!(sig.scope.resource, "src/main.rs");
    }
}
